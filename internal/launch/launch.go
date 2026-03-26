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

type toastConfig struct {
	clientTimeout     time.Duration
	requestTimeout    time.Duration
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
	postHealthDelay   time.Duration
	readyTimeout      time.Duration
}

var defaultToastConfig = toastConfig{
	clientTimeout:     2 * time.Second,
	requestTimeout:    2 * time.Second,
	initialRetryDelay: 250 * time.Millisecond,
	maxRetryDelay:     2 * time.Second,
	postHealthDelay:   3 * time.Second,
	readyTimeout:      60 * time.Second,
}

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

func SendToast(ctx context.Context, port int, plugins []string) error {
	return sendToastWithConfig(ctx, port, plugins, defaultToastConfig)
}

func sendToastWithConfig(ctx context.Context, port int, plugins []string, cfg toastConfig) error {
	client := &http.Client{Timeout: cfg.clientTimeout}
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

	if ctx == nil {
		ctx = context.Background()
	}

	readyCtx, cancel := context.WithTimeout(ctx, cfg.readyTimeout)
	defer cancel()
	if err := waitForHealthy(readyCtx, client, port, cfg); err != nil {
		return err
	}
	if err := waitForPostHealthDelay(readyCtx, cfg.postHealthDelay); err != nil {
		return err
	}

	var lastErr error
	retryDelay := initialToastRetryDelay(cfg)
	maxRetryDelay := maxToastRetryDelay(cfg, retryDelay)
	for {
		err = postToast(readyCtx, client, port, body, cfg.requestTimeout)
		if err == nil {
			return nil
		}
		lastErr = err

		timer := time.NewTimer(retryDelay)
		select {
		case <-readyCtx.Done():
			timer.Stop()
			return fmt.Errorf("toast endpoint did not become ready within %s: %w", cfg.readyTimeout, lastErr)
		case <-timer.C:
		}

		retryDelay *= 2
		if retryDelay > maxRetryDelay {
			retryDelay = maxRetryDelay
		}
	}
}

func initialToastRetryDelay(cfg toastConfig) time.Duration {
	retryDelay := cfg.initialRetryDelay
	if retryDelay <= 0 {
		retryDelay = 250 * time.Millisecond
	}
	return retryDelay
}

func maxToastRetryDelay(cfg toastConfig, retryDelay time.Duration) time.Duration {
	maxRetryDelay := cfg.maxRetryDelay
	if maxRetryDelay < retryDelay {
		maxRetryDelay = retryDelay
	}
	return maxRetryDelay
}

func waitForHealthy(ctx context.Context, client *http.Client, port int, cfg toastConfig) error {
	retryDelay := initialToastRetryDelay(cfg)
	maxRetryDelay := maxToastRetryDelay(cfg, retryDelay)
	var lastErr error
	for {
		err := getHealth(ctx, client, port, cfg.requestTimeout)
		if err == nil {
			return nil
		}
		lastErr = err

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("health endpoint did not become ready within %s: %w", cfg.readyTimeout, lastErr)
		case <-timer.C:
		}

		retryDelay *= 2
		if retryDelay > maxRetryDelay {
			retryDelay = maxRetryDelay
		}
	}
}

func waitForPostHealthDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func getHealth(ctx context.Context, client *http.Client, port int, requestTimeout time.Duration) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/global/health", port)
	healthCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(healthCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	applyServerAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("health request failed: %s", resp.Status)
	}

	var payload struct {
		Healthy bool   `json:"healthy"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return err
	}
	if !payload.Healthy {
		return fmt.Errorf("health response returned unhealthy")
	}

	return nil
}

func postToast(ctx context.Context, client *http.Client, port int, body []byte, requestTimeout time.Duration) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/tui/show-toast", port)
	toastCtx, toastCancel := context.WithTimeout(ctx, requestTimeout)
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("toast request failed: %s", resp.Status)
	}

	accepted, err := toastAccepted(respBody)
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("toast request returned false")
	}

	return nil
}

func toastAccepted(body []byte) (bool, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return true, nil
	}

	var accepted bool
	if err := json.Unmarshal([]byte(trimmed), &accepted); err == nil {
		return accepted, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		for _, key := range []string{"accepted", "ok", "success", "shown"} {
			value, ok := payload[key]
			if !ok {
				continue
			}
			flag, ok := value.(bool)
			if !ok {
				return false, fmt.Errorf("toast response field %q is not a boolean", key)
			}
			return flag, nil
		}
		return false, fmt.Errorf("toast response object missing acceptance field")
	}

	var text string
	if err := json.Unmarshal([]byte(trimmed), &text); err == nil {
		if strings.EqualFold(text, "true") {
			return true, nil
		}
		if strings.EqualFold(text, "false") {
			return false, nil
		}
	}

	if strings.EqualFold(trimmed, "true") {
		return true, nil
	}
	if strings.EqualFold(trimmed, "false") {
		return false, nil
	}

	return false, fmt.Errorf("toast response was not a recognized success payload")
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
