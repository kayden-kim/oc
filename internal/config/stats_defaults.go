package config

const (
	DefaultActivityMediumTokens int64 = 1_000_000
	DefaultActivityHighTokens   int64 = 5_000_000
	DefaultSessionGapMinutes          = 15
	DefaultStatsScope                 = "global"
)

func NormalizeStatsConfig(cfg StatsConfig) StatsConfig {
	if cfg.MediumTokens <= 0 {
		cfg.MediumTokens = DefaultActivityMediumTokens
	}
	if cfg.HighTokens <= 0 {
		cfg.HighTokens = DefaultActivityHighTokens
	}
	if cfg.HighTokens < cfg.MediumTokens {
		cfg.HighTokens = cfg.MediumTokens
	}
	if cfg.SessionGapMinutes <= 0 {
		cfg.SessionGapMinutes = DefaultSessionGapMinutes
	}
	if cfg.DefaultScope != "project" {
		cfg.DefaultScope = DefaultStatsScope
	}
	return cfg
}
