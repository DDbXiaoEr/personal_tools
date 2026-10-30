package mtls

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed control.html
var controlHTML []byte

type ControlStatus struct {
	Running       bool       `json:"running"`
	Mode          string     `json:"mode,omitempty"`
	Listen        string     `json:"listen,omitempty"`
	Target        string     `json:"target,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	UptimeSeconds int64      `json:"uptime_seconds,omitempty"`
	Cert          *CertInfo  `json:"cert,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type StartRequest struct {
	Mode           string `json:"mode"`
	Listen         string `json:"listen"`
	Target         string `json:"target"`
	CertPath       string `json:"cert_path"`
	KeyPath        string `json:"key_path"`
	CAPath         string `json:"ca_path"`
	CertPEM        string `json:"cert_pem"`
	KeyPEM         string `json:"key_pem"`
	CAPEM          string `json:"ca_pem"`
	Insecure       bool   `json:"insecure"`
	SNI            string `json:"sni"`
	ALPN           string `json:"alpn"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type controlRun struct {
	cancel    context.CancelFunc
	done      chan struct{}
	cfg       Config
	mode      string
	listen    string
	startedAt time.Time
	cert      *CertInfo
}

type Control struct {
	mu        sync.Mutex
	run       *controlRun
	lastError string
}

func NewControl() *Control {
	return &Control{}
}

func ServeControl(ctx context.Context, addr string) error {
	addr, err := normalizeListen(addr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("控制台监听失败: %w", err)
	}
	ctl := NewControl()
	srv := &http.Server{
		Handler:           ctl.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	log.Printf("控制台 http://%s", ln.Addr().String())
	select {
	case <-ctx.Done():
		ctl.Stop()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		return nil
	case err := <-errCh:
		ctl.Stop()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (c *Control) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(controlHTML)
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": c.Status()})
	})
	mux.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"ok": false, "error": "需要 application/json"})
			return
		}
		var req StartRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请求解析失败: " + err.Error()})
			return
		}
		st, err := c.Start(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "status": st})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": st})
	})
	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": c.Stop()})
	})
	return mux
}

func (c *Control) Start(req StartRequest) (ControlStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.run != nil {
		return c.statusLocked(), fmt.Errorf("已有代理在运行，请先停止")
	}
	cfg, mode, listen, err := req.toConfig()
	if err != nil {
		return ControlStatus{}, err
	}
	_, sni, _, err := parseTarget(cfg.Target)
	if err != nil {
		return ControlStatus{}, err
	}
	if cfg.ServerName != "" {
		sni = cfg.ServerName
	}
	if _, _, err := buildTLSConfig(cfg, sni, time.Now()); err != nil {
		return ControlStatus{}, err
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return ControlStatus{}, fmt.Errorf("监听 %s 失败: %w", listen, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &controlRun{
		cancel:    cancel,
		done:      make(chan struct{}),
		cfg:       cfg,
		mode:      mode,
		listen:    ln.Addr().String(),
		startedAt: time.Now(),
		cert:      certInfoOf(cfg),
	}
	c.lastError = ""
	c.run = run
	go c.serveRun(ctx, ln, run)
	return c.statusLocked(), nil
}

func (c *Control) serveRun(ctx context.Context, ln net.Listener, run *controlRun) {
	var err error
	if run.mode == "tcp" {
		err = ServeTCPListener(ctx, ln, run.cfg)
	} else {
		var h http.Handler
		h, err = NewHandler(run.cfg, false)
		if err == nil {
			err = ServeListener(ctx, ln, h)
		} else {
			_ = ln.Close()
		}
	}
	c.mu.Lock()
	if c.run == run {
		c.run = nil
	}
	if err != nil {
		c.lastError = err.Error()
		log.Printf("代理退出: %v", err)
	}
	c.mu.Unlock()
	close(run.done)
}

func (c *Control) Stop() ControlStatus {
	c.mu.Lock()
	run := c.run
	if run == nil {
		st := c.statusLocked()
		c.mu.Unlock()
		return st
	}
	c.mu.Unlock()

	run.cancel()
	select {
	case <-run.done:
	case <-time.After(6 * time.Second):
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked()
}

func (c *Control) Status() ControlStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked()
}

func (c *Control) statusLocked() ControlStatus {
	st := ControlStatus{Error: c.lastError}
	if c.run == nil {
		return st
	}
	st.Running = true
	st.Error = ""
	st.Mode = c.run.mode
	st.Listen = c.run.listen
	st.Target = c.run.cfg.Target
	started := c.run.startedAt
	st.StartedAt = &started
	st.UptimeSeconds = int64(time.Since(started).Seconds())
	st.Cert = c.run.cert
	return st
}

func (req StartRequest) toConfig() (Config, string, string, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "http"
	}
	if mode != "http" && mode != "tcp" {
		return Config{}, "", "", fmt.Errorf("模式必须是 http 或 tcp")
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return Config{}, "", "", fmt.Errorf("请填写目标地址")
	}
	listen := strings.TrimSpace(req.Listen)
	if listen == "" {
		return Config{}, "", "", fmt.Errorf("请填写本地监听端口")
	}
	normListen, err := normalizeListen(listen)
	if err != nil {
		return Config{}, "", "", err
	}
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	cfg := Config{
		Target:     target,
		CertFile:   strings.TrimSpace(req.CertPath),
		KeyFile:    strings.TrimSpace(req.KeyPath),
		CAFile:     strings.TrimSpace(req.CAPath),
		Insecure:   req.Insecure,
		ServerName: strings.TrimSpace(req.SNI),
		Timeout:    timeout,
		ALPN:       splitALPN(req.ALPN),
	}
	if strings.TrimSpace(req.CertPEM) != "" {
		cfg.CertPEM = []byte(req.CertPEM)
		cfg.CertFile = ""
	}
	if strings.TrimSpace(req.KeyPEM) != "" {
		cfg.KeyPEM = []byte(req.KeyPEM)
		cfg.KeyFile = ""
	}
	if strings.TrimSpace(req.CAPEM) != "" {
		cfg.CAPEM = []byte(req.CAPEM)
		cfg.CAFile = ""
	}
	return cfg, mode, normListen, nil
}

func certInfoOf(cfg Config) *CertInfo {
	var (
		pair tls.Certificate
		err  error
	)
	switch {
	case len(cfg.CertPEM) > 0:
		keyPEM := cfg.KeyPEM
		if len(keyPEM) == 0 {
			keyPEM = cfg.CertPEM
		}
		pair, err = tls.X509KeyPair(cfg.CertPEM, keyPEM)
	case cfg.CertFile != "":
		pair, err = LoadPair(cfg.CertFile, cfg.KeyFile)
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	infos, err := InfosFromTLSCert(pair, time.Now())
	if err != nil || len(infos) == 0 {
		return nil
	}
	return &infos[0]
}

func splitALPN(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "方法不允许"})
}
