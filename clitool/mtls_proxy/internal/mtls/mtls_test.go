package mtls

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInspectOK(t *testing.T) {
	b := mustBundle(t)
	res := Inspect(b.ClientCert, b.ClientKey, b.CACert)
	if !res.OK {
		t.Fatalf("expected ok, got %+v", res)
	}
	if !res.KeyOK || !res.CAVerified {
		t.Fatalf("expected key+ca ok: %+v", res)
	}
	if res.ClientCert == nil || res.ClientCert.Subject == "" {
		t.Fatalf("missing client cert info")
	}
}

func TestInspectExpired(t *testing.T) {
	b := mustBundle(t)
	res := Inspect(b.Expired, b.ExpiredKey, b.CACert)
	if res.OK {
		t.Fatalf("expired cert should fail")
	}
	if res.ErrorClass != "expired" {
		t.Fatalf("class = %s, want expired; %+v", res.ErrorClass, res)
	}
}

func TestInspectWrongCA(t *testing.T) {
	b := mustBundle(t)
	res := Inspect(b.ClientCert, b.ClientKey, b.OtherCA)
	if res.OK {
		t.Fatalf("wrong ca should fail")
	}
	if res.ErrorClass != "unknown_authority" {
		t.Fatalf("class = %s, want unknown_authority; %+v", res.ErrorClass, res)
	}
}

func TestInspectMismatchedKey(t *testing.T) {
	b := mustBundle(t)
	res := Inspect(b.ClientCert, b.ServerKey, "")
	if res.OK {
		t.Fatalf("mismatched key should fail")
	}
	if res.ErrorClass != "load" {
		t.Fatalf("class = %s, want load", res.ErrorClass)
	}
}

func TestProbeHandshakeOK(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestServer(b.ServerCert, b.ServerKey, b.CACert, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	res := Probe(Config{
		Target:   "https://" + addr,
		CertFile: b.ClientCert,
		KeyFile:  b.ClientKey,
		CAFile:   b.CACert,
		Timeout:  3 * time.Second,
	})
	if !res.OK {
		t.Fatalf("probe failed: %+v", res)
	}
	if res.TLSVersion == "" || len(res.PeerCerts) == 0 {
		t.Fatalf("missing tls details: %+v", res)
	}
}

func TestProbeRejectsBadClient(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestServer(b.ServerCert, b.ServerKey, b.CACert, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	res := Probe(Config{
		Target:   "https://" + addr,
		CertFile: b.Expired,
		KeyFile:  b.ExpiredKey,
		CAFile:   b.CACert,
		Timeout:  3 * time.Second,
	})
	if res.OK {
		t.Fatalf("expired client should fail")
	}
	if res.ErrorClass != "expired" {
		t.Fatalf("class = %s, want expired", res.ErrorClass)
	}
}

func TestProbeUnknownAuthority(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestServer(b.ServerCert, b.ServerKey, b.CACert, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	res := Probe(Config{
		Target:   "https://" + addr,
		CertFile: b.ClientCert,
		KeyFile:  b.ClientKey,
		CAFile:   b.CACert,
		Timeout:  3 * time.Second,
	})
	if !res.OK {
		t.Fatalf("probe with matching ca should succeed: %+v", res)
	}

	res = Probe(Config{
		Target:   "https://" + addr,
		CertFile: b.ClientCert,
		KeyFile:  b.ClientKey,
		CAFile:   b.OtherCA,
		Timeout:  3 * time.Second,
	})
	if res.OK {
		t.Fatalf("wrong server ca should fail")
	}
	if res.ErrorClass != "unknown_authority" {
		t.Fatalf("class = %s, want unknown_authority; err=%s", res.ErrorClass, res.Error)
	}
}

func TestProbeHTTP(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestHTTPS(b.ServerCert, b.ServerKey, b.CACert, true, 200, "pong")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	res := Probe(Config{
		Target:   "https://" + addr,
		CertFile: b.ClientCert,
		KeyFile:  b.ClientKey,
		CAFile:   b.CACert,
		Method:   "GET",
		Timeout:  3 * time.Second,
	})
	if !res.OK {
		t.Fatalf("http probe failed: %+v", res)
	}
	if res.HTTPStatus != 200 {
		t.Fatalf("status = %d", res.HTTPStatus)
	}
}

func TestHandlerProxyAndCheck(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestHTTPS(b.ServerCert, b.ServerKey, b.CACert, true, 200, "pong")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	h, err := NewHandler(Config{
		Target:   "https://" + addr,
		CertFile: b.ClientCert,
		KeyFile:  b.ClientKey,
		CAFile:   b.CACert,
		Timeout:  3 * time.Second,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8000")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "pong") {
		t.Fatalf("proxy status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/__mtls/check", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("check status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"ok": true`) {
		t.Fatalf("check body=%s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/__mtls/health", nil))
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != "ok" {
		t.Fatalf("health: %d %s", rr.Code, rr.Body.String())
	}
}

func TestServeListen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	h, err := NewHandler(Config{Target: "https://127.0.0.1:1", Timeout: 50 * time.Millisecond}, false)
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeListener(ctx, ln, h)
	}()
	resp, err := http.Get("http://" + addr + "/__mtls/health")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("health %d", resp.StatusCode)
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not stop")
	}
}

func TestParseTarget(t *testing.T) {
	hp, sni, path, err := parseTarget("https://api.example.com:8443/v1")
	if err != nil {
		t.Fatal(err)
	}
	if hp != "api.example.com:8443" || sni != "api.example.com" || path != "/v1" {
		t.Fatalf("%s %s %s", hp, sni, path)
	}
	hp, sni, path, err = parseTarget("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if hp != "example.com:443" || sni != "example.com" || path != "/" {
		t.Fatalf("%s %s %s", hp, sni, path)
	}
}

func TestFormatJSON(t *testing.T) {
	s, err := FormatJSON(Result{OK: true, Target: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `"ok": true`) {
		t.Fatalf("%s", s)
	}
}

func TestFormatProbe(t *testing.T) {
	out := FormatProbe(Result{
		OK:         false,
		Error:      "boom",
		ErrorClass: "handshake",
		Target:     "https://x",
		ClientCert: &CertInfo{Subject: "CN=client", Issuer: "CN=ca"},
	})
	if !strings.Contains(out, "失败") || !strings.Contains(out, "boom") {
		t.Fatalf("%s", out)
	}
}

func TestInsecureSkipVerify(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestServer(b.ServerCert, b.ServerKey, b.CACert, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	res := Probe(Config{
		Target:   "https://" + addr,
		CertFile: b.ClientCert,
		KeyFile:  b.ClientKey,
		Insecure: true,
		Timeout:  3 * time.Second,
	})
	if !res.OK {
		t.Fatalf("insecure should succeed: %+v", res)
	}
}

func TestMissingCert(t *testing.T) {
	res := Inspect("/no/such.crt", "", "")
	if res.OK || res.ErrorClass != "load" {
		t.Fatalf("%+v", res)
	}
}

func TestNormalizeListen(t *testing.T) {
	got, err := normalizeListen("8080")
	if err != nil || got != "127.0.0.1:8080" {
		t.Fatalf("%s %v", got, err)
	}
}

func TestInspectNoCAStillParses(t *testing.T) {
	b := mustBundle(t)
	res := Inspect(b.ClientCert, b.ClientKey, "")
	if !res.OK {
		t.Fatalf("%+v", res)
	}
	if res.CAVerified {
		t.Fatalf("ca should not be verified")
	}
}

func TestTCPTunnelEcho(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestEcho(b.ServerCert, b.ServerKey, b.CACert, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeTCPListener(ctx, ln, Config{
			Target:   "https://" + addr,
			CertFile: b.ClientCert,
			KeyFile:  b.ClientKey,
			CAFile:   b.CACert,
			Timeout:  3 * time.Second,
		})
	}()

	c, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("hello-mtls")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 10)
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := io.ReadFull(c, buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf[:n]) != "hello-mtls" {
		t.Fatalf("got %q", buf[:n])
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel did not stop")
	}
}

func TestTCPTunnelRejectsBadClient(t *testing.T) {
	b := mustBundle(t)
	addr, closeFn, err := StartTestEcho(b.ServerCert, b.ServerKey, b.CACert, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = ServeTCPListener(ctx, ln, Config{
			Target:   "https://" + addr,
			CertFile: b.Expired,
			KeyFile:  b.ExpiredKey,
			CAFile:   b.CACert,
			Timeout:  time.Second,
		})
	}()

	c, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _ = c.Write([]byte("x"))
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 8)
	n, err := c.Read(buf)
	if n != 0 && err == nil {
		t.Fatalf("expected closed connection, got %q", buf[:n])
	}
}

func mustBundle(t *testing.T) TestBundle {
	t.Helper()
	dir := t.TempDir()
	b, err := GenerateTestBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.ClientCert); err != nil {
		t.Fatal(err)
	}
	return b
}
