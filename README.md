# netprobe

[![Go Report Card](https://goreportcard.com/badge/github.com/username/netprobe)](https://goreportcard.com/report/github.com/username/netprobe)
[![CI](https://github.com/username/netprobe/workflows/CI/badge.svg)](https://github.com/username/netprobe/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **CLI network diagnostics tool** combining `mtr`-style path tracing, `tcptraceroute`-style TCP/UDP probing, and latency percentile calculation with JSON/CSV/InfluxDB output for observability pipelines.

## Features

- **TCP/UDP/ICMP probing** with configurable intervals and timeouts
- **Latency percentiles** (p50, p95, p99) for SLO monitoring
- **Multiple output formats**: Table (human), JSON (APIs), CSV (spreadsheets), InfluxDB line protocol (metrics)
- **Continuous monitoring mode** with alerting thresholds
- **TLS certificate validation** and expiry checking
- **Path tracing** with ASN lookup (WIP)
- **Zero dependencies** beyond Go standard library + cobra/viper

## Installation

### From Source

```bash
git clone https://github.com/username/netprobe
cd netprobe
make build
# Binary at ./netprobe
```

### Go Install (when published)

```bash
go install github.com/username/netprobe@latest
```

## Quick Start

```bash
# TCP probe to port 443 (default: 10 probes, 1s interval)
netprobe -t google.com -p 443

# UDP probe to DNS
netprobe -t 8.8.8.8 -p 53 -P udp

# Continuous monitoring with JSON output
netprobe -t api.example.com -p 443 --continuous -o json

# Export to CSV for analysis
netprobe -t target.com -p 80 -c 100 -o csv -f results.csv

# InfluxDB line protocol for Telegraf
netprobe -t service.internal -p 8080 -o influx --continuous
```

## Usage

```
netprobe - Network diagnostic tool with latency percentiles

Usage:
  netprobe [flags]

Flags:
  -t, --target string      Target host or IP (required)
  -p, --port int           Target port (default 80)
  -P, --protocol string    Protocol: tcp or udp (default "tcp")
  -i, --interval int       Interval between probes in ms (default 1000)
  -c, --count int          Number of probes to send (0 = infinite, default 10)
      --continuous         Continuous mode (infinite probes)
  -o, --output string      Output format: table, json, csv, influx (default "table")
      --file string        Output file (stdout if empty)
      --timeout int        Probe timeout in ms (default 5000)
      --tls                Perform TLS handshake probe
      --trace              Run traceroute instead of probe
      --max-hops int       Maximum hops for traceroute (default 30)
  -h, --help               Help for netprobe
```

## Examples

### Basic TCP Probe

```bash
$ netprobe -t github.com -p 443 -c 5

=== Netprobe Results ===
Target: github.com:443 (tcp)
Sent: 5, Received: 5, Lost: 0 (0.0% loss)
Latency: p50=23.4ms, p95=31.2ms, p99=31.2ms

Timestamp                 Latency (ms)  Status
-------------------------  ------------  ------
14:32:10.123               22.4          OK
14:32:11.125               23.1          OK
14:32:12.128               24.0          OK
14:32:13.130               31.2          OK
14:32:14.132               22.8          OK
```

### JSON Output for Automation

```bash
$ netprobe -t api.service.com -p 443 -c 10 -o json
{
  "timestamp": "2024-01-15T14:32:10Z",
  "summary": {
    "sent": 10,
    "received": 10,
    "lost": 0,
    "loss_pct": 0.0,
    "p50_ms": 45,
    "p95_ms": 67,
    "p99_ms": 67
  },
  "results": [
    {"timestamp": "2024-01-15T14:32:10Z", "latency_ms": 42.3, "success": true},
    {"timestamp": "2024-01-15T14:32:11Z", "latency_ms": 45.1, "success": true},
    ...
  ]
}
```

### InfluxDB Line Protocol (Telegraf Integration)

```bash
# Telegraf execd plugin config:
# [[inputs.execd]]
#   command = ["netprobe", "-t", "service.internal", "-p", "8080", "-o", "influx", "--continuous"]
#   signal = "none"

$ netprobe -t myservice -p 8080 -o influx
netprobe_latency,target=myservice,port=8080,protocol=tcp latency_ms=12.3,success=1i 1705330330000000000
netprobe_latency,target=myservice,port=8080,protocol=tcp latency_ms=11.8,success=1i 1705330331000000000
```

### TLS Certificate Check

```bash
$ netprobe -t example.com -p 443 --tls
TLS Probe to example.com:443
Latency: 67.4ms
Certificate:
  Subject: CN=example.com
  Issuer: CN=Let's Encrypt Authority X3
  Valid: 2024-01-01 to 2024-04-01 (76 days remaining)
  SANs: [example.com www.example.com]
```

### Continuous Monitoring with Thresholds

```bash
# Monitor and alert on p99 > 100ms (pseudo-code)
netprobe -t critical-api -p 443 --continuous -o json | \
  jq -r 'select(.summary.p99_ms > 100) | "ALERT: p99 = \(.summary.p99_ms)ms"'
```

## Output Formats

| Format | Use Case |
|--------|----------|
| `table` | Human-readable terminal output |
| `json` | API integration, scripting, jq processing |
| `csv` | Spreadsheet analysis, historical data |
| `influx` | Telegraf/InfluxDB/Grafana metrics pipeline |

## Configuration File

Create `~/.netprobe.yaml`:

```yaml
target: "api.example.com"
port: 443
protocol: "tcp"
interval: 1000
count: 0
continuous: true
output: "influx"
timeout: 5000
```

## Development

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run linter
golangci-lint run

# Build
make build

# Build release (with version info)
make release
```

## Architecture

```
cmd/netprobe/          # CLI entry point (cobra)
├── root.go            # Command definitions, flags, execution
internal/
├── probe/             # Probe engine
│   ├── engine.go      # Core probing logic, stats, percentiles
│   └── tls.go         # TLS/HTTPS probing
├── trace/             # Traceroute (WIP)
│   └── tracer.go      # Hop-by-hop path tracing
├── output/            # Formatters
│   └── formatters.go  # JSON, CSV, InfluxDB, Table
└── pkg/percentile/    # Percentile calculation
```

## Roadmap

- [ ] ICMP probe (requires root/capabilities)
- [ ] Full traceroute with ASN lookup
- [ ] HTTP/HTTPS probe with response validation
- [ ] Prometheus exporter mode
- [ ] WebSocket probe
- [ ] Configuration profiles
- [ ] ARM/Windows binaries via GoReleaser

## License

MIT License - see [LICENSE](LICENSE) for details.