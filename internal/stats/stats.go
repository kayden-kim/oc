package stats

import (
	"database/sql"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

const dayWindow = 30

const defaultSessionGapMinutes = 15

const unlimitedUsageItems = 0

func LoadGlobal() (Report, error) {
	return loadAtWithOptions(time.Now(), "", Options{})
}

func LoadForDir(dir string) (Report, error) {
	return loadAtWithOptions(time.Now(), dir, Options{})
}

func LoadGlobalWithOptions(options Options) (Report, error) {
	return loadAtWithOptions(time.Now(), "", options)
}

func LoadForDirWithOptions(dir string, options Options) (Report, error) {
	return loadAtWithOptions(time.Now(), dir, options)
}

func loadGlobalAt(now time.Time) (Report, error) {
	return loadAtWithOptions(now, "", Options{})
}

func loadForDirAt(dir string, now time.Time) (Report, error) {
	return loadAtWithOptions(now, dir, Options{})
}

func loadForDirAtWithOptions(dir string, now time.Time, options Options) (Report, error) {
	return loadAtWithOptions(now, dir, options)
}

func mergePartFileStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	query := `
		SELECT
			p.time_created,
			CAST(COALESCE(p.data, '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM part p
		%s
		LEFT JOIN message m ON m.id = p.message_id
		WHERE p.time_created >= ?
	`
	joinClause := ""
	args := []any{since}
	if dir != "" {
		joinClause = "JOIN session s ON s.id = p.session_id"
		query = strings.Replace(query, "%s", joinClause, 1)
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

	seenByDay := make(map[string]map[string]struct{}, len(dayMap))
	for rows.Next() {
		var createdAt int64
		var raw string
		var summary int64
		var messageAgent string
		if err := rows.Scan(&createdAt, &raw, &summary, &messageAgent); err != nil {
			return err
		}
		if summary != 0 || strings.EqualFold(messageAgent, "compaction") {
			continue
		}
		bucketKey := dayKey(opencodedb.UnixTimestampToTime(createdAt).In(loc))
		day := dayMap[bucketKey]
		if day == nil {
			continue
		}
		files := extractChangedFilesFromPart(raw)
		if len(files) == 0 {
			continue
		}
		seen := seenByDay[bucketKey]
		if seen == nil {
			seen = map[string]struct{}{}
			seenByDay[bucketKey] = seen
		}
		for _, file := range files {
			normalized := normalizeChangedFilePath(file)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			day.ChangedFiles++
		}
	}

	return rows.Err()
}

func normalizeOptions(options Options) Options {
	if options.SessionGapMinutes <= 0 {
		options.SessionGapMinutes = defaultSessionGapMinutes
	}
	return options
}

func agentModelUsageKey(agent string, model string) string {
	return agent + "\x00" + model
}

func providerModelUsageKey(provider string, model string) string {
	return provider + "\x00" + model
}
