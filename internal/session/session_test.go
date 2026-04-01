package session

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
	_ "modernc.org/sqlite"
)

func TestListSessionsDB_ReadsMatchingRootSessions(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			directory TEXT NOT NULL,
			parent_id TEXT,
			time_updated INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	other := filepath.Join(tmp, "other")
	for _, path := range []string{dir, other} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec(`
		INSERT INTO session (id, title, directory, parent_id, time_updated) VALUES
			('ses_old', 'Old', ?, NULL, ?),
			('ses_new', 'New', ?, NULL, ?),
			('ses_child', 'Child', ?, 'ses_new', ?),
			('ses_other', 'Other', ?, NULL, ?)
	`, dir, time.Now().Add(-2*time.Hour).UnixMilli(), dir, time.Now().Add(-time.Hour).UnixMilli(), dir, time.Now().UnixMilli(), other, time.Now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENCODE_DB", dbPath)
	items, err := listSessionsDB(dir)
	if err != nil {
		t.Fatalf("listSessionsDB returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 root sessions, got %d", len(items))
	}
	if items[0].ID != "ses_new" || items[1].ID != "ses_old" {
		t.Fatalf("unexpected session order: %+v", items)
	}
	if items[0].Title != "New" || items[1].Title != "Old" {
		t.Fatalf("unexpected session titles: %+v", items)
	}
}

func TestOpencodeDBPath_ReturnsErrorForMissingOverride(t *testing.T) {
	t.Setenv("OPENCODE_DB", filepath.Join(t.TempDir(), "missing.db"))
	_, err := opencodedb.DBPath()
	if err == nil {
		t.Fatal("expected missing override DB to return error")
	}
}

func TestOpencodeDataDir_DefaultsByPlatform(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")

	dir, err := opencodedb.DataDir()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, ".local", "share", "opencode")
	if dir != want {
		t.Fatalf("expected %q, got %q", want, dir)
	}
}

func TestOpencodeDataDir_UsesXDGDataHomeOverride(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg-data")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", xdg)

	dir, err := opencodedb.DataDir()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(xdg, "opencode")
	if dir != want {
		t.Fatalf("expected %q, got %q", want, dir)
	}
}

func TestUnixTimestampToTime(t *testing.T) {
	now := time.Now().Local().Truncate(time.Second)
	tests := []struct {
		name  string
		input int64
		want  time.Time
	}{
		{name: "seconds", input: now.Unix(), want: time.Unix(now.Unix(), 0).Local()},
		{name: "milliseconds", input: now.UnixMilli(), want: time.UnixMilli(now.UnixMilli()).Local()},
		{name: "microseconds", input: now.UnixMicro(), want: time.UnixMicro(now.UnixMicro()).Local()},
		{name: "nanoseconds", input: now.UnixNano(), want: time.Unix(0, now.UnixNano()).Local()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := opencodedb.UnixTimestampToTime(tt.input)
			if !got.Equal(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestLatest(t *testing.T) {
	if got := Latest(nil); got != (SessionItem{}) {
		t.Fatalf("expected zero session for empty input, got %+v", got)
	}

	items := []SessionItem{{ID: "ses_latest", Title: "Latest"}, {ID: "ses_older", Title: "Older"}}
	if got := Latest(items); got != items[0] {
		t.Fatalf("expected %+v, got %+v", items[0], got)
	}
}

func TestSameDir(t *testing.T) {
	left := filepath.Join("root", "workspace")
	right := filepath.Join("root", "workspace")
	if !sameDir(left, right) {
		t.Fatal("expected identical cleaned paths to match")
	}

	if runtime.GOOS == "windows" {
		if !sameDir(`C:\Work\Repo`, `c:\work\repo`) {
			t.Fatal("expected sameDir to ignore case on windows")
		}
	}
}

func TestSQLiteDSN(t *testing.T) {
	path := filepath.Join("tmp", "opencode.db")
	got := opencodedb.SQLiteDSN(path)
	if !strings.HasPrefix(got, "file:") {
		t.Fatalf("expected file DSN, got %q", got)
	}
	if !strings.Contains(got, "?mode=ro&_pragma=busy_timeout(5000)") {
		t.Fatalf("expected read-only DSN suffix, got %q", got)
	}
	if runtime.GOOS == "windows" && !strings.HasPrefix(strings.TrimPrefix(got, "file:"), "/") {
		t.Fatalf("expected windows DSN path to start with slash, got %q", got)
	}
}
