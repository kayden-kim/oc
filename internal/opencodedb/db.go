package opencodedb

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func SQLiteDSN(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
}

func DBPath() (string, error) {
	if name := os.Getenv("OPENCODE_DB"); name != "" {
		if filepath.IsAbs(name) {
			if _, err := os.Stat(name); err == nil {
				return name, nil
			} else {
				return "", err
			}
		}

		root, err := DataDir()
		if err != nil {
			return "", err
		}
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else {
			return "", err
		}
	}

	root, err := DataDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, "opencode.db")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	paths, err := filepath.Glob(filepath.Join(root, "opencode-*.db"))
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", os.ErrNotExist
	}

	sort.SliceStable(paths, func(i, j int) bool {
		left, leftErr := os.Stat(paths[i])
		right, rightErr := os.Stat(paths[j])
		if leftErr != nil || rightErr != nil {
			return paths[i] < paths[j]
		}
		return left.ModTime().After(right.ModTime())
	})

	return paths[0], nil
}

func DataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "opencode"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "opencode"), nil
}

func UnixTimestampToTime(value int64) time.Time {
	switch {
	case value >= 1_000_000_000_000_000_000 || value <= -1_000_000_000_000_000_000:
		return time.Unix(0, value).Local()
	case value >= 1_000_000_000_000_000 || value <= -1_000_000_000_000_000:
		return time.UnixMicro(value).Local()
	case value >= 1_000_000_000_000 || value <= -1_000_000_000_000:
		return time.UnixMilli(value).Local()
	default:
		return time.Unix(value, 0).Local()
	}
}
