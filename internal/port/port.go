package port

import (
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"strings"
)

const maxAttempts = 15

// ParseRange parses a port range string like "50000-55000" into min and max values.
// Returns an error if the format is invalid or the range is not sensible.
func ParseRange(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid port range format %q: expected \"min-max\"", s)
	}

	minPort, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid min port %q: %w", parts[0], err)
	}
	maxPort, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid max port %q: %w", parts[1], err)
	}

	if minPort < 1 || maxPort > 65535 {
		return 0, 0, fmt.Errorf("port range %d-%d out of valid range (1-65535)", minPort, maxPort)
	}
	if minPort > maxPort {
		return 0, 0, fmt.Errorf("min port %d is greater than max port %d", minPort, maxPort)
	}

	return minPort, maxPort, nil
}

// IsAvailable checks if a TCP port is available for binding on localhost.
func IsAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// RandomInRange returns a random port number between min and max (inclusive).
func RandomInRange(minPort, maxPort int) int {
	return minPort + rand.IntN(maxPort-minPort+1)
}

// SelectResult holds the result of a port selection attempt.
type SelectResult struct {
	Port     int
	Attempts int
	Found    bool
}

// Select tries to find an available port within the given range.
// It makes up to maxAttempts attempts, picking a random port each time.
// The logFn callback is called for each attempt to allow the caller to display progress.
func Select(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) SelectResult {
	for i := 1; i <= maxAttempts; i++ {
		port := RandomInRange(minPort, maxPort)
		available := checkAvailable(port)
		if logFn != nil {
			logFn(i, port, available)
		}
		if available {
			return SelectResult{Port: port, Attempts: i, Found: true}
		}
	}
	return SelectResult{Found: false, Attempts: maxAttempts}
}
