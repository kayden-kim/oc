package stats

import (
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

type windowReportAccumulator struct {
	report          WindowReport
	sessionAgg      map[string]*SessionUsage
	modelAgg        map[string]*ModelUsage
	projectAgg      map[string]projectUsage
	agentAgg        map[string]int
	agentModelAgg   map[string]int
	skillAgg        map[string]int
	toolAgg         map[string]int
	seenMessageCost map[string]struct{}
	seenSessions    map[string]struct{}
	eventTimes      []int64
	loc             *time.Location
}

func newWindowReportAccumulator(label string, start, end time.Time) *windowReportAccumulator {
	loc := start.Location()
	if loc == nil {
		loc = time.Local
	}
	return &windowReportAccumulator{
		report:          WindowReport{Label: label, Start: start, End: end},
		sessionAgg:      map[string]*SessionUsage{},
		modelAgg:        map[string]*ModelUsage{},
		projectAgg:      map[string]projectUsage{},
		agentAgg:        map[string]int{},
		agentModelAgg:   map[string]int{},
		skillAgg:        map[string]int{},
		toolAgg:         map[string]int{},
		seenMessageCost: map[string]struct{}{},
		seenSessions:    map[string]struct{}{},
		eventTimes:      make([]int64, 0),
		loc:             loc,
	}
}

func (a *windowReportAccumulator) addMessage(row windowMessageRow) {
	if !isWindowAssistantMessage(row) {
		return
	}

	a.report.Messages++
	a.report.Cost += row.Cost
	if row.Agent != "" {
		a.report.TotalSubtasks++
		a.agentAgg[row.Agent]++
	}
	a.trackSessionEvent(row.SessionID, row.Title, row.CreatedAt)
	session := ensureSessionUsage(a.sessionAgg, row.SessionID, row.Title)
	session.Messages++
	session.Cost += row.Cost
}

func (a *windowReportAccumulator) addPart(row windowPartRow) error {
	if !isWindowUsagePart(row) {
		return nil
	}

	a.trackSessionEvent(row.SessionID, row.Title, row.CreatedAt)
	session := ensureSessionUsage(a.sessionAgg, row.SessionID, row.Title)
	if row.Type == "tool" {
		a.report.TotalToolCalls++
		if row.Tool != "" {
			a.toolAgg[row.Tool]++
		}
		if row.Tool == "skill" {
			a.report.TotalSkillCalls++
			if row.SkillName != "" {
				a.skillAgg[row.SkillName]++
			}
		}
	}
	if row.Type != "step-finish" {
		return nil
	}

	totalTokens := windowPartTotalTokens(row)
	model := a.addPartTokens(row, session, totalTokens)
	a.addAgentModelUsage(row)
	a.addProjectTokens(row.Directory, totalTokens)

	if row.MessageCost > 0 {
		if _, ok := a.seenMessageCost[row.MessageID]; ok {
			return nil
		}
		a.seenMessageCost[row.MessageID] = struct{}{}
		model.Cost += row.MessageCost
		a.addProjectCost(row.Directory, row.MessageCost)
		return nil
	}

	cost, err := estimateWindowPartCost(row, "window")
	if err != nil {
		return err
	}
	a.report.Cost += cost
	model.Cost += cost
	session.Cost += cost
	a.addProjectCost(row.Directory, cost)
	return nil
}

func (a *windowReportAccumulator) addPartTokens(row windowPartRow, session *SessionUsage, totalTokens int64) *ModelUsage {
	modelKey := modelLabel(row.ProviderID, row.ModelID)
	if strings.TrimSpace(modelKey) == "" {
		modelKey = row.ModelID
	}
	model := ensureModelUsage(a.modelAgg, modelKey)
	a.report.InputTokens += row.InputTokens
	a.report.OutputTokens += row.OutputTokens
	a.report.CacheReadTokens += row.CacheReadTokens
	a.report.CacheWriteTokens += row.CacheWriteTokens
	a.report.ReasoningTokens += row.ReasoningTokens
	model.InputTokens += row.InputTokens
	model.OutputTokens += row.OutputTokens
	model.CacheReadTokens += row.CacheReadTokens
	model.CacheWriteTokens += row.CacheWriteTokens
	model.ReasoningTokens += row.ReasoningTokens
	model.TotalTokens += totalTokens
	a.report.Tokens += totalTokens
	session.Tokens += totalTokens
	stamp := opencodedb.UnixTimestampToTime(row.CreatedAt).In(a.loc)
	a.report.HalfHourSlots[stamp.Hour()*2+stamp.Minute()/30] += totalTokens
	return model
}

func (a *windowReportAccumulator) addAgentModelUsage(row windowPartRow) {
	agentName := strings.TrimSpace(row.Agent)
	if agentName == "" {
		agentName = strings.TrimSpace(row.MessageAgent)
	}
	modelName := strings.TrimSpace(row.ModelID)
	if agentName == "" || modelName == "" || strings.EqualFold(agentName, "compaction") {
		return
	}
	a.agentModelAgg[agentModelUsageKey(agentName, modelName)]++
	a.report.TotalAgentModelCalls++
}

func (a *windowReportAccumulator) addProjectTokens(directory string, totalTokens int64) {
	projectKey := normalizeProjectUsageKey(directory)
	if projectKey == "" {
		return
	}
	usage := a.projectAgg[projectKey]
	usage.Tokens += totalTokens
	a.projectAgg[projectKey] = usage
}

func (a *windowReportAccumulator) addProjectCost(directory string, cost float64) {
	projectKey := normalizeProjectUsageKey(directory)
	if projectKey == "" {
		return
	}
	usage := a.projectAgg[projectKey]
	usage.Cost += cost
	a.projectAgg[projectKey] = usage
}

func (a *windowReportAccumulator) trackSessionEvent(sessionID, title string, createdAt int64) {
	a.seenSessions[sessionID] = struct{}{}
	a.eventTimes = append(a.eventTimes, createdAt)
	ensureSessionUsage(a.sessionAgg, sessionID, title)
}

func (a *windowReportAccumulator) finalize() WindowReport {
	a.report.Sessions = len(a.seenSessions)
	a.report.Models = collectSortedModels(a.modelAgg)
	a.report.AllSessions = collectSortedSessions(a.sessionAgg)
	a.report.TopSessions = append([]SessionUsage(nil), a.report.AllSessions...)
	if len(a.report.TopSessions) > 8 {
		a.report.TopSessions = a.report.TopSessions[:8]
	}
	a.report.TopProjects = topUsageAmountsWithCosts(a.projectAgg, unlimitedUsageItems)
	a.report.TopAgents = topUsageCounts(a.agentAgg, unlimitedUsageItems)
	a.report.TopAgentModels = topUsageCounts(a.agentModelAgg, unlimitedUsageItems)
	a.report.TopSkills = topUsageCounts(a.skillAgg, unlimitedUsageItems)
	a.report.TopTools = topUsageCounts(a.toolAgg, unlimitedUsageItems)
	a.report.UniqueProjectCount = len(a.projectAgg)
	for _, usage := range a.projectAgg {
		a.report.TotalProjectCost += usage.Cost
	}
	a.report.ActiveMinutes = computeSessionMinutes(a.eventTimes, defaultSessionGapMinutes)
	return a.report
}
