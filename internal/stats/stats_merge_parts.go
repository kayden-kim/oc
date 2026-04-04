package stats

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

func mergePartStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	query := `
		SELECT
			p.message_id,
			p.time_created,
			CAST(COALESCE(json_extract(p.data, '$.type'), '') AS TEXT),
			CAST(COALESCE(json_extract(p.data, '$.tool'), '') AS TEXT),
			CAST(COALESCE(json_type(p.data, '$.state.input.name'), '') AS TEXT),
			CAST(COALESCE(json_extract(p.data, '$.state.input.name'), '') AS TEXT),
			CAST(COALESCE(json_extract(p.data, '$.agent'), '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.providerID'), '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.modelID'), '') AS TEXT),
			CAST(COALESCE(json_extract(p.data, '$.cost'), 0) AS REAL),
			CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
			CAST(COALESCE(json_extract(p.data, '$.tokens.input'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.output'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.reasoning'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.cache.read'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.cache.write'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM part p
		%s
		LEFT JOIN message m ON m.id = p.message_id
		WHERE p.time_created >= ?
	`
	args := []any{since}
	if dir != "" {
		query = strings.Replace(query, "%s", "JOIN session s ON s.id = p.session_id", 1)
		query = strings.Replace(query, "WHERE p.time_created >= ?", "WHERE "+scopedDirectoryClause()+" AND p.time_created >= ?", 1)
		args = []any{scopedDirectoryArg(dir), since}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	seenModelMessageCost := map[string]struct{}{}
	for rows.Next() {
		var event partEvent
		var summary int64
		var skillNameType string
		if err := rows.Scan(
			&event.MessageID,
			&event.CreatedAt,
			&event.Type,
			&event.Tool,
			&skillNameType,
			&event.SkillName,
			&event.Agent,
			&event.ProviderID,
			&event.ModelID,
			&event.Cost,
			&event.MessageCost,
			&event.InputTokens,
			&event.OutputTokens,
			&event.ReasoningTokens,
			&event.CacheReadTokens,
			&event.CacheWriteTokens,
			&summary,
			&event.MessageAgent,
		); err != nil {
			return err
		}
		if skillNameType != "text" {
			event.SkillName = ""
		} else {
			event.SkillName = strings.TrimSpace(event.SkillName)
		}
		event.Summary = summary != 0
		if event.Summary || strings.EqualFold(event.MessageAgent, "compaction") || event.Type == "compaction" {
			continue
		}
		day := dayMap[dayKey(opencodedb.UnixTimestampToTime(event.CreatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.eventTimes = append(day.eventTimes, event.CreatedAt)
		switch event.Type {
		case "tool":
			day.ToolCalls++
			if event.Tool != "" {
				day.ToolCounts[event.Tool]++
				day.UniqueTools[event.Tool] = struct{}{}
			}
			if event.Tool == "skill" {
				day.SkillCalls++
				if event.SkillName != "" {
					day.SkillCounts[event.SkillName]++
					day.UniqueSkills[event.SkillName] = struct{}{}
				}
			}
		case "step-finish":
			day.StepFinishes++
			stepCost := 0.0
			if event.MessageCost > 0 {
				if _, ok := seenModelMessageCost[event.MessageID]; !ok {
					seenModelMessageCost[event.MessageID] = struct{}{}
					stepCost = event.MessageCost
				}
			} else if event.Cost > 0 {
				day.Cost += event.Cost
				stepCost = event.Cost
			} else {
				estimatedCost, err := estimatePartCost(event)
				if err != nil {
					return fmt.Errorf("estimate step-finish cost: %w", err)
				}
				day.Cost += estimatedCost
				stepCost = estimatedCost
			}
			stepTokens := event.InputTokens + event.OutputTokens + event.ReasoningTokens + event.CacheReadTokens + event.CacheWriteTokens
			day.Tokens += stepTokens
			day.ReasoningTokens += event.ReasoningTokens
			st := opencodedb.UnixTimestampToTime(event.CreatedAt).In(loc)
			day.SlotTokens[st.Hour()*2+st.Minute()/30] += stepTokens
			name := modelLabel(event.ProviderID, event.ModelID)
			modelName := strings.TrimSpace(event.ModelID)
			if name != "" {
				day.ModelCounts[name] += stepTokens
				day.ModelCosts[name] += stepCost
			}
			agentName := strings.TrimSpace(event.Agent)
			if agentName == "" {
				agentName = strings.TrimSpace(event.MessageAgent)
			}
			if agentName != "" && !strings.EqualFold(agentName, "compaction") && modelName != "" {
				key := agentModelUsageKey(agentName, modelName)
				day.AgentModelCounts[key]++
				day.UniqueAgentModels[key] = struct{}{}
			}
		}
	}

	return rows.Err()
}

func extractChangedFilesFromPart(raw string) []string {
	type partPayload struct {
		Type  string   `json:"type"`
		Tool  string   `json:"tool"`
		Files []string `json:"files"`
		State struct {
			Status string `json:"status"`
			Input  struct {
				FilePath  string `json:"filePath"`
				PatchText string `json:"patchText"`
			} `json:"input"`
		} `json:"state"`
	}

	var payload partPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	fileSet := map[string]struct{}{}
	addFile := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		fileSet[path] = struct{}{}
	}

	switch payload.Type {
	case "patch":
		for _, file := range payload.Files {
			addFile(file)
		}
	case "tool":
		if payload.State.Status != "completed" {
			break
		}
		switch payload.Tool {
		case "write", "edit":
			addFile(payload.State.Input.FilePath)
		case "apply_patch":
			for _, file := range extractFilesFromPatchText(payload.State.Input.PatchText) {
				addFile(file)
			}
		}
	}

	if len(fileSet) == 0 {
		return nil
	}
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	return files
}

func extractFilesFromPatchText(patchText string) []string {
	if strings.TrimSpace(patchText) == "" {
		return nil
	}
	files := map[string]struct{}{}
	for _, line := range strings.Split(patchText, "\n") {
		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			files[strings.TrimSpace(strings.TrimPrefix(line, "*** Add File: "))] = struct{}{}
		case strings.HasPrefix(line, "*** Update File: "):
			files[strings.TrimSpace(strings.TrimPrefix(line, "*** Update File: "))] = struct{}{}
		case strings.HasPrefix(line, "*** Delete File: "):
			files[strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File: "))] = struct{}{}
		}
	}
	if len(files) == 0 {
		return nil
	}
	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	return result
}

func normalizeChangedFilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.ToSlash(path))
	}
	return path
}
