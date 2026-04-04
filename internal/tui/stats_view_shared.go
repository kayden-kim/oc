package tui

import "github.com/kayden-kim/oc/internal/stats"

func (m Model) renderSharedDetailActivityLines(report stats.WindowReport) []string {
	lines := []string{}
	if len(report.TopAgentModels) > 0 {
		lines = append(lines, "", activitySectionHeader("Agents", len(report.TopAgentModels)))
		lines = append(lines, m.renderAgentModelUsageLines(report.TopAgentModels, int64(report.TotalAgentModelCalls))...)
	}
	if len(report.TopSkills) > 0 {
		lines = append(lines, "", activitySectionHeader("Skills", len(report.TopSkills)))
		lines = append(lines, m.renderUsageLines("count", report.TopSkills, int64(report.TotalSkillCalls))...)
	}
	if len(report.TopTools) > 0 {
		lines = append(lines, "", activitySectionHeader("Tools", len(report.TopTools)))
		lines = append(lines, m.renderUsageLines("count", report.TopTools, int64(report.TotalToolCalls))...)
	}
	return lines
}
