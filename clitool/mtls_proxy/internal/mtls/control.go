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
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed control.html
var controlHTML []byte

type ControlStatus struct {
	Forwards []ForwardStatus `json:"forwards"`
}

type ForwardStatus struct {
	ID            string     `json:"id"`
	Name          string     `json:"name,omitempty"`
	Running       bool       `json:"running"`
	Mode          string     `json:"mode"`
	Listen        string     `json:"listen"`
	ActualListen  string     `json:"actual_listen,omitempty"`
	Target        string     `json:"target"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	UptimeSeconds int64      `json:"uptime_seconds,omitempty"`
	Cert          *CertInfo  `json:"cert,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type ForwardRequest struct {
	Name           string `json:"name"`
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
	id     string
	name   string
	mode   string
	listen string
	cfg    Config
	cert   *CertInfo

	running   bool
	actual    string
	startedAt time.Time
	cancel    context.CancelFunc
	done      chan struct{}
	err       string
}

type Control struct {
	mu   sync.Mutex
	runs map[string]*controlRun
	seq  int
}

func NewControl() *Control {
	return &Control{runs: make(map[string]*controlRun)}
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
		ctl.Close()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		return nil
	case err := <-errCh:
		ctl.Close()
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
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "forwards": c.Status().Forwards})
	})
	mux.HandleFunc("POST /api/forwards", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"ok": false, "error": "需要 application/json"})
			return
		}
		var req ForwardRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请求解析失败: " + err.Error()})
			return
		}
		fw, err := c.Add(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "forward": fw})
	})
	mux.HandleFunc("POST /api/forwards/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		fw, err := c.StartForward(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "forward": fw})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "forward": fw})
	})
	mux.HandleFunc("POST /api/forwards/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		fw, err := c.StopForward(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "forward": fw})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "forward": fw})
	})
	mux.HandleFunc("DELETE /api/forwards/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := c.DeleteForward(r.PathValue("id")); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	return mux
}

func (c *Control) Add(req ForwardRequest) (ForwardStatus, error) {
	cfg, mode, listen, err := req.toConfig()
	if err != nil {
		return ForwardStatus{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return ForwardStatus{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !portZero(listen) {
		for _, run := range c.runs {
			if run.listen == listen {
				return ForwardStatus{}, fmt.Errorf("监听地址 %s 已被配置 %s 使用", listen, run.label())
			}
		}
	}
	c.seq++
	run := &controlRun{
		id:     fmt.Sprintf("fw-%04d", c.seq),
		name:   strings.TrimSpace(req.Name),
		mode:   mode,
		listen: listen,
		cfg:    cfg,
		cert:   certInfoOf(cfg),
	}
	c.runs[run.id] = run
	return run.status(time.Now()), nil
}

func (c *Control) StartForward(id string) (ForwardStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	run := c.runs[id]
	if run == nil {
		return ForwardStatus{}, fmt.Errorf("未找到转发 %s", id)
	}
	if run.running {
		return run.status(time.Now()), fmt.Errorf("转发 %s 已在运行", run.label())
	}

	var h http.Handler
	var err error
	if run.mode == "tcp" {
		err = validateConfig(run.cfg)
	} else {
		h, err = NewHandler(run.cfg, false)
	}
	if err != nil {
		run.err = err.Error()
		return run.status(time.Now()), err
	}
	ln, err := net.Listen("tcp", run.listen)
	if err != nil {
		run.err = fmt.Sprintf("监听 %s 失败: %v", run.listen, err)
		return run.status(time.Now()), fmt.Errorf("%s", run.err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	run.running = true
	run.actual = ln.Addr().String()
	run.startedAt = time.Now()
	run.cancel = cancel
	run.done = make(chan struct{})
	run.err = ""
	run.cert = certInfoOf(run.cfg)
	log.Printf("转发 %s 已启动：%s %s -> %s", run.label(), strings.ToUpper(run.mode), run.actual, run.cfg.Target)

	go c.serveRun(ctx, ln, run, h)
	return run.status(time.Now()), nil
}

func (c *Control) serveRun(ctx context.Context, ln net.Listener, run *controlRun, h http.Handler) {
	var err error
	if run.mode == "tcp" {
		err = ServeTCPListener(ctx, ln, run.cfg)
	} else {
		err = ServeListener(ctx, ln, h)
	}

	c.mu.Lock()
	if c.runs[run.id] == run {
		run.running = false
		run.actual = ""
		run.cancel = nil
		if err != nil {
			run.err = err.Error()
			log.Printf("转发 %s 退出: %v", run.label(), err)
		}
	}
	done := run.done
	c.mu.Unlock()
	close(done)
}

func (c *Control) StopForward(id string) (ForwardStatus, error) {
	c.mu.Lock()
	run := c.runs[id]
	if run == nil {
		c.mu.Unlock()
		return ForwardStatus{}, fmt.Errorf("未找到转发 %s", id)
	}
	c.mu.Unlock()
	c.stopRun(run)

	c.mu.Lock()
	defer c.mu.Unlock()
	return run.status(time.Now()), nil
}

func (c *Control) DeleteForward(id string) error {
	c.mu.Lock()
	run := c.runs[id]
	if run == nil {
		c.mu.Unlock()
		return fmt.Errorf("未找到转发 %s", id)
	}
	c.mu.Unlock()

	c.stopRun(run)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runs[id] == run {
		delete(c.runs, id)
	}
	return nil
}

func (c *Control) Close() {
	c.mu.Lock()
	runs := make([]*controlRun, 0, len(c.runs))
	for _, run := range c.runs {
		runs = append(runs, run)
	}
	c.mu.Unlock()

	var wg sync.WaitGroup
	for _, run := range runs {
		wg.Add(1)
		go func(run *controlRun) {
			defer wg.Done()
			c.stopRun(run)
		}(run)
	}
	wg.Wait()
}

func (c *Control) stopRun(run *controlRun) {
	c.mu.Lock()
	if !run.running {
		c.mu.Unlock()
		return
	}
	cancel := run.cancel
	done := run.done
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	select {
	case <-done:
	case <-time.After(6 * time.Second):
	}
}

func (c *Control) Status() ControlStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	st := ControlStatus{Forwards: make([]ForwardStatus, 0, len(c.runs))}
	for _, run := range c.runs {
		st.Forwards = append(st.Forwards, run.status(now))
	}
	sort.Slice(st.Forwards, func(i, j int) bool {
		return st.Forwards[i].ID < st.Forwards[j].ID
	})
	return st
}

func (run *controlRun) label() string {
	if run.name != "" {
		return run.name
	}
	return run.id
}

func (run *controlRun) status(now time.Time) ForwardStatus {
	st := ForwardStatus{
		ID:      run.id,
		Name:    run.name,
		Running: run.running,
		Mode:    run.mode,
		Listen:  run.listen,
		Target:  run.cfg.Target,
		Cert:    run.cert,
		Error:   run.err,
	}
	if run.running {
		st.ActualListen = run.actual
		started := run.startedAt
		st.StartedAt = &started
		st.UptimeSeconds = int64(now.Sub(started).Seconds())
		st.Error = ""
	}
	return st
}

func validateConfig(cfg Config) error {
	_, sni, _, err := parseTarget(cfg.Target)
	if err != nil {
		return err
	}
	if cfg.ServerName != "" {
		sni = cfg.ServerName
	}
	if _, _, err := buildTLSConfig(cfg, sni, time.Now()); err != nil {
		return err
	}
	return nil
}

func portZero(listen string) bool {
	_, port, err := net.SplitHostPort(listen)
	return err == nil && port == "0"
}

func (req ForwardRequest) toConfig() (Config, string, string, error) {
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
