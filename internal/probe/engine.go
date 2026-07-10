package probe

import (
	"context"
	"net"
	"sync"
	"time"
)

type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

type ProbeConfig struct {
	Target     string
	Port       int
	Protocol   string
	Interval   time.Duration
	Count      int
	Continuous bool
	Timeout    time.Duration
}

type ProbeResult struct {
	Timestamp time.Time
	Latency   time.Duration
	Success   bool
	Error     string
}

type Stats struct {
	Sent     int
	Received int
	Lost     int
	LossPct  float64
}

type Percentiles struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

type ProbeEngine struct {
	config     ProbeConfig
	results    []ProbeResult
	resultsMu  sync.Mutex
	sent       int
	received   int
	lost       int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewProbeEngine(config ProbeConfig) *ProbeEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProbeEngine{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		results: make([]ProbeResult, 0),
	}
}

func (e *ProbeEngine) Start(ctx context.Context) {
	e.wg.Add(1)
	go e.run(ctx)
}

func (e *ProbeEngine) Stop() {
	e.cancel()
	e.wg.Wait()
}

func (e *ProbeEngine) run(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.Interval)
	defer ticker.Stop()

	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e.config.Count > 0 && count >= e.config.Count {
				return
			}

			e.wg.Add(1)
			go e.probe(ctx)

			count++
		}
	}
}

func (e *ProbeEngine) probe(parentCtx context.Context) {
	defer e.wg.Done()

	ctx, cancel := context.WithTimeout(parentCtx, e.config.Timeout)
	defer cancel()

	start := time.Now()

	var conn net.Conn
	var err error

	address := net.JoinHostPort(e.config.Target, string(rune(e.config.Port)))

	switch e.config.Protocol {
	case "tcp":
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", address)
	case "udp":
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(ctx, "udp", address)
	default:
		err = ErrInvalidProtocol
	}

	latency := time.Since(start)

	e.resultsMu.Lock()
	e.sent++
	if err != nil {
		e.lost++
		e.results = append(e.results, ProbeResult{
			Timestamp: start,
			Latency:   latency,
			Success:   false,
			Error:     err.Error(),
		})
	} else {
		e.received++
		if conn != nil {
			conn.Close()
		}
		e.results = append(e.results, ProbeResult{
			Timestamp: start,
			Latency:   latency,
			Success:   true,
		})
	}
	e.resultsMu.Unlock()
}

func (e *ProbeEngine) GetResults() []ProbeResult {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	results := make([]ProbeResult, len(e.results))
	copy(results, e.results)
	return results
}

func (e *ProbeEngine) GetStats() (sent, received, lost int, lossPct float64) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	if e.sent > 0 {
		lossPct = float64(e.lost) / float64(e.sent) * 100
	}
	return e.sent, e.received, e.lost, lossPct
}

func (e *ProbeEngine) GetPercentiles() (p50, p95, p99 time.Duration) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()

	latencies := make([]time.Duration, 0, len(e.results))
	for _, r := range e.results {
		if r.Success {
			latencies = append(latencies, r.Latency)
		}
	}

	if len(latencies) == 0 {
		return 0, 0, 0
	}

	// Sort latencies
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	p50 = percentile(latencies, 50)
	p95 = percentile(latencies, 95)
	p99 = percentile(latencies, 99)

	return p50, p95, p99
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(float64(len(sorted)-1) * p / 100)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

var ErrInvalidProtocol = &ProtocolError{}

type ProtocolError struct{}

func (e *ProtocolError) Error() string {
	return "invalid protocol"
}