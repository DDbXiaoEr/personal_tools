package mtls

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func NewHandler(cfg Config, jsonOut bool) (http.Handler, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	hostport, sni, _, err := parseTarget(cfg.Target)
	if err != nil {
		return nil, err
	}
	if cfg.ServerName != "" {
		sni = cfg.ServerName
	}
	tlsCfg, _, err := buildTLSConfig(cfg, sni, time.Now())
	if err != nil {
		return nil, err
	}

	target := &url.URL{Scheme: "https", Host: hostport}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	proxy.Transport = &http.Transport{
		TLSClientConfig:       tlsCfg,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: cfg.Timeout,
	}
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.URL.Scheme = "https"
		req.URL.Host = hostport
		req.Host = hostport
		req.Header.Del("Origin")
		req.Header.Del("Referer")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Strict-Transport-Security")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("%s %s 代理失败: %v", r.RemoteAddr, r.URL.Path, err)
		http.Error(w, "mTLS 代理失败: "+err.Error(), http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/__mtls/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/__mtls/check", func(w http.ResponseWriter, r *http.Request) {
		res := Probe(cfg)
		wantJSON := jsonOut || strings.Contains(r.Header.Get("Accept"), "application/json")
		if res.OK {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
		var body string
		if wantJSON {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			body, _ = FormatJSON(res)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			body = FormatProbe(res)
		}
		_, _ = w.Write([]byte(body))
		status := "成功"
		if !res.OK {
			status = "失败"
		}
		log.Printf("%s %s %s %dms %s", r.RemoteAddr, r.URL.Path, status, res.DurationMS, res.ErrorClass)
	})
	mux.Handle("/", proxy)
	return mux, nil
}
