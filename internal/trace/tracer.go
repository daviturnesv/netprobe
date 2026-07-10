package trace

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HopInfo struct {
	TTL       int
	IP        string
	Hostname  string
	Latency   time.Duration
	ASN       string
	ASNName   string
}

type TraceResult struct {
	Target    string
	Hops      []HopInfo
	Completed bool
}

type Tracer struct {
	target    string
	maxHops   int
	timeout   time.Duration
	protocol  string // "icmp", "tcp", "udp"
}

func NewTracer(target string, maxHops int, timeout time.Duration, protocol string) *Tracer {
	return &Tracer{
		target:   target,
		maxHops:  maxHops,
		timeout:  timeout,
		protocol: protocol,
	}
}

func (t *Tracer) Trace() (*TraceResult, error) {
	result := &TraceResult{
		Target: t.target,
		Hops:   make([]HopInfo, 0, t.maxHops),
	}

	// Resolve target
	ips, err := net.LookupIP(t.target)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target: %w", err)
	}
	targetIP := ips[0].String()

	var wg sync.WaitGroup
	hopChan := make(chan HopInfo, t.maxHops)

	for ttl := 1; ttl <= t.maxHops; ttl++ {
		wg.Add(1)
		go func(ttl int) {
			defer wg.Done()
			hop := t.probeHop(ttl, targetIP)
			if hop != nil {
				hopChan <- *hop
			}
		}(ttl)
	}

	go func() {
		wg.Wait()
		close(hopChan)
	}()

	for hop := range hopChan {
		result.Hops = append(result.Hops, hop)
		if hop.IP == targetIP {
			result.Completed = true
			break
		}
	}

	return result, nil
}

func (t *Tracer) probeHop(ttl int, targetIP string) *HopInfo {
	// Simplified - in production would use raw sockets for ICMP
	// For now, use TCP/UDP with TTL
	var conn net.Conn
	var err error

	dialer := &net.Dialer{
		Timeout: t.timeout,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
			})
		},
	}

	address := fmt.Sprintf("%s:80", t.target)
	if t.protocol == "udp" {
		address = fmt.Sprintf("%s:53", t.target)
	}

	start := time.Now()
	conn, err = dialer.Dial(t.protocol, address)
	latency := time.Since(start)

	if err != nil {
		// Try to get ICMP error
		return &HopInfo{
			TTL:     ttl,
			IP:      "*",
			Latency: latency,
		}
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.TCPAddr)
	hopIP := localAddr.IP.String()

	// Try reverse DNS
	hostname, _ := net.LookupAddr(hopIP)
	host := ""
	if len(hostname) > 0 {
		host = strings.TrimSuffix(hostname[0], ".")
	}

	return &HopInfo{
		TTL:      ttl,
		IP:       hopIP,
		Hostname: host,
		Latency:  latency,
	}
}

// ASN lookup (simplified - in production would use whois or API)
func (t *Tracer) lookupASN(ip string) (asn, name string) {
	// Placeholder - would integrate with ipinfo.io, team-cymru, or similar
	return "", ""
}