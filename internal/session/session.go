package session

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/tui"
	_ "modernc.org/sqlite"
)

type sessionRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Directory string `json:"directory"`
}

func List(dir string) ([]tui.SessionItem, error) {
	items, err := listSessionsDB(dir)
	if err == nil {
		return items, nil
	}

	return listSessionsCLI(dir)
}

func listSessionsDB(dir string) ([]tui.SessionItem, error) {
	dbPath, err := opencodeDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := "SELECT id, title, time_updated, directory FROM session WHERE parent_id IS NULL AND directory = ? ORDER BY time_updated DESC LIMIT 100"
	if runtime.GOOS == "windows" {
		query = "SELECT id, title, time_updated, directory FROM session WHERE parent_id IS NULL AND replace(lower(directory), '\\', '/') = replace(lower(?), '\\', '/') ORDER BY time_updated DESC LIMIT 100"
	}

	rows, err := db.Query(query, filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []tui.SessionItem
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(&row.ID, &row.Title, &row.Updated, &row.Directory); err != nil {
			return nil, err
		}
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, tui.SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: unixTimestampToTime(row.Updated)})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func listSessionsCLI(dir string) ([]tui.SessionItem, error) {
	cmd := exec.Command("opencode", "session", "list", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rows []sessionRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Updated > rows[j].Updated
	})

	var result []tui.SessionItem
	for _, row := range rows {
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, tui.SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: unixTimestampToTime(row.Updated)})
	}

	return result, nil
}

func sqliteDSN(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
}

func opencodeDBPath() (string, error) {
	if name := os.Getenv("OPENCODE_DB"); name != "" {
		if filepath.IsAbs(name) {
			if _, err := os.Stat(name); err == nil {
				return name, nil
			} else {
				return "", err
			}
		}

		root, err := opencodeDataDir()
		if err != nil {
			return "", err
		}
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else {
			return "", err
		}
	}

	root, err := opencodeDataDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, "opencode.db")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	paths, err := filepath.Glob(filepath.Join(root, "opencode-*.db"))
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", os.ErrNotExist
	}

	sort.SliceStable(paths, func(i, j int) bool {
		left, leftErr := os.Stat(paths[i])
		right, rightErr := os.Stat(paths[j])
		if leftErr != nil || rightErr != nil {
			return paths[i] < paths[j]
		}
		return left.ModTime().After(right.ModTime())
	})

	return paths[0], nil
}

func opencodeDataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "opencode"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "opencode"), nil
	}

	return filepath.Join(home, ".local", "share", "opencode"), nil
}

func unixTimestampToTime(value int64) time.Time {
	switch {
	case value >= 1_000_000_000_000_000_000 || value <= -1_000_000_000_000_000_000:
		return time.Unix(0, value).Local()
	case value >= 1_000_000_000_000_000 || value <= -1_000_000_000_000_000:
		return time.UnixMicro(value).Local()
	case value >= 1_000_000_000_000 || value <= -1_000_000_000_000:
		return time.UnixMilli(value).Local()
	default:
		return time.Unix(value, 0).Local()
	}
}

func Latest(items []tui.SessionItem) tui.SessionItem {
	if len(items) == 0 {
		return tui.SessionItem{}
	}
	return items[0]
}

func sameDir(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
