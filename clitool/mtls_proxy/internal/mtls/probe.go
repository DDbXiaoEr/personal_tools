package mtls

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Config struct {
	Target     string
	CertFile   string
	KeyFile    string
	CAFile     string
	Insecure   bool
	ServerName string
	Timeout    time.Duration
	Method     string
	Path       string
	ALPN       []string
}

type Result struct {
	OK          bool       `json:"ok"`
	Error       string     `json:"error,omitempty"`
	ErrorClass  string     `json:"error_class,omitempty"`
	DurationMS  int64      `json:"duration_ms"`
	Target      string     `json:"target"`
	TLSVersion  string     `json:"tls_version,omitempty"`
	CipherSuite string     `json:"cipher_suite,omitempty"`
	ServerName  string     `json:"server_name,omitempty"`
	ClientCert  *CertInfo  `json:"client_cert,omitempty"`
	PeerCerts   []CertInfo `json:"peer_certs,omitempty"`
	HTTPStatus  int        `json:"http_status,omitempty"`
	HTTPProto   string     `json:"http_proto,omitempty"`
	InspectedAt time.Time  `json:"inspected_at"`
}

func Probe(cfg Config) Result {
	now := time.Now()
	res := Result{Target: cfg.Target, InspectedAt: now}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	hostport, sni, urlPath, err := parseTarget(cfg.Target)
	if err != nil {
		res.Error = err.Error()
		res.ErrorClass = "config"
		return res
	}
	if cfg.ServerName != "" {
		sni = cfg.ServerName
	}
	res.ServerName = sni
	if cfg.Path != "" {
		urlPath = cfg.Path
		if urlPath[0] != '/' {
			urlPath = "/" + urlPath
		}
	}

	tlsCfg, clientInfos, err := buildTLSConfig(cfg, sni, now)
	if err != nil {
		res.Error = err.Error()
		res.ErrorClass = "load"
		return res
	}
	if len(clientInfos) > 0 {
		leaf := clientInfos[0]
		res.ClientCert = &leaf
		if leaf.Expired {
			res.Error = "客户端证书已过期"
			res.ErrorClass = "expired"
			return res
		}
		if leaf.NotYetValid {
			res.Error = "客户端证书尚未生效"
			res.ErrorClass = "not_yet_valid"
			return res
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	start := time.Now()

	if cfg.Method == "" {
		err = handshake(ctx, hostport, tlsCfg, &res, now)
	} else {
		err = httpProbe(ctx, cfg, hostport, sni, urlPath, tlsCfg, &res, now)
	}
	res.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		res.OK = false
		res.Error = err.Error()
		if res.ErrorClass == "" {
			res.ErrorClass = classify(err)
		}
		return res
	}
	res.OK = true
	return res
}

func buildTLSConfig(cfg Config, sni string, now time.Time) (*tls.Config, []CertInfo, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         sni,
		InsecureSkipVerify: cfg.Insecure,
		NextProtos:         append([]string{}, cfg.ALPN...),
	}
	var infos []CertInfo
	if cfg.CertFile != "" {
		pair, err := LoadPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{pair}
		infos, err = InfosFromTLSCert(pair, now)
		if err != nil {
			return nil, nil, err
		}
	}
	if cfg.CAFile != "" {
		pool, err := LoadCertPool(cfg.CAFile)
		if err != nil {
			return nil, nil, err
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, infos, nil
}

func handshake(ctx context.Context, hostport string, tlsCfg *tls.Config, res *Result, now time.Time) error {
	d := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    tlsCfg,
	}
	conn, err := d.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return err
	}
	defer conn.Close()
	tc, ok := conn.(*tls.Conn)
	if !ok {
		return fmt.Errorf("非 TLS 连接")
	}
	fillFromState(res, tc.ConnectionState(), now)
	return nil
}

func httpProbe(ctx context.Context, cfg Config, hostport, sni, urlPath string, tlsCfg *tls.Config, res *Result, now time.Time) error {
	var (
		mu      sync.Mutex
		tlsConn *tls.Conn
	)
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    tlsCfg,
	}
	transport := &http.Transport{
		TLSClientConfig:   tlsCfg,
		DisableKeepAlives: true,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			tc, ok := c.(*tls.Conn)
			if !ok {
				c.Close()
				return nil, fmt.Errorf("非 TLS 连接")
			}
			mu.Lock()
			tlsConn = tc
			mu.Unlock()
			return tc, nil
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	endpoint := &url.URL{Scheme: "https", Host: hostport, Path: urlPath}
	if sni != "" && net.ParseIP(sni) == nil {
		endpoint.Host = hostport
	}
	req, err := http.NewRequestWithContext(ctx, cfg.Method, endpoint.String(), nil)
	if err != nil {
		return err
	}
	if sni != "" {
		req.Host = sni
	}
	resp, err := client.Do(req)
	mu.Lock()
	if tlsConn != nil {
		fillFromState(res, tlsConn.ConnectionState(), now)
	}
	mu.Unlock()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	res.HTTPStatus = resp.StatusCode
	res.HTTPProto = resp.Proto
	return nil
}

func fillFromState(res *Result, st tls.ConnectionState, now time.Time) {
	res.TLSVersion = tlsVersionName(st.Version)
	res.CipherSuite = tls.CipherSuiteName(st.CipherSuite)
	if res.ServerName == "" {
		res.ServerName = st.ServerName
	}
	res.PeerCerts = make([]CertInfo, 0, len(st.PeerCertificates))
	for _, c := range st.PeerCertificates {
		res.PeerCerts = append(res.PeerCerts, InfoFromCert(c, now))
	}
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
