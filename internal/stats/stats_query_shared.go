package stats

import (
	"path/filepath"
	"runtime"
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
