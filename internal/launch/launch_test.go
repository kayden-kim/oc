package launch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

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

func TestSendToast_PostsOnceAfterHealthResponse(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
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

	err = SendToast(port, []string{"oh-my-opencode"})
	if err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected 1 toast attempt, got %d", got)
	}
}

func TestSendToast_RetriesUntilSuccess(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
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

	err = SendToast(port, []string{"oh-my-opencode"})
	if err != nil {
		t.Fatalf("SendToast returned error: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 toast attempts, got %d", got)
	}
}

func TestSendToast_ReturnsErrorAfterMaxRetries(t *testing.T) {
	var attempts atomic.Int32
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
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

	err = SendToast(port, []string{"oh-my-opencode"})
	if err == nil {
		t.Fatal("expected SendToast to fail after exhausting retries")
	}
	if got := attempts.Load(); got != maxAttempts {
		t.Fatalf("expected %d toast attempts, got %d", maxAttempts, got)
	}
}

func TestWaitForServerHealthy_AcceptsErrorResponse(t *testing.T) {
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/global/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"not ready"}`)
	}))

	port, err := strconv.Atoi(loopbackServerPort(server))
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := &http.Client{Timeout: clientTimeout}

	if err := waitForServerHealthy(ctx, client, port); err != nil {
		t.Fatalf("expected any health response to be accepted, got %v", err)
	}
}

func TestWaitForServerHealthy_RetriesAtOneSecondIntervals(t *testing.T) {
	var calledAt []time.Time
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calledAt = append(calledAt, time.Now())
		if len(calledAt) == 1 {
			return nil, errors.New("connection refused")
		}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("boom")),
			Header:     make(http.Header),
		}, nil
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := waitForServerHealthy(ctx, client, 12345); err != nil {
		t.Fatalf("waitForServerHealthy returned error: %v", err)
	}
	if len(calledAt) != 2 {
		t.Fatalf("expected 2 health attempts, got %d", len(calledAt))
	}
	interval := calledAt[1].Sub(calledAt[0])
	if interval < 900*time.Millisecond {
		t.Fatalf("expected retry interval around 1s, got %v", interval)
	}
}

func TestWaitForServerHealthy_TimesOutAfterFiveSeconds(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()
	err := waitForServerHealthy(ctx, client, 12345)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed < 5*time.Second {
		t.Fatalf("expected health wait to last about 5s, got %v", elapsed)
	}
}
