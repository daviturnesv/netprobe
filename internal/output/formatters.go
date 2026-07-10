package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"netprobe/internal/probe"
)

type JSONOutput struct {
	Target      string            `json:"target"`
	Port        int               `json:"port"`
	Protocol    string            `json:"protocol"`
	Timestamp   string            `json:"timestamp"`
	Stats       Stats             `json:"stats"`
	Percentiles Percentiles       `json:"percentiles"`
	Results     []ProbeResultOut  `json:"results"`
}

type Stats struct {
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	Lost     int     `json:"lost"`
	LossPct  float64 `json:"loss_pct"`
}

type Percentiles struct {
	P50 string `json:"p50"`
	P95 string `json:"p95"`
	P99 string `json:"p99"`
}

type ProbeResultOut struct {
	Timestamp string  `json:"timestamp"`
	LatencyMs float64 `json:"latency_ms"`
	Success   bool    `json:"success"`
	Error     string  `json:"error,omitempty"`
}

type JSONFormatter struct{}

func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

func (f *JSONFormatter) FormatProbe(results []probe.ProbeResult, stats probe.Stats, percentiles probe.Percentiles, target string, port int, protocol string) ([]byte, error) {
	output := JSONOutput{
		Target:    target,
		Port:      port,
		Protocol:  protocol,
		Timestamp: time.Now().Format(time.RFC3339),
		Stats: Stats{
			Sent:     stats.Sent,
			Received: stats.Received,
			Lost:     stats.Lost,
			LossPct:  stats.LossPct,
		},
		Percentiles: Percentiles{
			P50: percentiles.P50.String(),
			P95: percentiles.P95.String(),
			P99: percentiles.P99.String(),
		},
		Results: make([]ProbeResultOut, len(results)),
	}

	for i, r := range results {
		output.Results[i] = ProbeResultOut{
			Timestamp: r.Timestamp.Format(time.RFC3339),
			LatencyMs: r.Latency.Seconds() * 1000,
			Success:   r.Success,
			Error:     r.Error,
		}
	}

	return json.MarshalIndent(output, "", "  ")
}

type CSVFormatter struct{}

func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

func (f *CSVFormatter) FormatProbe(results []probe.ProbeResult) ([]byte, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	writer.Write([]string{"timestamp", "latency_ms", "success", "error"})

	for _, r := range results {
		latency := ""
		if r.Success {
			latency = fmt.Sprintf("%.3f", r.Latency.Seconds()*1000)
		}
		writer.Write([]string{
			r.Timestamp.Format(time.RFC3339),
			latency,
			fmt.Sprintf("%v", r.Success),
			r.Error,
		})
	}

	writer.Flush()
	return []byte(sb.String()), writer.Error()
}

type InfluxFormatter struct{}

func NewInfluxFormatter() *InfluxFormatter {
	return &InfluxFormatter{}
}

func (f *InfluxFormatter) FormatProbe(results []probe.ProbeResult, measurement string, tags map[string]string) ([]byte, error) {
	var lines []string

	tagStr := ""
	for k, v := range tags {
		tagStr += fmt.Sprintf(",%s=%s", k, escapeInflux(v))
	}

	for _, r := range results {
		if !r.Success {
			continue
		}
		fields := fmt.Sprintf("latency_ms=%.3f,success=1i", r.Latency.Seconds()*1000)
		line := fmt.Sprintf("%s%s %s %d", measurement, tagStr, fields, r.Timestamp.UnixNano())
		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

func escapeInflux(s string) string {
	s = strings.ReplaceAll(s, " ", "\\ ")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "=", "\\=")
	return s
}

func WriteToFile(data []byte, filename string) error {
	return os.WriteFile(filename, data, 0644)
}