package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

type TLSProbeResult struct {
	ProbeResult
	CertificateInfo *CertificateInfo
	HTTPStatus      int
	HTTPBody        string
}

type CertificateInfo struct {
	Subject       string
	Issuer        string
	NotBefore     time.Time
	NotAfter      time.Time
	DaysUntilExpiry int
	DNSNames      []string
}

func ProbeTLS(target string, port int, timeout time.Duration) (*TLSProbeResult, error) {
	address := fmt.Sprintf("%s:%d", target, port)
	start := time.Now()

	result := &TLSProbeResult{
		ProbeResult: ProbeResult{
			Timestamp: start,
		},
	}

	// TLS handshake
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		InsecureSkipVerify: true, // We'll verify manually
	})

	latency := time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Latency = latency
		return result, nil
	}
	defer conn.Close()

	// Get certificate info
	state := conn.ConnectionState()
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		result.CertificateInfo = &CertificateInfo{
			Subject:   cert.Subject.String(),
			Issuer:    cert.Issuer.String(),
			NotBefore: cert.NotBefore,
			NotAfter:  cert.NotAfter,
			DaysUntilExpiry: int(time.Until(cert.NotAfter).Hours() / 24),
			DNSNames:  cert.DNSNames,
		}
	}

	result.Success = true
	result.Latency = latency

	// Optional: HTTP probe
	httpResult, err := probeHTTP(target, port, timeout)
	if err == nil {
		result.HTTPStatus = httpResult.HTTPStatus
		result.HTTPBody = httpResult.HTTPBody
	}

	return result, nil
}

type HTTPProbeResult struct {
	HTTPStatus int
	HTTPBody   string
}

func probeHTTP(target string, port int, timeout time.Duration) (*HTTPProbeResult, error) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	url := fmt.Sprintf("https://%s:%d/", target, port)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return &HTTPProbeResult{
		HTTPStatus: resp.StatusCode,
	}, nil
}