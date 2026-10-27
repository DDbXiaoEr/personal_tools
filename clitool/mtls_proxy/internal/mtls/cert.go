package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type CertInfo struct {
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	Serial      string    `json:"serial"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	DNSNames    []string  `json:"dns_names,omitempty"`
	IPAddrs     []string  `json:"ip_addrs,omitempty"`
	IsCA        bool      `json:"is_ca"`
	Expired     bool      `json:"expired"`
	NotYetValid bool      `json:"not_yet_valid"`
}

type InspectResult struct {
	OK         bool       `json:"ok"`
	Error      string     `json:"error,omitempty"`
	ErrorClass string     `json:"error_class,omitempty"`
	KeyOK      bool       `json:"key_ok"`
	ClientCert *CertInfo  `json:"client_cert,omitempty"`
	Chain      []CertInfo `json:"chain,omitempty"`
	CAVerified bool       `json:"ca_verified"`
}

func LoadPair(certFile, keyFile string) (tls.Certificate, error) {
	if strings.TrimSpace(certFile) == "" {
		return tls.Certificate{}, fmt.Errorf("未指定客户端证书")
	}
	if strings.TrimSpace(keyFile) == "" {
		keyFile = certFile
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取证书: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取私钥: %w", err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("加载证书和私钥: %w", err)
	}
	return pair, nil
}

func LoadCertPool(caFile string) (*x509.CertPool, error) {
	if strings.TrimSpace(caFile) == "" {
		return nil, nil
	}
	pemBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("读取 CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("CA 文件不含有效 PEM 证书: %s", caFile)
	}
	return pool, nil
}

func InfoFromCert(c *x509.Certificate, now time.Time) CertInfo {
	if now.IsZero() {
		now = time.Now()
	}
	ips := make([]string, 0, len(c.IPAddresses))
	for _, ip := range c.IPAddresses {
		ips = append(ips, ip.String())
	}
	return CertInfo{
		Subject:     c.Subject.String(),
		Issuer:      c.Issuer.String(),
		Serial:      c.SerialNumber.Text(16),
		NotBefore:   c.NotBefore,
		NotAfter:    c.NotAfter,
		DNSNames:    append([]string{}, c.DNSNames...),
		IPAddrs:     ips,
		IsCA:        c.IsCA,
		Expired:     now.After(c.NotAfter),
		NotYetValid: now.Before(c.NotBefore),
	}
}

func InfosFromTLSCert(pair tls.Certificate, now time.Time) ([]CertInfo, error) {
	out := make([]CertInfo, 0, len(pair.Certificate))
	for i, der := range pair.Certificate {
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("解析证书链第 %d 张: %w", i, err)
		}
		out = append(out, InfoFromCert(c, now))
	}
	return out, nil
}

func Inspect(certFile, keyFile, caFile string) InspectResult {
	now := time.Now()
	pair, err := LoadPair(certFile, keyFile)
	if err != nil {
		return InspectResult{Error: err.Error(), ErrorClass: "load"}
	}
	chain, err := InfosFromTLSCert(pair, now)
	if err != nil {
		return InspectResult{Error: err.Error(), ErrorClass: "parse", KeyOK: true}
	}
	res := InspectResult{KeyOK: true, Chain: chain}
	if len(chain) > 0 {
		leaf := chain[0]
		res.ClientCert = &leaf
		switch {
		case leaf.Expired:
			res.Error = "客户端证书已过期"
			res.ErrorClass = "expired"
		case leaf.NotYetValid:
			res.Error = "客户端证书尚未生效"
			res.ErrorClass = "not_yet_valid"
		default:
			res.OK = true
		}
	}
	if strings.TrimSpace(caFile) == "" {
		return res
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		res.OK = false
		res.Error = err.Error()
		res.ErrorClass = "parse"
		return res
	}
	inter := x509.NewCertPool()
	for i := 1; i < len(pair.Certificate); i++ {
		c, err := x509.ParseCertificate(pair.Certificate[i])
		if err != nil {
			res.OK = false
			res.Error = err.Error()
			res.ErrorClass = "parse"
			return res
		}
		inter.AddCert(c)
	}
	roots, err := LoadCertPool(caFile)
	if err != nil {
		res.OK = false
		res.Error = err.Error()
		res.ErrorClass = "load"
		return res
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inter,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageAny},
	}); err != nil {
		res.OK = false
		res.CAVerified = false
		res.Error = fmt.Sprintf("CA 校验失败: %v", err)
		res.ErrorClass = classify(err)
		return res
	}
	res.CAVerified = true
	if res.ErrorClass == "" {
		res.OK = true
		res.Error = ""
	}
	return res
}

func parseTarget(raw string) (hostport, serverName, path string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("未指定目标地址")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := parseURL(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("目标地址无效: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "tls" {
		return "", "", "", fmt.Errorf("目标必须是 https，当前为 %s", u.Scheme)
	}
	hostport = u.Host
	if !strings.Contains(hostport, ":") {
		hostport = net.JoinHostPort(u.Hostname(), "443")
	}
	serverName = u.Hostname()
	path = u.RequestURI()
	if path == "" {
		path = "/"
	}
	return hostport, serverName, path, nil
}
