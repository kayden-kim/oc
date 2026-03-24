package launch

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newLoopbackServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on loopback: %v", err)
	}
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func loopbackServerPort(server *httptest.Server) string {
	serverURL := strings.TrimPrefix(server.URL, "http://")
	return serverURL[strings.LastIndex(serverURL, ":")+1:]
}

func TestSendToast_PostsOnceWhenEndpointReady(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempts.Add(1)
			fmt.Fprint(w, "true")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	err = sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
		clientTimeout:  50 * time.Millisecond,
		requestTimeout: 50 * time.Millisecond,
		retryInterval:  10 * time.Millisecond,
		readyTimeout:   200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected 1 toast attempt, got %d", got)
	}
}

func TestDefaultToastConfig(t *testing.T) {
	if defaultToastConfig.startupDelay != 5*time.Second {
		t.Fatalf("expected 5s startup delay, got %s", defaultToastConfig.startupDelay)
	}
	if defaultToastConfig.readyTimeout != 10*time.Second {
		t.Fatalf("expected 10s ready timeout, got %s", defaultToastConfig.readyTimeout)
	}
}

func TestSendToast_RetriesUntilSuccess(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempt := attempts.Add(1)
			if attempt < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, "false")
				return
			}
			fmt.Fprint(w, "true")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	err = sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
		clientTimeout:  50 * time.Millisecond,
		requestTimeout: 50 * time.Millisecond,
		retryInterval:  10 * time.Millisecond,
		readyTimeout:   200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 toast attempts, got %d", got)
	}
}

func TestSendToast_RetriesUntilServerStartsListening(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve loopback port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	var attempts atomic.Int32
	var healthCalls atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempts.Add(1)
			fmt.Fprint(w, "true")
		case "/global/health":
			healthCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	server.Listener, err = net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("failed to listen on reserved port: %v", err)
	}
	defer server.Close()

	started := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		server.Start()
		close(started)
	}()

	err = sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
		clientTimeout:  20 * time.Millisecond,
		requestTimeout: 20 * time.Millisecond,
		retryInterval:  10 * time.Millisecond,
		readyTimeout:   300 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	<-started
	if got := attempts.Load(); got == 0 {
		t.Fatal("expected toast attempt after server started")
	}
	if got := healthCalls.Load(); got != 0 {
		t.Fatalf("expected no health probe, got %d", got)
	}
	if got := attempts.Load(); got < 1 {
		t.Fatalf("expected at least one toast attempt, got %d", got)
	}
	if got := attempts.Load(); got > 10 {
		t.Fatalf("expected bounded retries before success, got %d", got)
	}
	if got := attempts.Load(); got < 2 {
		t.Fatalf("expected at least one failed dial before success, got %d attempts", got)
	}
}

func TestSendToast_RetriesWhenEndpointReturnsFalse(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempt := attempts.Add(1)
			if attempt < 3 {
				fmt.Fprint(w, "false")
				return
			}
			fmt.Fprint(w, "true")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	err = sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
		clientTimeout:  50 * time.Millisecond,
		requestTimeout: 50 * time.Millisecond,
		retryInterval:  10 * time.Millisecond,
		readyTimeout:   200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 toast attempts, got %d", got)
	}
}

func TestSendToast_WaitsBeforeFirstAttempt(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempts.Add(1)
			fmt.Fprint(w, "true")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
			startupDelay:   40 * time.Millisecond,
			clientTimeout:  20 * time.Millisecond,
			requestTimeout: 20 * time.Millisecond,
			retryInterval:  10 * time.Millisecond,
			readyTimeout:   200 * time.Millisecond,
		})
	}()

	time.Sleep(20 * time.Millisecond)
	if got := attempts.Load(); got != 0 {
		t.Fatalf("expected no toast attempt before startup delay, got %d", got)
	}

	if err := <-done; err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected one toast attempt after startup delay, got %d", got)
	}
}

func TestSendToast_RetriesForFullDeadlineAfterStartupDelay(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempts.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "false")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	start := time.Now()
	err = sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
		startupDelay:   40 * time.Millisecond,
		clientTimeout:  20 * time.Millisecond,
		requestTimeout: 20 * time.Millisecond,
		retryInterval:  10 * time.Millisecond,
		readyTimeout:   80 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected SendToast to fail after exhausting the retry deadline")
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("expected startup delay plus full retry deadline, got %v", elapsed)
	}
	if got := attempts.Load(); got < 2 {
		t.Fatalf("expected multiple attempts during retry deadline, got %d", got)
	}
}

func TestSendToast_ReturnsErrorAfterDeadline(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tui/show-toast":
			attempts.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "false")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	err = sendToastWithConfig(port, []string{"oh-my-opencode"}, toastConfig{
		clientTimeout:  50 * time.Millisecond,
		requestTimeout: 50 * time.Millisecond,
		retryInterval:  10 * time.Millisecond,
		readyTimeout:   80 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected SendToast to fail after exhausting the ready timeout")
	}
	if got := attempts.Load(); got < 2 {
		t.Fatalf("expected multiple toast attempts before timeout, got %d", got)
	}
}
