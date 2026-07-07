use clap::{Parser, Subcommand, ValueEnum};
use serde::{Deserialize, Serialize};
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::net::{TcpStream, UdpSocket};
use tokio::sync::Mutex;
use tokio::time::{sleep, timeout};
use anyhow::{Context, Result};
use csv::Writer;
use indicatif::{ProgressBar, ProgressStyle};
use chrono::{DateTime, Utc};

#[derive(Parser, Debug)]
#[command(name = "netprobe")]
#[command(about = "Network diagnostic CLI - TCP/UDP probe with latency percentiles (p50/p95/p99), JSON/CSV output", long_about = None)]
#[command(version = "0.1.0")]
struct Cli {
    #[command(subcommand)]
    command: Commands,

    /// Target host or IP address
    #[arg(short, long, global = true)]
    host: Option<String>,

    /// Target port
    #[arg(short, long, global = true, default_value = "80")]
    port: u16,

    /// Number of probes to send
    #[arg(short = 'c', long, global = true, default_value = "10")]
    count: usize,

    /// Interval between probes (ms)
    #[arg(short = 'i', long, global = true, default_value = "1000")]
    interval: u64,

    /// Timeout per probe (ms)
    #[arg(short = 't', long, global = true, default_value = "5000")]
    timeout: u64,

    /// Output format
    #[arg(short, long, global = true, value_enum, default_value = "text")]
    format: OutputFormat,

    /// Output file path (default: stdout)
    #[arg(short, long, global = true)]
    output: Option<String>,

    /// Continuous mode (run indefinitely)
    #[arg(short, long, global = true)]
    continuous: bool,

    /// Protocol to use
    #[arg(short, long, global = true, value_enum, default_value = "tcp")]
    protocol: Protocol,

    /// Verbose output
    #[arg(short, long, global = true)]
    verbose: bool,
}

#[derive(Subcommand, Debug)]
enum Commands {
    /// Run TCP/UDP probes
    Probe,
    /// Run continuous monitoring
    Monitor,
    /// Show version
    Version,
}

#[derive(ValueEnum, Clone, Debug, Serialize, Deserialize)]
enum OutputFormat {
    Text,
    Json,
    Csv,
}

#[derive(ValueEnum, Clone, Debug, Serialize, Deserialize)]
enum Protocol {
    Tcp,
    Udp,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct ProbeResult {
    timestamp: DateTime<Utc>,
    host: String,
    port: u16,
    protocol: Protocol,
    sequence: usize,
    success: bool,
    latency_ms: Option<f64>,
    error: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SummaryStats {
    host: String,
    port: u16,
    protocol: Protocol,
    total_probes: usize,
    successful: usize,
    failed: usize,
    loss_percent: f64,
    min_ms: Option<f64>,
    max_ms: Option<f64>,
    mean_ms: Option<f64>,
    p50_ms: Option<f64>,
    p95_ms: Option<f64>,
    p99_ms: Option<f64>,
    stddev_ms: Option<f64>,
}

#[derive(Debug)]
struct ProbeEngine {
    host: String,
    port: u16,
    protocol: Protocol,
    timeout: Duration,
    count: usize,
    interval: Duration,
    continuous: bool,
    verbose: bool,
}

impl ProbeEngine {
    fn new(host: String, port: u16, protocol: Protocol, timeout: Duration, count: usize, interval: Duration, continuous: bool, verbose: bool) -> Self {
        Self {
            host,
            port,
            protocol,
            timeout,
            count,
            interval,
            continuous,
            verbose,
        }
    }

    async fn run(&self) -> Result<Vec<ProbeResult>> {
        let results = Arc::new(Mutex::new(Vec::new()));
        let target_addr = self.resolve_target().await?;

        let pb = if !self.continuous && self.count > 1 {
            Some(ProgressBar::new(self.count as u64))
        } else {
            None
        };

        if let Some(pb) = &pb {
            pb.set_style(ProgressStyle::default_bar()
                .template("{spinner:.green} [{elapsed_precise}] [{bar:40.cyan/blue}] {pos}/{len} ({eta})")
                .unwrap()
                .progress_chars("#>-"));
        }

        let mut sequence = 0;
        loop {
            let result = self.run_single_probe(target_addr, sequence).await;
            results.lock().await.push(result);

            if let Some(pb) = &pb {
                pb.inc(1);
            }

            sequence += 1;

            if !self.continuous && sequence >= self.count {
                break;
            }

            if self.continuous || sequence < self.count {
                sleep(self.interval).await;
            }
        }

        if let Some(pb) = pb {
            pb.finish_with_message("Done");
        }

        Ok(Arc::try_unwrap(results).unwrap().into_inner())
    }

    async fn resolve_target(&self) -> Result<SocketAddr> {
        let addr_str = format!("{}:{}", self.host, self.port);
        let addrs = tokio::net::lookup_host(addr_str).await
            .with_context(|| format!("Failed to resolve {}:{}", self.host, self.port))?;
        Ok(addrs.into_iter().next().unwrap())
    }

    async fn run_single_probe(&self, target: SocketAddr, sequence: usize) -> ProbeResult {
        let start = Instant::now();
        let timestamp = Utc::now();

        let result = match self.protocol {
            Protocol::Tcp => self.tcp_probe(target).await,
            Protocol::Udp => self.udp_probe(target).await,
        };

        let latency = start.elapsed().as_secs_f64() * 1000.0;

        match result {
            Ok(_) => ProbeResult {
                timestamp,
                host: self.host.clone(),
                port: self.port,
                protocol: self.protocol.clone(),
                sequence,
                success: true,
                latency_ms: Some(latency),
                error: None,
            },
            Err(e) => ProbeResult {
                timestamp,
                host: self.host.clone(),
                port: self.port,
                protocol: self.protocol.clone(),
                sequence,
                success: false,
                latency_ms: None,
                error: Some(e.to_string()),
            },
        }
    }

    async fn tcp_probe(&self, target: SocketAddr) -> Result<()> {
        let stream = timeout(self.timeout, TcpStream::connect(target)).await??;
        drop(stream);
        Ok(())
    }

    async fn udp_probe(&self, target: SocketAddr) -> Result<()> {
        let socket = UdpSocket::bind("0.0.0.0:0").await?;
        socket.connect(target).await?;
        
        // Send a small payload
        let payload = b"netprobe";
        socket.send(payload).await?;
        
        // Try to receive response (with timeout)
        let mut buf = [0u8; 1024];
        let _ = timeout(self.timeout, socket.recv(&mut buf)).await.ok();
        
        Ok(())
    }
}

fn calculate_percentile(sorted: &[f64], p: f64) -> Option<f64> {
    if sorted.is_empty() {
        return None;
    }
    let index = (p / 100.0 * (sorted.len() - 1) as f64).round() as usize;
    Some(sorted[index])
}

fn calculate_stats(results: &[ProbeResult]) -> SummaryStats {
    let successful: Vec<f64> = results.iter()
        .filter(|r| r.success)
        .filter_map(|r| r.latency_ms)
        .collect();

    let mut sorted = successful.clone();
    sorted.sort_by(|a, b| a.partial_cmp(b).unwrap());

    let total = results.len();
    let success_count = successful.len();
    let failed = total - success_count;

    let (min_ms, max_ms, mean_ms, p50_ms, p95_ms, p99_ms, stddev_ms) = if !sorted.is_empty() {
        let min = Some(sorted[0]);
        let max = Some(sorted[sorted.len() - 1]);
        let mean = Some(sorted.iter().sum::<f64>() / sorted.len() as f64);
        let p50 = calculate_percentile(&sorted, 50.0);
        let p95 = calculate_percentile(&sorted, 95.0);
        let p99 = calculate_percentile(&sorted, 99.0);
        
        let variance = sorted.iter().map(|x| (x - mean.unwrap()).powi(2)).sum::<f64>() / sorted.len() as f64;
        let stddev = Some(variance.sqrt());
        
        (min, max, mean, p50, p95, p99, stddev)
    } else {
        (None, None, None, None, None, None, None)
    };

    SummaryStats {
        host: results.first().map(|r| r.host.clone()).unwrap_or_default(),
        port: results.first().map(|r| r.port).unwrap_or(0),
        protocol: results.first().map(|r| r.protocol.clone()).unwrap_or(Protocol::Tcp),
        total_probes: total,
        successful: success_count,
        failed,
        loss_percent: if total > 0 { failed as f64 / total as f64 * 100.0 } else { 0.0 },
        min_ms,
        max_ms,
        mean_ms,
        p50_ms,
        p95_ms,
        p99_ms,
        stddev_ms,
    }
}

fn output_text(results: &[ProbeResult], stats: &SummaryStats, verbose: bool) {
    println!("\n=== NetProbe Results ===");
    println!("Target: {}:{} ({})", stats.host, stats.port, format!("{:?}", stats.protocol).to_lowercase());
    println!("Probes: {} | Success: {} | Failed: {} | Loss: {:.1}%", 
        stats.total_probes, stats.successful, stats.failed, stats.loss_percent);
    
    if stats.successful > 0 {
        println!("\nLatency (ms):");
        println!("  Min:     {:.2}", stats.min_ms.unwrap_or(0.0));
        println!("  Max:     {:.2}", stats.max_ms.unwrap_or(0.0));
        println!("  Mean:    {:.2}", stats.mean_ms.unwrap_or(0.0));
        println!("  P50:     {:.2}", stats.p50_ms.unwrap_or(0.0));
        println!("  P95:     {:.2}", stats.p95_ms.unwrap_or(0.0));
        println!("  P99:     {:.2}", stats.p99_ms.unwrap_or(0.0));
        println!("  StdDev:  {:.2}", stats.stddev_ms.unwrap_or(0.0));
    }

    if verbose {
        println!("\nDetailed results:");
        for r in results {
            let status = if r.success { "✓" } else { "✗" };
            let latency = r.latency_ms.map(|l| format!("{:.2}ms", l)).unwrap_or_else(|| "timeout".to_string());
            println!("  {} Seq={} {} {}", status, r.sequence, latency, r.error.as_deref().unwrap_or(""));
        }
    }
}

fn output_json(results: &[ProbeResult], stats: &SummaryStats) -> String {
    let output = serde_json::json!({
        "summary": stats,
        "results": results
    });
    serde_json::to_string_pretty(&output).unwrap()
}

fn output_csv(results: &[ProbeResult], stats: &SummaryStats) -> String {
    let mut wtr = Writer::from_writer(vec![]);
    
    // Write summary as comments
    wtr.write_record(&["# Summary"]).ok();
    wtr.write_record(&["# Host", &stats.host]).ok();
    wtr.write_record(&["# Port", &stats.port.to_string()]).ok();
    wtr.write_record(&["# Protocol", &format!("{:?}", stats.protocol)]).ok();
    wtr.write_record(&["# Total", &stats.total_probes.to_string()]).ok();
    wtr.write_record(&["# Successful", &stats.successful.to_string()]).ok();
    wtr.write_record(&["# Failed", &stats.failed.to_string()]).ok();
    wtr.write_record(&["# Loss %", &format!("{:.1}", stats.loss_percent)]).ok();
    
    // Write headers
    wtr.write_record(&[
        "timestamp", "host", "port", "protocol", "sequence", 
        "success", "latency_ms", "error"
    ]).ok();
    
    for r in results {
        let error_str = r.error.as_deref().unwrap_or("");
        wtr.write_record(&[
            &r.timestamp.to_rfc3339(),
            &r.host,
            &r.port.to_string(),
            &format!("{:?}", r.protocol),
            &r.sequence.to_string(),
            &r.success.to_string(),
            &r.latency_ms.map(|l| format!("{:.2}", l)).unwrap_or_default(),
            error_str,
        ]).ok();
    }
    
    String::from_utf8(wtr.into_inner().unwrap()).unwrap()
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    let host = cli.host.unwrap_or_else(|| "127.0.0.1".to_string());
    let timeout = Duration::from_millis(cli.timeout);
    let interval = Duration::from_millis(cli.interval);

    let engine = ProbeEngine::new(
        host,
        cli.port,
        cli.protocol,
        timeout,
        cli.count,
        interval,
        cli.continuous,
        cli.verbose,
    );

    let results = engine.run().await?;
    let stats = calculate_stats(&results);

    let output = match cli.format {
        OutputFormat::Text => {
            output_text(&results, &stats, cli.verbose);
            String::new()
        }
        OutputFormat::Json => output_json(&results, &stats),
        OutputFormat::Csv => output_csv(&results, &stats),
    };

    if let Some(path) = cli.output {
        if !output.is_empty() {
            tokio::fs::write(&path, output).await
                .with_context(|| format!("Failed to write output to {}", path))?;
            println!("Output written to {}", path);
        }
    } else if !output.is_empty() {
        println!("{}", output);
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_calculate_percentile() {
        let data = vec![1.0, 2.0, 3.0, 4.0, 5.0];
        assert_eq!(calculate_percentile(&data, 50.0), Some(3.0));
        assert_eq!(calculate_percentile(&data, 95.0), Some(5.0));
        assert_eq!(calculate_percentile(&data, 0.0), Some(1.0));
        assert_eq!(calculate_percentile(&data, 100.0), Some(5.0));
    }

    #[test]
    fn test_calculate_percentile_empty() {
        let data: Vec<f64> = vec![];
        assert_eq!(calculate_percentile(&data, 50.0), None);
    }
}