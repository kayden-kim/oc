package launch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/port"
)

const (
	healthTimeout  = 5 * time.Second
	healthInterval = 1 * time.Second
	clientTimeout  = 1 * time.Second
	requestTimeout = 5 * time.Second
	retryInterval  = 1 * time.Second
	maxAttempts    = 5
)

func Port(args []string) (int, bool) {
	for i, arg := range args {
		switch {
		case arg == "--port" && i+1 < len(args):
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, false
			}
			return port, true
		case strings.HasPrefix(arg, "--port="):
			port, err := strconv.Atoi(strings.TrimPrefix(arg, "--port="))
			if err != nil {
				return 0, false
			}
			return port, true
		}
	}

	return 0, false
}

func HasPortFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--port" || strings.HasPrefix(arg, "--port=") {
			return true
		}
	}

	return false
}

func ResolvePortArgs(portsRange string, parsePortRange func(string) (int, int, error), selectPort func(int, int, func(int) bool, func(int, int, bool)) port.SelectResult, isPortAvailable func(int) bool, logFn func(string)) []string {
	if portsRange == "" {
		return nil
	}

	minPort, maxPort, err := parsePortRange(portsRange)
	if err != nil {
		if logFn != nil {
			logFn(fmt.Sprintf("Warning: invalid ports config %q: %v", portsRange, err))
			logFn("Launching opencode without --port flag.")
		}
		return nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Port selection: range %d-%d", minPort, maxPort))
	}

	result := selectPort(minPort, maxPort, isPortAvailable, func(attempt, p int, available bool) {
		if logFn == nil {
			return
		}

		status := "in use"
		if available {
			status = "available"
		}
		logFn(fmt.Sprintf("  [%2d/15] port %d ... %s", attempt, p, status))
	})
	if !result.Found {
		if logFn != nil {
			logFn("Warning: no available port found after 15 attempts.")
			logFn("Launching opencode without --port flag.")
		}
		return nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Using port %d", result.Port))
	}

	return []string{"--port", strconv.Itoa(result.Port)}
}

func SendToast(port int, plugins []string) error {
	client := &http.Client{Timeout: clientTimeout}
	healthCtx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()
	if err := waitForServerHealthy(healthCtx, client, port); err != nil {
		return err
	}

	payload := struct {
		Title   string `json:"title,omitempty"`
		Message string `json:"message"`
		Variant string `json:"variant"`
	}{
		Title:   "OC Launcher",
		Message: buildToastMessage(plugins, strconv.Itoa(port)),
		Variant: "info",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := range maxAttempts {
		err = postToast(client, port, body)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxAttempts-1 {
			time.Sleep(retryInterval)
		}
	}

	return lastErr
}

func waitForServerHealthy(ctx context.Context, client *http.Client, port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/global/health", port)
	ticker := time.NewTicker(healthInterval)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		applyServerAuth(req)

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func postToast(client *http.Client, port int, body []byte) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/tui/show-toast", port)
	toastCtx, toastCancel := context.WithTimeout(context.Background(), requestTimeout)
	defer toastCancel()

	req, err := http.NewRequestWithContext(toastCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	applyServerAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("toast request failed: %s", resp.Status)
	}

	var accepted bool
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil && err != io.EOF {
		return err
	}
	if !accepted {
		return fmt.Errorf("toast request returned false")
	}

	return nil
}

func applyServerAuth(req *http.Request) {
	password := os.Getenv("OPENCODE_SERVER_PASSWORD")
	if password == "" {
		return
	}

	username := os.Getenv("OPENCODE_SERVER_USERNAME")
	if username == "" {
		username = "opencode"
	}
	req.SetBasicAuth(username, password)
}

func buildToastMessage(plugins []string, portArg string) string {
	var parts []string
	if len(plugins) > 0 {
		parts = append(parts, fmt.Sprintf("Plugins: %s", strings.Join(plugins, ", ")))
	}
	if portArg != "" {
		parts = append(parts, fmt.Sprintf("Port: %s", portArg))
	}
	if len(parts) == 0 {
		return "OpenCode launched"
	}
	return strings.Join(parts, " | ")
}
