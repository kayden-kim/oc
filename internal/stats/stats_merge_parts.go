package stats

import (
	"database/sql"
	"fmt"
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
