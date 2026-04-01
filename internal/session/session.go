package session

import (
	"database/sql"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
	_ "modernc.org/sqlite"
)

type SessionItem struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

type sessionRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Directory string `json:"directory"`
}

func List(dir string) ([]SessionItem, error) {
	items, err := listSessionsDB(dir)
	if err == nil {
		return items, nil
	}

	return listSessionsCLI(dir)
}

func listSessionsDB(dir string) ([]SessionItem, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
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

	var result []SessionItem
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(&row.ID, &row.Title, &row.Updated, &row.Directory); err != nil {
			return nil, err
		}
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: opencodedb.UnixTimestampToTime(row.Updated)})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func listSessionsCLI(dir string) ([]SessionItem, error) {
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

	var result []SessionItem
	for _, row := range rows {
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: opencodedb.UnixTimestampToTime(row.Updated)})
	}

	return result, nil
}

func Latest(items []SessionItem) SessionItem {
	if len(items) == 0 {
		return SessionItem{}
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
