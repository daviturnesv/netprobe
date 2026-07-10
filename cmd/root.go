package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"netprobe/internal/output"
	"netprobe/internal/probe"
)

var (
	cfgFile       string
	target        string
	port          int
	protocol      string
	interval      int
	count         int
	continuous    bool
	outputFormat  string
	outputFile    string
	timeout       int
	tlsProbe      bool
	traceMode     bool
	maxHops       int
)

var rootCmd = &cobra.Command{
	Use:   "netprobe",
	Short: "Network diagnostic tool - TCP/UDP probe with latency percentiles",
	Long: `netprobe is a CLI tool for network diagnostics, similar to mtr + tcptraceroute.
It sends TCP/UDP probes to a target and calculates latency percentiles (p50, p95, p99).
Supports JSON, CSV, and InfluxDB line protocol output for automation and monitoring.`,
	RunE: runProbe,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.netprobe.yaml)")
	rootCmd.PersistentFlags().StringVarP(&target, "target", "t", "", "Target host or IP (required)")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 80, "Target port")
	rootCmd.PersistentFlags().StringVarP(&protocol, "protocol", "P", "tcp", "Protocol: tcp or udp")
	rootCmd.PersistentFlags().IntVarP(&interval, "interval", "i", 1000, "Interval between probes in milliseconds")
	rootCmd.PersistentFlags().IntVarP(&count, "count", "c", 10, "Number of probes to send (0 = infinite)")
	rootCmd.PersistentFlags().BoolVar(&continuous, "continuous", false, "Continuous mode (infinite probes)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, csv, influx")
	rootCmd.PersistentFlags().StringVar(&outputFile, "file", "", "Output file (stdout if empty)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 5000, "Probe timeout in milliseconds")
	rootCmd.PersistentFlags().BoolVar(&tlsProbe, "tls", false, "Perform TLS handshake probe")
	rootCmd.PersistentFlags().BoolVar(&traceMode, "trace", false, "Run traceroute instead of probe")
	rootCmd.PersistentFlags().IntVar(&maxHops, "max-hops", 30, "Maximum hops for traceroute")

	viper.BindPFlag("target", rootCmd.PersistentFlags().Lookup("target"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("protocol", rootCmd.PersistentFlags().Lookup("protocol"))
	viper.BindPFlag("interval", rootCmd.PersistentFlags().Lookup("interval"))
	viper.BindPFlag("count", rootCmd.PersistentFlags().Lookup("count"))
	viper.BindPFlag("continuous", rootCmd.PersistentFlags().Lookup("continuous"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("file", rootCmd.PersistentFlags().Lookup("file"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("tls", rootCmd.PersistentFlags().Lookup("tls"))
	viper.BindPFlag("trace", rootCmd.PersistentFlags().Lookup("trace"))
	viper.BindPFlag("max-hops", rootCmd.PersistentFlags().Lookup("max-hops"))

	rootCmd.MarkPersistentFlagRequired("target")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.SetConfigName(".netprobe")
			viper.SetConfigType("yaml")
		}
	}
	viper.AutomaticEnv()
	viper.ReadInConfig()
}

func runProbe(cmd *cobra.Command, args []string) error {
	// Validate protocol
	if protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("protocol must be 'tcp' or 'udp'")
	}

	// Validate target (basic)
	if _, err := netip.ParseAddr(target); err != nil {
		// Try resolving as hostname
		fmt.Fprintf(os.Stderr, "Resolving %s... ", target)
	}

	// Handle trace mode
	if traceMode {
		return runTrace()
	}

	// Handle TLS probe
	if tlsProbe {
		return runTLSProbe()
	}

	// Standard probe
	return runStandardProbe()
}

func runStandardProbe() error {
	config := probe.ProbeConfig{
		Target:     target,
		Port:       port,
		Protocol:   protocol,
		Interval:   time.Duration(interval) * time.Millisecond,
		Count:      count,
		Continuous: continuous,
		Timeout:    time.Duration(timeout) * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := probe.NewProbeEngine(config)
	engine.Start(ctx)

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	// signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// For simplicity, just wait for completion
	// In production, handle signals properly

	if continuous || count == 0 {
		fmt.Printf("Running continuous probe to %s:%d (%s)... Press Ctrl+C to stop\n", target, port, protocol)
		select {} // Wait forever
	} else {
		fmt.Printf("Probing %s:%d (%s) x%d every %dms\n", target, port, protocol, count, interval)
		// Wait for completion
		time.Sleep(config.Interval * time.Duration(config.Count+2))
	}

	engine.Stop()

	// Get results
	results := engine.GetResults()
	p50, p95, p99 := engine.GetPercentiles()
	sent, received, lost, lossPct := engine.GetStats()

	// Format output
	var data []byte
	var err error

	switch outputFormat {
	case "json":
		formatter := output.NewJSONFormatter()
		data, err = formatter.FormatProbe(results, probe.Stats{Sent: sent, Received: received, Lost: lost, LossPct: lossPct}, probe.Percentiles{P50: p50, P95: p95, P99: p99}, target, port, protocol)
	case "csv":
		formatter := output.NewCSVFormatter()
		data, err = formatter.FormatProbe(results)
	case "influx":
		formatter := output.NewInfluxFormatter()
		tags := map[string]string{
			"target":   target,
			"port":     fmt.Sprintf("%d", port),
			"protocol": protocol,
		}
		data, err = formatter.FormatProbe(results, "netprobe_latency", tags)
	default: // table
		data = []byte(formatTable(results, sent, received, lost, lossPct, p50, p95, p99))
	}

	if err != nil {
		return fmt.Errorf("format error: %w", err)
	}

	if outputFile != "" {
		if err := output.WriteToFile(data, outputFile); err != nil {
			return fmt.Errorf("write file error: %w", err)
		}
		fmt.Printf("Results written to %s\n", outputFile)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func runTrace() error {
	fmt.Printf("Traceroute to %s (max %d hops)...\n", target, maxHops)
	fmt.Println("Traceroute not fully implemented yet (requires raw sockets/ICMP)")
	return nil
}

func runTLSProbe() error {
	fmt.Printf("TLS probe to %s:%d...\n", target, port)
	fmt.Println("TLS probe not fully implemented yet")
	return nil
}

func formatTable(results []probe.ProbeResult, sent, received, lost int, lossPct float64, p50, p95, p99 time.Duration) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== Netprobe Results ===\n"))
	sb.WriteString(fmt.Sprintf("Target: %s:%d (%s)\n", target, port, protocol))
	sb.WriteString(fmt.Sprintf("Sent: %d, Received: %d, Lost: %d (%.1f%% loss)\n", sent, received, lost, lossPct))
	sb.WriteString(fmt.Sprintf("Latency: p50=%v, p95=%v, p99=%v\n\n", p50, p95, p99))

	if len(results) > 0 {
		sb.WriteString("Timestamp                 Latency (ms)  Status\n")
		sb.WriteString(strings.Repeat("-", 55) + "\n")
		for _, r := range results {
			status := "OK"
			latency := "-"
			if r.Success {
				latency = fmt.Sprintf("%.3f", r.Latency.Seconds()*1000)
			} else {
				status = "FAIL"
				if r.Error != "" {
					status = r.Error[:min(20, len(r.Error))]
				}
			}
			sb.WriteString(fmt.Sprintf("%-25s  %-12s  %s\n", r.Timestamp.Format("15:04:05.000"), latency, status))
		}
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}