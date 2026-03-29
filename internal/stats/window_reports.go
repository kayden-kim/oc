package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func LoadWindowReport(dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	dbPath, err := opencodeDBPath()
	if err != nil {
		return WindowReport{}, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return WindowReport{}, err
	}
	defer db.Close()
	return buildWindowReport(db, dir, label, start, end)
}

func buildWindowReport(db *sql.DB, dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	report := WindowReport{Label: label, Start: start, End: end}
	sessionAgg := map[string]*SessionUsage{}
	modelAgg := map[string]*ModelUsage{}
	seenMessageCost := map[string]struct{}{}
	seenSessions := map[string]struct{}{}

	messageRows, err := loadWindowMessages(db, dir, start, end)
	if err != nil {
		return WindowReport{}, err
	}
	for _, row := range messageRows {
		if row.Summary || strings.EqualFold(row.Agent, "compaction") || row.Role != "assistant" {
			continue
		}
		report.Messages++
		report.Cost += row.Cost
		seenSessions[row.SessionID] = struct{}{}
		session := ensureSessionUsage(sessionAgg, row.SessionID, row.Title)
		session.Messages++
		session.Cost += row.Cost
	}

	partRows, err := loadWindowParts(db, dir, start, end)
	if err != nil {
		return WindowReport{}, err
	}
	for _, row := range partRows {
		if row.Summary || strings.EqualFold(row.MessageAgent, "compaction") || row.Type == "compaction" {
			continue
		}
		seenSessions[row.SessionID] = struct{}{}
		session := ensureSessionUsage(sessionAgg, row.SessionID, row.Title)
		if row.Type != "step-finish" {
			continue
		}

		model := ensureModelUsage(modelAgg, providerLabel(row.ProviderID), row.ModelID)
		model.InputTokens += row.InputTokens
		model.OutputTokens += row.OutputTokens
		model.CacheReadTokens += row.CacheReadTokens
		model.CacheWriteTokens += row.CacheWriteTokens
		model.ReasoningTokens += row.ReasoningTokens
		totalTokens := row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheWriteTokens + row.ReasoningTokens
		model.TotalTokens += totalTokens
		report.Tokens += row.InputTokens + row.OutputTokens + row.ReasoningTokens
		session.Tokens += totalTokens

		if row.MessageCost > 0 {
			if _, ok := seenMessageCost[row.MessageID]; !ok {
				seenMessageCost[row.MessageID] = struct{}{}
				model.Cost += row.MessageCost
			}
			continue
		}

		cost := row.Cost
		if cost <= 0 {
			event := partEvent{
				ProviderID:       row.ProviderID,
				ModelID:          row.ModelID,
				InputTokens:      row.InputTokens,
				OutputTokens:     row.OutputTokens,
				ReasoningTokens:  row.ReasoningTokens,
				CacheReadTokens:  row.CacheReadTokens,
				CacheWriteTokens: row.CacheWriteTokens,
			}
			estimatedCost, err := estimatePartCost(event)
			if err != nil {
				return WindowReport{}, fmt.Errorf("estimate window cost: %w", err)
			}
			cost = estimatedCost
		}
		report.Cost += cost
		model.Cost += cost
		session.Cost += cost
	}

	report.Sessions = len(seenSessions)
	report.Models = collectSortedModels(modelAgg)
	report.TopSessions = collectSortedSessions(sessionAgg)
	if len(report.TopSessions) > 8 {
		report.TopSessions = report.TopSessions[:8]
	}
	return report, nil
}

func loadWindowMessages(db *sql.DB, dir string, start, end time.Time) ([]windowMessageRow, error) {
	query := `
		SELECT m.id, m.session_id, CAST(COALESCE(s.title, '') AS TEXT), m.time_created,
		       CAST(COALESCE(json_extract(m.data, '$.role'), '') AS TEXT),
		       CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
		       CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM message m
		LEFT JOIN session s ON s.id = m.session_id
		WHERE m.time_created >= ? AND m.time_created < ?
	`
	args := []any{start.UnixMilli(), end.UnixMilli()}
	if dir != "" {
		query += " AND " + scopedDirectoryClause()
		args = append(args, scopedDirectoryArg(dir))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []windowMessageRow{}
	for rows.Next() {
		var row windowMessageRow
		var summary int64
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.CreatedAt, &row.Role, &row.Cost, &summary, &row.Agent); err != nil {
			return nil, err
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadWindowParts(db *sql.DB, dir string, start, end time.Time) ([]windowPartRow, error) {
	query := `
		SELECT p.message_id, p.session_id, CAST(COALESCE(s.title, '') AS TEXT), p.time_created,
		       CAST(COALESCE(json_extract(p.data, '$.type'), '') AS TEXT),
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
		LEFT JOIN message m ON m.id = p.message_id
		LEFT JOIN session s ON s.id = p.session_id
		WHERE p.time_created >= ? AND p.time_created < ?
	`
	args := []any{start.UnixMilli(), end.UnixMilli()}
	if dir != "" {
		query += " AND " + scopedDirectoryClause()
		args = append(args, scopedDirectoryArg(dir))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []windowPartRow{}
	for rows.Next() {
		var row windowPartRow
		var summary int64
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.CreatedAt, &row.Type, &row.ProviderID, &row.ModelID, &row.Cost, &row.MessageCost, &row.InputTokens, &row.OutputTokens, &row.ReasoningTokens, &row.CacheReadTokens, &row.CacheWriteTokens, &summary, &row.MessageAgent); err != nil {
			return nil, err
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func ensureSessionUsage(m map[string]*SessionUsage, id, title string) *SessionUsage {
	if usage, ok := m[id]; ok {
		if usage.Title == "" && title != "" {
			usage.Title = title
		}
		return usage
	}
	usage := &SessionUsage{ID: id, Title: title}
	m[id] = usage
	return usage
}

func ensureModelUsage(m map[string]*ModelUsage, source, model string) *ModelUsage {
	key := source + "\x00" + model
	if usage, ok := m[key]; ok {
		return usage
	}
	usage := &ModelUsage{Source: source, Model: model}
	m[key] = usage
	return usage
}

func collectSortedModels(m map[string]*ModelUsage) []ModelUsage {
	result := make([]ModelUsage, 0, len(m))
	for _, item := range m {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Cost == result[j].Cost {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}

func collectSortedSessions(m map[string]*SessionUsage) []SessionUsage {
	result := make([]SessionUsage, 0, len(m))
	for _, item := range m {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Cost == result[j].Cost {
			if result[i].Tokens == result[j].Tokens {
				return result[i].Messages > result[j].Messages
			}
			return result[i].Tokens > result[j].Tokens
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}
