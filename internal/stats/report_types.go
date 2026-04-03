package stats

import "time"

type Day struct {
	Date              time.Time
	AssistantMessages int
	ToolCalls         int
	SkillCalls        int
	StepFinishes      int
	Subtasks          int
	Cost              float64
	Tokens            int64
	ReasoningTokens   int64
	SessionMinutes    int
	CodeLines         int
	ChangedFiles      int
	ToolCounts        map[string]int
	SkillCounts       map[string]int
	AgentCounts       map[string]int
	AgentModelCounts  map[string]int
	ModelCounts       map[string]int64
	ModelCosts        map[string]float64
	eventTimes        []int64
	UniqueTools       map[string]struct{}
	UniqueSkills      map[string]struct{}
	UniqueAgents      map[string]struct{}
	UniqueAgentModels map[string]struct{}
	SlotTokens        [48]int64
}

func newEmptyDay(date time.Time) Day {
	return Day{
		Date:              date,
		ToolCounts:        map[string]int{},
		SkillCounts:       map[string]int{},
		AgentCounts:       map[string]int{},
		AgentModelCounts:  map[string]int{},
		ModelCounts:       map[string]int64{},
		ModelCosts:        map[string]float64{},
		UniqueTools:       map[string]struct{}{},
		UniqueSkills:      map[string]struct{}{},
		UniqueAgents:      map[string]struct{}{},
		UniqueAgentModels: map[string]struct{}{},
	}
}

func buildEmptyDays(now time.Time) []Day {
	start := startOfDay(now).AddDate(0, 0, -(dayWindow - 1))
	days := make([]Day, 0, dayWindow)
	for i := 0; i < dayWindow; i++ {
		days = append(days, newEmptyDay(start.AddDate(0, 0, i)))
	}
	return days
}

func newEmptyReport(days []Day) Report {
	return Report{
		Days:                   days,
		HighestBurnDay:         newEmptyDay(time.Time{}),
		HighestCodeDay:         newEmptyDay(time.Time{}),
		HighestChangedFilesDay: newEmptyDay(time.Time{}),
		LongestSessionDay:      newEmptyDay(time.Time{}),
		MostEfficientDay:       newEmptyDay(time.Time{}),
	}
}

type UsageCount struct {
	Name   string
	Count  int
	Amount int64
	Cost   float64
}

type projectUsage struct {
	Tokens int64
	Cost   float64
}

type Options struct {
	SessionGapMinutes int
}

type Report struct {
	Days                     []Day
	ActiveDays               int
	AgentDays                int
	CurrentStreak            int
	BestStreak               int
	CurrentHourlyStreakSlots int
	BestHourlyStreakSlots    int
	WeeklyActiveDays         int
	WeeklyAgentDays          int
	ThirtyDayCost            float64
	ThirtyDayTokens          int64
	ThirtyDaySessionMinutes  int
	ThirtyDayCodeLines       int
	ThirtyDayChangedFiles    int
	TotalToolCalls           int
	TotalSkillCalls          int
	TotalSubtasks            int
	UniqueToolCount          int
	UniqueSkillCount         int
	UniqueAgentCount         int
	UniqueAgentModelCount    int
	UniqueModelCount         int
	TodayCost                float64
	YesterdayCost            float64
	TodayTokens              int64
	YesterdayTokens          int64
	TotalModelTokens         int64
	TotalModelCost           float64
	TotalProjectCost         float64
	TotalAgentModelCalls     int
	TodaySessionMinutes      int
	YesterdaySessionMinutes  int
	TodayCodeLines           int
	YesterdayCodeLines       int
	TodayChangedFiles        int
	YesterdayChangedFiles    int
	TodayReasoningShare      float64
	RecentReasoningShare     float64
	WeekdayActiveCounts      [7]int
	WeekdayAgentCounts       [7]int
	TopTools                 []UsageCount
	TopSkills                []UsageCount
	TopAgents                []UsageCount
	TopAgentModels           []UsageCount
	TopProjects              []UsageCount
	TopModels                []UsageCount
	UniqueProjectCount       int
	HighestBurnDay           Day
	HighestCodeDay           Day
	HighestChangedFilesDay   Day
	LongestSessionDay        Day
	MostEfficientDay         Day
	Rolling24hSlots          [48]int64
	Rolling24hSessionMinutes int
}

type WindowReport struct {
	Label                string
	Start                time.Time
	End                  time.Time
	Messages             int
	Sessions             int
	InputTokens          int64
	OutputTokens         int64
	CacheReadTokens      int64
	CacheWriteTokens     int64
	ReasoningTokens      int64
	Tokens               int64
	Cost                 float64
	TotalToolCalls       int
	TotalSkillCalls      int
	TotalSubtasks        int
	TotalAgentModelCalls int
	TotalProjectCost     float64
	UniqueProjectCount   int
	CodeLines            int
	ChangedFiles         int
	Models               []ModelUsage
	AllSessions          []SessionUsage
	TopSessions          []SessionUsage
	TopProjects          []UsageCount
	TopAgents            []UsageCount
	TopAgentModels       []UsageCount
	TopSkills            []UsageCount
	TopTools             []UsageCount
	HalfHourSlots        [48]int64
	ActiveMinutes        int
}

type ModelUsage struct {
	Model            string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
	TotalTokens      int64
	Cost             float64
}

type SessionUsage struct {
	ID       string
	Title    string
	Cost     float64
	Tokens   int64
	Messages int
}

type DailySummary struct {
	Date     time.Time
	Messages int
	Sessions int
	Tokens   int64
	Cost     float64
	FocusTag string // "heavy", "spike", "quiet", or "--" for no tag
}

type MonthDailyReport struct {
	MonthStart    time.Time
	MonthEnd      time.Time
	ActiveDays    int
	TotalMessages int
	TotalSessions int
	TotalTokens   int64
	TotalCost     float64
	Days          []DailySummary
}

type MonthlySummary struct {
	MonthStart    time.Time
	MonthEnd      time.Time
	ActiveDays    int
	TotalMessages int
	TotalSessions int
	TotalTokens   int64
	TotalCost     float64
}

type YearMonthlyReport struct {
	Start         time.Time
	End           time.Time
	ActiveMonths  int
	CurrentStreak int
	BestStreak    int
	TotalMessages int
	TotalSessions int
	TotalTokens   int64
	TotalCost     float64
	Months        []MonthlySummary
}

type DailyLoadKey struct {
	Scope      string
	MonthStart time.Time
	Date       time.Time
	Kind       string // "month" or "day"
}

type windowMessageRow struct {
	MessageID string
	SessionID string
	Title     string
	Directory string
	CreatedAt int64
	Role      string
	Cost      float64
	Summary   bool
	Agent     string
}

type windowPartRow struct {
	MessageID        string
	SessionID        string
	Title            string
	Directory        string
	CreatedAt        int64
	Type             string
	Tool             string
	SkillName        string
	Agent            string
	ProviderID       string
	ModelID          string
	Cost             float64
	MessageCost      float64
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	Summary          bool
	MessageAgent     string
}

type messageEvent struct {
	CreatedAt int64
	Role      string
	Cost      float64
	Summary   bool
	Agent     string
}

type partEvent struct {
	MessageID        string
	CreatedAt        int64
	Type             string
	Tool             string
	SkillName        string
	Agent            string
	ProviderID       string
	ModelID          string
	Cost             float64
	MessageCost      float64
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	Summary          bool
	MessageAgent     string
}
