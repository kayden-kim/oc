package stats

import "time"

const dayWindow = 30

const defaultSessionGapMinutes = 15

const unlimitedUsageItems = 0

func LoadGlobal() (Report, error) {
	return loadAtWithOptions(time.Now(), "", Options{})
}

func LoadForDir(dir string) (Report, error) {
	return loadAtWithOptions(time.Now(), dir, Options{})
}

func LoadGlobalWithOptions(options Options) (Report, error) {
	return loadAtWithOptions(time.Now(), "", options)
}

func LoadForDirWithOptions(dir string, options Options) (Report, error) {
	return loadAtWithOptions(time.Now(), dir, options)
}

func loadGlobalAt(now time.Time) (Report, error) {
	return loadAtWithOptions(now, "", Options{})
}

func loadForDirAt(dir string, now time.Time) (Report, error) {
	return loadAtWithOptions(now, dir, Options{})
}

func loadForDirAtWithOptions(dir string, now time.Time, options Options) (Report, error) {
	return loadAtWithOptions(now, dir, options)
}

func normalizeOptions(options Options) Options {
	if options.SessionGapMinutes <= 0 {
		options.SessionGapMinutes = defaultSessionGapMinutes
	}
	return options
}

func agentModelUsageKey(agent string, model string) string {
	return agent + "\x00" + model
}

func providerModelUsageKey(provider string, model string) string {
	return provider + "\x00" + model
}
