package output

import (
	"testing"
	"time"

	"netprobe/internal/probe"
)

func TestJSONFormatter(t *testing.T) {
	formatter := NewJSONFormatter()

	results := []probe.ProbeResult{
		{Timestamp: time.Now(), Latency: 10 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 20 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 30 * time.Millisecond, Success: false, Error: "timeout"},
	}

	stats := probe.Stats{Sent: 3, Received: 2, Lost: 1, LossPct: 33.33}
	percentiles := probe.Percentiles{
		P50: 20 * time.Millisecond,
		P95: 30 * time.Millisecond,
		P99: 30 * time.Millisecond,
	}

	data, err := formatter.FormatProbe(results, stats, percentiles, "example.com", 443, "tcp")
	if err != nil {
		t.Fatalf("FormatProbe error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty output")
	}
}

func TestCSVFormatter(t *testing.T) {
	formatter := NewCSVFormatter()

	results := []probe.ProbeResult{
		{Timestamp: time.Now(), Latency: 10 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 20 * time.Millisecond, Success: false, Error: "timeout"},
	}

	data, err := formatter.FormatProbe(results)
	if err != nil {
		t.Fatalf("FormatProbe error: %v", err)
	}

	output := string(data)
	if len(output) == 0 {
		t.Fatal("Expected non-empty output")
	}

	// Check for CSV headers
	if !contains(output, "timestamp,latency_ms,success,error") {
		t.Errorf("Missing CSV header, got: %s", output)
	}
}

func TestInfluxFormatter(t *testing.T) {
	formatter := NewInfluxFormatter()

	results := []probe.ProbeResult{
		{Timestamp: time.Now(), Latency: 10 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 20 * time.Millisecond, Success: true},
	}

	tags := map[string]string{
		"target":   "example.com",
		"port":     "443",
		"protocol": "tcp",
	}

	data, err := formatter.FormatProbe(results, "netprobe_latency", tags)
	if err != nil {
		t.Fatalf("FormatProbe error: %v", err)
	}

	output := string(data)
	if len(output) == 0 {
		t.Fatal("Expected non-empty output")
	}

	// Check for Influx line protocol format
	if !contains(output, "netprobe_latency") {
		t.Errorf("Missing measurement name, got: %s", output)
	}
	if !contains(output, "target=example.com") {
		t.Errorf("Missing tags, got: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}