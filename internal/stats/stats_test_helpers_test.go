package stats

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const defaultStatsTestSchema = `
	CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
	CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
`

var statsTestAnchor = time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)

type statsTestWorkSession struct {
	db      *sql.DB
	tempDir string
	dir     string
	now     time.Time
}

type usageSnapshot struct {
	Name  string
	Count int
}

type usageMetric struct {
	Name   string
	Count  int
	Amount int64
}

func openStatsTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	return openStatsTestDBWithSchema(t, defaultStatsTestSchema)
}

func openStatsTestDBWithSchema(t *testing.T, schema string) (*sql.DB, string) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", dbPath)
	return db, tmp
}

func setupStatsTestWorkSession(t *testing.T) statsTestWorkSession {
	t.Helper()
	db, tempDir := openStatsTestDB(t)
	dir := filepath.Join(tempDir, "work")
	insertSession(t, db, "ses_work", dir)
	return statsTestWorkSession{db: db, tempDir: tempDir, dir: dir, now: statsTestAnchor}
}

func rankedUsageFromReportField(t *testing.T, report Report, fieldName string) []usageSnapshot {
	t.Helper()
	value := reflect.ValueOf(report)
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	items := make([]usageSnapshot, 0, field.Len())
	for i := range field.Len() {
		entry := field.Index(i)
		items = append(items, usageSnapshot{
			Name:  entry.FieldByName("Name").String(),
			Count: int(entry.FieldByName("Count").Int()),
		})
	}
	return items
}

func rankedUsageMetricsFromReportField(t *testing.T, report Report, fieldName string) []usageMetric {
	t.Helper()
	value := reflect.ValueOf(report)
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	items := make([]usageMetric, 0, field.Len())
	for i := range field.Len() {
		entry := field.Index(i)
		item := usageMetric{Name: entry.FieldByName("Name").String()}
		if v := entry.FieldByName("Count"); v.IsValid() {
			item.Count = int(v.Int())
		}
		if v := entry.FieldByName("Amount"); v.IsValid() {
			item.Amount = v.Int()
		}
		items = append(items, item)
	}
	return items
}

func intFieldFromReport(t *testing.T, report Report, fieldName string) int64 {
	t.Helper()
	field := reflect.ValueOf(report).FieldByName(fieldName)
	if !field.IsValid() {
		return 0
	}
	return field.Int()
}

func insertSession(t *testing.T, db *sql.DB, id string, dir string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated) VALUES (?, ?, ?, NULL, ?)`, id, id, dir, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
}

func insertMessage(t *testing.T, db *sql.DB, id string, sessionID string, created time.Time, data string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`, id, sessionID, created.UnixMilli(), data); err != nil {
		t.Fatal(err)
	}
}

func insertPart(t *testing.T, db *sql.DB, id string, messageID string, sessionID string, created time.Time, data string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, ?, ?, ?)`, id, messageID, sessionID, created.UnixMilli(), data); err != nil {
		t.Fatal(err)
	}
}
