package stats

import (
	"database/sql"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

func mergeMessageStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	query := `
		SELECT
			m.time_created,
			CAST(COALESCE(json_extract(m.data, '$.role'), '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM message m
		%s
		WHERE m.time_created >= ?
	`
	args := []any{since}
	if dir != "" {
		query = strings.Replace(query, "%s", "JOIN session s ON s.id = m.session_id", 1)
		query = strings.Replace(query, "WHERE m.time_created >= ?", "WHERE "+scopedDirectoryClause()+" AND m.time_created >= ?", 1)
		args = []any{scopedDirectoryArg(dir), since}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var event messageEvent
		var summary int64
		if err := rows.Scan(&event.CreatedAt, &event.Role, &event.Cost, &summary, &event.Agent); err != nil {
			return err
		}
		event.Summary = summary != 0
		if event.Summary || strings.EqualFold(event.Agent, "compaction") {
			continue
		}
		if event.Role != "assistant" {
			continue
		}
		day := dayMap[dayKey(opencodedb.UnixTimestampToTime(event.CreatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.AssistantMessages++
		day.eventTimes = append(day.eventTimes, event.CreatedAt)
		if event.Agent != "" {
			day.Subtasks++
			day.AgentCounts[event.Agent]++
			day.UniqueAgents[event.Agent] = struct{}{}
		}
		day.Cost += event.Cost
	}

	return rows.Err()
}
