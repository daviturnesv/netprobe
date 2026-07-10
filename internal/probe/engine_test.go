package probe

import (
	"testing"
	"time"
)

func TestPercentileCalculation(t *testing.T) {
	engine := NewProbeEngine(ProbeConfig{
		Target:  "example.com",
		Port:    80,
		Timeout: 5 * time.Second,
	})

	// Add some mock results
	engine.resultsMu.Lock()
	engine.results = []ProbeResult{
		{Timestamp: time.Now(), Latency: 10 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 20 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 30 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 40 * time.Millisecond, Success: true},
		{Timestamp: time.Now(), Latency: 50 * time.Millisecond, Success: true},
	}
	engine.sent = 5
	engine.received = 5
	engine.resultsMu.Unlock()

	p50, p95, p99 := engine.GetPercentiles()

	if p50 != 30*time.Millisecond {
		t.Errorf("Expected p50=30ms, got %v", p50)
	}
	if p95 != 50*time.Millisecond {
		t.Errorf("Expected p95=50ms, got %v", p95)
	}
	if p99 != 50*time.Millisecond {
		t.Errorf("Expected p99=50ms, got %v", p99)
	}
}

func TestStatsCalculation(t *testing.T) {
	engine := NewProbeEngine(ProbeConfig{
		Target:  "example.com",
		Port:    80,
		Timeout: 5 * time.Second,
	})

	engine.resultsMu.Lock()
	engine.sent = 10
	engine.received = 8
	engine.lost = 2
	engine.resultsMu.Unlock()

	sent, received, lost, lossPct := engine.GetStats()

	if sent != 10 {
		t.Errorf("Expected sent=10, got %d", sent)
	}
	if received != 8 {
		t.Errorf("Expected received=8, got %d", received)
	}
	if lost != 2 {
		t.Errorf("Expected lost=2, got %d", lost)
	}
	if lossPct != 20.0 {
		t.Errorf("Expected lossPct=20.0, got %f", lossPct)
	}
}