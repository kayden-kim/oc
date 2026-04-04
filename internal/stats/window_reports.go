package stats

import (
	"database/sql"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

func LoadWindowReport(dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return WindowReport{}, err
	}
	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return WindowReport{}, err
	}
	defer db.Close()
	return buildWindowReport(db, dir, label, start, end)
}

func buildWindowReport(db *sql.DB, dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	accumulator := newWindowReportAccumulator(label, start, end)
	messageRows, err := loadWindowMessages(db, dir, start, end)
	if err != nil {
		return WindowReport{}, err
	}
	for _, row := range messageRows {
		accumulator.addMessage(row)
	}

	partRows, err := loadWindowParts(db, dir, start, end)
	if err != nil {
		return WindowReport{}, err
	}
	for _, row := range partRows {
		if err := accumulator.addPart(row); err != nil {
			return WindowReport{}, err
		}
	}
	report := accumulator.finalize()
	if report.CodeLines, err = loadWindowCodeLines(db, dir, start, end); err != nil {
		return WindowReport{}, err
	}
	if report.ChangedFiles, err = loadWindowChangedFiles(db, dir, start, end); err != nil {
		return WindowReport{}, err
	}
	return report, nil
}

func LoadMonthDailyReport(dir string, monthStart time.Time) (MonthDailyReport, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return MonthDailyReport{}, err
	}
	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return MonthDailyReport{}, err
	}
	defer db.Close()
	return buildMonthDailyReport(db, dir, monthStart)
}

func LoadYearMonthlyReport(dir string, endMonth time.Time) (YearMonthlyReport, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return YearMonthlyReport{}, err
	}
	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return YearMonthlyReport{}, err
	}
	defer db.Close()
	return buildYearMonthlyReport(db, dir, endMonth)
}
