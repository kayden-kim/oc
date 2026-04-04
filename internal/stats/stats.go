package stats

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
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

func normalizeProjectUsageKey(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "(unknown project)"
	}
	cleaned := filepath.Clean(directory)
	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.ToSlash(cleaned))
	}
	return cleaned
}

func mergeSessionCodeStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	hasColumns, err := hasSessionSummaryColumns(db)
	if err != nil {
		return err
	}
	if !hasColumns {
		return nil
	}

	query := `
		SELECT
			s.time_updated,
			CAST(COALESCE(s.summary_additions, 0) AS INTEGER),
			CAST(COALESCE(s.summary_deletions, 0) AS INTEGER)
		FROM session s
		WHERE %s s.time_updated >= ?
	`
	args := []any{since}
	wherePrefix := ""
	if dir != "" {
		wherePrefix = scopedDirectoryClause() + " AND "
		args = []any{scopedDirectoryArg(dir), since}
	}
	rows, err := db.Query(fmt.Sprintf(query, wherePrefix), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var updatedAt int64
		var additions int
		var deletions int
		if err := rows.Scan(&updatedAt, &additions, &deletions); err != nil {
			return err
		}
		day := dayMap[dayKey(opencodedb.UnixTimestampToTime(updatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.CodeLines += additions + deletions
	}

	return rows.Err()
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

func hasSessionSummaryColumns(db *sql.DB) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(session)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	hasAdditions := false
	hasDeletions := false
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		switch name {
		case "summary_additions":
			hasAdditions = true
		case "summary_deletions":
			hasDeletions = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return hasAdditions && hasDeletions, nil
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
