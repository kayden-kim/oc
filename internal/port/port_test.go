package port

import (
	"net"
	"testing"
)

func TestParseRange_Valid(t *testing.T) {
	tests := []struct {
		input   string
		wantMin int
		wantMax int
	}{
		{"50000-55000", 50000, 55000},
		{"1-65535", 1, 65535},
		{"8080-8080", 8080, 8080},
		{" 3000 - 4000 ", 3000, 4000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			min, max, err := ParseRange(tt.input)
			if err != nil {
				t.Fatalf("ParseRange(%q) returned error: %v", tt.input, err)
			}
			if min != tt.wantMin {
				t.Errorf("min = %d, want %d", min, tt.wantMin)
			}
			if max != tt.wantMax {
				t.Errorf("max = %d, want %d", max, tt.wantMax)
			}
		})
	}
}

func TestParseRange_Invalid(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"abc", "no dash separator"},
		{"100", "single number"},
		{"abc-def", "non-numeric values"},
		{"0-100", "min port below 1"},
		{"100-70000", "max port above 65535"},
		{"5000-4000", "min greater than max"},
		{"", "empty string"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, _, err := ParseRange(tt.input)
			if err == nil {
				t.Fatalf("ParseRange(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestRandomInRange(t *testing.T) {
	min, max := 50000, 55000
	for i := 0; i < 100; i++ {
		p := RandomInRange(min, max)
		if p < min || p > max {
			t.Fatalf("RandomInRange(%d, %d) = %d, out of range", min, max, p)
		}
	}
}

func TestRandomInRange_SingleValue(t *testing.T) {
	p := RandomInRange(8080, 8080)
	if p != 8080 {
		t.Fatalf("RandomInRange(8080, 8080) = %d, want 8080", p)
	}
}

func TestIsAvailable_OpenPort(t *testing.T) {
	// Find a port that is currently available by letting the OS assign one
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Now the port should be available
	if !IsAvailable(port) {
		t.Errorf("expected port %d to be available after closing listener", port)
	}
}

func TestIsAvailable_OccupiedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if IsAvailable(port) {
		t.Errorf("expected port %d to be unavailable while listener is active", port)
	}
}

func TestSelect_FindsAvailablePort(t *testing.T) {
	// All ports are available
	alwaysAvailable := func(int) bool { return true }
	var logged []int

	result := Select(50000, 55000, alwaysAvailable, func(attempt, port int, available bool) {
		logged = append(logged, attempt)
	})

	if !result.Found {
		t.Fatal("expected port to be found")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
	if result.Port < 50000 || result.Port > 55000 {
		t.Errorf("port %d out of range", result.Port)
	}
	if len(logged) != 1 {
		t.Errorf("expected 1 log call, got %d", len(logged))
	}
}

func TestSelect_FindsPortOnThirdAttempt(t *testing.T) {
	callCount := 0
	availableOnThird := func(int) bool {
		callCount++
		return callCount == 3
	}

	result := Select(50000, 55000, availableOnThird, nil)

	if !result.Found {
		t.Fatal("expected port to be found")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
}

func TestSelect_ExhaustsAllAttempts(t *testing.T) {
	neverAvailable := func(int) bool { return false }
	logCount := 0

	result := Select(50000, 55000, neverAvailable, func(attempt, port int, available bool) {
		logCount++
		if available {
			t.Error("log callback reported available, but all ports should be unavailable")
		}
	})

	if result.Found {
		t.Fatal("expected no port to be found")
	}
	if result.Attempts != 15 {
		t.Errorf("expected 15 attempts, got %d", result.Attempts)
	}
	if logCount != 15 {
		t.Errorf("expected 15 log calls, got %d", logCount)
	}
}

func TestSelect_NilLogFn(t *testing.T) {
	alwaysAvailable := func(int) bool { return true }

	// Should not panic with nil logFn
	result := Select(50000, 55000, alwaysAvailable, nil)
	if !result.Found {
		t.Fatal("expected port to be found")
	}
}
