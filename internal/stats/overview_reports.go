package stats

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
	_ "modernc.org/sqlite"
)

func loadAtWithOptions(now time.Time, dir string, options Options) (Report, error) {
	options = normalizeOptions(options)
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return buildReport(nil, now, options), nil
		}
		return Report{}, err
	}

	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return Report{}, err
	}
	defer db.Close()

	since := startOfDay(now).AddDate(0, 0, -(dayWindow - 1)).UnixMilli()
	dayMap := make(map[string]*Day, dayWindow)
	for _, day := range buildEmptyDays(now) {
		copyDay := day
		dayMap[dayKey(day.Date)] = &copyDay
	}

	if err := mergeMessageStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}
	if err := mergePartStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}
	if err := mergeSessionCodeStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}
	if err := mergePartFileStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}

	days := make([]Day, 0, len(dayMap))
	for _, day := range dayMap {
		days = append(days, *day)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.Before(days[j].Date)
	})

	report := buildReport(days, now, options)
	if dir == "" {
		projectTotals, err := loadProjectUsage(db, since)
		if err != nil {
			return Report{}, err
		}
		report.TopProjects = topUsageAmountsWithCosts(projectTotals, unlimitedUsageItems)
		for _, usage := range projectTotals {
			report.TotalProjectCost += usage.Cost
		}
		report.UniqueProjectCount = len(projectTotals)
	}

	return report, nil
}

func loadProjectUsage(db *sql.DB, since int64) (map[string]projectUsage, error) {
	tokenTotals, err := loadProjectTokenTotals(db, since)
	if err != nil {
		return nil, err
	}
	costTotals, err := loadProjectCostTotals(db, since)
	if err != nil {
		return nil, err
	}

	projectTotals := make(map[string]projectUsage, len(tokenTotals)+len(costTotals))
	for key, tokens := range tokenTotals {
		usage := projectTotals[key]
		usage.Tokens = tokens
		projectTotals[key] = usage
	}
	for key, cost := range costTotals {
		usage := projectTotals[key]
		usage.Cost = cost
		projectTotals[key] = usage
	}

	return projectTotals, nil
}

func loadProjectTokenTotals(db *sql.DB, since int64) (map[string]int64, error) {
	rows, err := db.Query(`
		SELECT
			CAST(COALESCE(s.directory, '') AS TEXT),
			SUM(
				CAST(COALESCE(json_extract(p.data, '$.tokens.input'), 0) AS INTEGER) +
				CAST(COALESCE(json_extract(p.data, '$.tokens.output'), 0) AS INTEGER) +
				CAST(COALESCE(json_extract(p.data, '$.tokens.reasoning'), 0) AS INTEGER) +
				CAST(COALESCE(json_extract(p.data, '$.tokens.cache.read'), 0) AS INTEGER) +
				CAST(COALESCE(json_extract(p.data, '$.tokens.cache.write'), 0) AS INTEGER)
			) AS total_tokens
		FROM part p
		LEFT JOIN message m ON m.id = p.message_id
		LEFT JOIN session s ON s.id = p.session_id
		WHERE p.time_created >= ?
		  AND CAST(COALESCE(json_extract(p.data, '$.type'), '') AS TEXT) = 'step-finish'
		  AND CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER) = 0
		  AND lower(CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)) != 'compaction'
		GROUP BY CAST(COALESCE(s.directory, '') AS TEXT)
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make(map[string]int64)
	for rows.Next() {
		var directory string
		var totalTokens int64
		if err := rows.Scan(&directory, &totalTokens); err != nil {
			return nil, err
		}
		if totalTokens <= 0 {
			continue
		}
		totals[normalizeProjectUsageKey(directory)] = totalTokens
	}

	return totals, rows.Err()
}

func loadProjectCostTotals(db *sql.DB, since int64) (map[string]float64, error) {
	rows, err := db.Query(`
		SELECT
			CAST(COALESCE(s.directory, '') AS TEXT),
			p.message_id,
			CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
			CAST(COALESCE(json_extract(p.data, '$.cost'), 0) AS REAL),
			CAST(COALESCE(json_extract(p.data, '$.tokens.input'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.output'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.reasoning'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.cache.read'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(p.data, '$.tokens.cache.write'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.providerID'), '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.modelID'), '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT),
			CAST(COALESCE(json_extract(p.data, '$.type'), '') AS TEXT)
		FROM part p
		LEFT JOIN message m ON m.id = p.message_id
		LEFT JOIN session s ON s.id = p.session_id
		WHERE p.time_created >= ?
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make(map[string]float64)
	seenMessageCost := make(map[string]struct{})
	for rows.Next() {
		var directory string
		var event partEvent
		var summary int64
		if err := rows.Scan(
			&directory,
			&event.MessageID,
			&event.MessageCost,
			&event.Cost,
			&event.InputTokens,
			&event.OutputTokens,
			&event.ReasoningTokens,
			&event.CacheReadTokens,
			&event.CacheWriteTokens,
			&event.ProviderID,
			&event.ModelID,
			&summary,
			&event.MessageAgent,
			&event.Type,
		); err != nil {
			return nil, err
		}
		if event.Type != "step-finish" || summary != 0 || strings.EqualFold(event.MessageAgent, "compaction") || event.Type == "compaction" {
			continue
		}

		stepCost := 0.0
		if event.MessageCost > 0 {
			if _, ok := seenMessageCost[event.MessageID]; !ok {
				seenMessageCost[event.MessageID] = struct{}{}
				stepCost = event.MessageCost
			}
		} else if event.Cost > 0 {
			stepCost = event.Cost
		} else {
			estimatedCost, err := estimatePartCost(event)
			if err != nil {
				return nil, fmt.Errorf("estimate project step-finish cost: %w", err)
			}
			stepCost = estimatedCost
		}
		if stepCost <= 0 {
			continue
		}
		totals[normalizeProjectUsageKey(directory)] += stepCost
	}

	return totals, rows.Err()
}
