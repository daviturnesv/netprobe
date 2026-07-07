# NetProbe

Network diagnostic CLI - TCP/UDP probe with latency percentiles (p50/p95/p99), JSON/CSV output

## Features

- **TCP/UDP probes** with configurable count, interval, and timeout
- **Latency percentiles** (p50, p95, p99) + mean, min, max, stddev
- **Multiple output formats**: text (human-readable), JSON, CSV
- **Continuous monitoring mode** for long-running diagnostics
- **Cross-platform**: Linux, macOS, Windows (TCP only on Windows)

## Installation

### From Source (Rust)

```bash
git clone https://github.com/daviturnesv/netprobe
cd netprobe
cargo install --path .
```

### Pre-built Binaries

Download from [Releases](https://github.com/daviturnesv/netprobe/releases).

## Usage

```bash
# Basic TCP probe (default: 10 probes to port 80)
netprobe probe --host example.com

# UDP probe with custom count and interval
netprobe probe --host 1.1.1.1 --port 53 --protocol udp --count 20 --interval 500

# JSON output for automation
netprobe probe --host google.com --port 443 --count 10 --format json

# CSV output for spreadsheet analysis
netprobe probe --host 8.8.8.8 --port 53 --count 50 --format csv --output results.csv

# Continuous monitoring mode
netprobe monitor --host 1.1.1.1 --port 53 --interval 1000

# Verbose output showing each probe
netprobe probe --host example.com --count 5 --verbose
```

## Output Examples

### Text (default)
```
=== NetProbe Results ===
Target: 1.1.1.1:53 (tcp)
Probes: 5 | Success: 5 | Failed: 0 | Loss: 0.0%

Latency (ms):
  Min:     23.36
  Max:     32.42
  Mean:    27.43
  P50:     25.32
  P95:     32.42
  P99:     32.42
  StdDev:  4.08
```

### JSON
```json
{
  "results": [
    {
      "timestamp": "2026-07-07T01:06:12.428Z",
      "host": "1.1.1.1",
      "port": 53,
      "protocol": "Tcp",
      "sequence": 0,
      "success": true,
      "latency_ms": 32.422393,
      "error": null
    }
  ],
  "summary": {
    "host": "1.1.1.1",
    "port": 53,
    "protocol": "Tcp",
    "total_probes": 5,
    "successful": 5,
    "failed": 0,
    "loss_percent": 0.0,
    "min_ms": 23.36,
    "max_ms": 32.42,
    "mean_ms": 27.43,
    "p50_ms": 25.32,
    "p95_ms": 32.42,
    "p99_ms": 32.42,
    "stddev_ms": 4.08
  }
}
```

## Options

| Option | Short | Description | Default |
|--------|-------|-------------|---------|
| `--host` | `-h` | Target host or IP | 127.0.0.1 |
| `--port` | `-p` | Target port | 80 |
| `--count` | `-c` | Number of probes | 10 |
| `--interval` | `-i` | Interval between probes (ms) | 1000 |
| `--timeout` | `-t` | Timeout per probe (ms) | 5000 |
| `--format` | `-f` | Output format (text, json, csv) | text |
| `--output` | `-o` | Output file path | stdout |
| `--continuous` | `-c` | Continuous mode | false |
| `--protocol` | `-p` | Protocol (tcp, udp) | tcp |
| `--verbose` | `-v` | Verbose output | false |

## Development

```bash
# Run tests
cargo test

# Build release
cargo build --release

# Lint
cargo clippy

# Format
cargo fmt
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and lint
5. Submit a PR