package stats

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func loadWindowCodeLines(db *sql.DB, dir string, start, end time.Time) (int, error) {
	hasColumns, err := hasSessionSummaryColumns(db)
	if err != nil {
		return 0, err
	}
	if !hasColumns {
		return 0, nil
	}

	query := `
		SELECT
			CAST(COALESCE(s.summary_additions, 0) AS INTEGER),
			CAST(COALESCE(s.summary_deletions, 0) AS INTEGER)
		FROM session s
		WHERE %s s.time_updated >= ? AND s.time_updated < ?
	`
	args := []any{start.UnixMilli(), end.UnixMilli()}
	wherePrefix := ""
	if dir != "" {
		wherePrefix = scopedDirectoryClause() + " AND "
		args = []any{scopedDirectoryArg(dir), start.UnixMilli(), end.UnixMilli()}
	}
	rows, err := db.Query(fmt.Sprintf(query, wherePrefix), args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var additions int
		var deletions int
		if err := rows.Scan(&additions, &deletions); err != nil {
			return 0, err
		}
		total += additions + deletions
	}
	return total, rows.Err()
}

func loadWindowChangedFiles(db *sql.DB, dir string, start, end time.Time) (int, error) {
	query := `
		SELECT
			CAST(COALESCE(p.data, '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM part p
		%s
		LEFT JOIN message m ON m.id = p.message_id
		WHERE p.time_created >= ? AND p.time_created < ?
	`
	joinClause := ""
	args := []any{start.UnixMilli(), end.UnixMilli()}
	if dir != "" {
		joinClause = "JOIN session s ON s.id = p.session_id"
		query = strings.Replace(query, "%s", joinClause, 1)
		query = strings.Replace(query, "WHERE p.time_created >= ? AND p.time_created < ?", "WHERE "+scopedDirectoryClause()+" AND p.time_created >= ? AND p.time_created < ?", 1)
		args = []any{scopedDirectoryArg(dir), start.UnixMilli(), end.UnixMilli()}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var raw string
		var summary int64
		var messageAgent string
		if err := rows.Scan(&raw, &summary, &messageAgent); err != nil {
			return 0, err
		}
		if summary != 0 || strings.EqualFold(messageAgent, "compaction") {
			continue
		}
		for _, file := range extractChangedFilesFromPart(raw) {
			normalized := normalizeChangedFilePath(file)
			if normalized == "" {
				continue
			}
			seen[normalized] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return len(seen), nil
}

func loadWindowMessages(db *sql.DB, dir string, start, end time.Time) ([]windowMessageRow, error) {
	query := `
		SELECT m.id, m.session_id, CAST(COALESCE(s.title, '') AS TEXT), CAST(COALESCE(s.directory, '') AS TEXT), m.time_created,
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
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.Directory, &row.CreatedAt, &row.Role, &row.Cost, &summary, &row.Agent); err != nil {
			return nil, err
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadWindowParts(db *sql.DB, dir string, start, end time.Time) ([]windowPartRow, error) {
	query := `
		SELECT p.message_id, p.session_id, CAST(COALESCE(s.title, '') AS TEXT), CAST(COALESCE(s.directory, '') AS TEXT), p.time_created,
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
		var skillNameType string
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.Directory, &row.CreatedAt, &row.Type, &row.Tool, &skillNameType, &row.SkillName, &row.Agent, &row.ProviderID, &row.ModelID, &row.Cost, &row.MessageCost, &row.InputTokens, &row.OutputTokens, &row.ReasoningTokens, &row.CacheReadTokens, &row.CacheWriteTokens, &summary, &row.MessageAgent); err != nil {
			return nil, err
		}
		if skillNameType != "text" {
			row.SkillName = ""
		} else {
			row.SkillName = strings.TrimSpace(row.SkillName)
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}
