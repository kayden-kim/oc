package stats

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func startOfDay(t time.Time) time.Time {
	local := t.Local()
	year, month, day := local.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, local.Location())
}

func startOfMonth(t time.Time) time.Time {
	local := t.Local()
	year, month, _ := local.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, local.Location())
}

func dayKey(t time.Time) string {
	return startOfDay(t).Format("2006-01-02")
}

func scopedDirectoryClause() string {
	if runtime.GOOS == "windows" {
		return "replace(lower(s.directory), '\\', '/') = replace(lower(?), '\\', '/')"
	}
	return "s.directory = ?"
}

func scopedDirectoryArg(dir string) string {
	return filepath.Clean(dir)
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
