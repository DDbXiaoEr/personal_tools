package mtls

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestControlAddStartStopDelete(t *testing.T) {
	b := mustBundle(t)
	ctl := NewControl()
	defer ctl.Close()

	fw, err := ctl.Add(ForwardRequest{
		Name:           "本地调试",
		Mode:           "http",
		Listen:         "127.0.0.1:0",
		Target:         "https://127.0.0.1:1",
		CertPath:       b.ClientCert,
		KeyPath:        b.ClientKey,
		CAPath:         b.CACert,
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fw.Running || fw.ID == "" || fw.Mode != "http" {
		t.Fatalf("bad add result: %+v", fw)
	}
	if fw.Cert == nil || fw.Cert.Subject == "" {
		t.Fatalf("missing cert: %+v", fw)
	}

	started, err := ctl.StartForward(fw.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !started.Running || started.ActualListen == "" {
		t.Fatalf("bad start result: %+v", started)
	}
	if err := checkHealth(started.ActualListen); err != nil {
		t.Fatal(err)
	}

	if _, err := ctl.StartForward(fw.ID); err == nil {
		t.Fatal("second start should fail while running")
	}

	stopped, err := ctl.StopForward(fw.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Running {
		t.Fatalf("still running: %+v", stopped)
	}
	if err := checkHealth(started.ActualListen); err == nil {
		t.Fatal("listener should be closed after stop")
	}

	if err := ctl.DeleteForward(fw.ID); err != nil {
		t.Fatal(err)
	}
	if list := ctl.Status().Forwards; len(list) != 0 {
		t.Fatalf("forward not deleted: %+v", list)
	}
}

func TestControlMultipleForwardsParallel(t *testing.T) {
	b := mustBundle(t)
	ctl := NewControl()
	defer ctl.Close()

	add := func(name string) ForwardStatus {
		t.Helper()
		fw, err := ctl.Add(ForwardRequest{
			Name:           name,
			Mode:           "http",
			Listen:         "127.0.0.1:0",
			Target:         "https://127.0.0.1:1",
			CertPath:       b.ClientCert,
			KeyPath:        b.ClientKey,
			CAPath:         b.CACert,
			TimeoutSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return fw
	}

	first := add("第一个")
	second := add("第二个")

	first, err := ctl.StartForward(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err = ctl.StartForward(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ActualListen == second.ActualListen {
		t.Fatalf("listeners should differ: %s", first.ActualListen)
	}
	if err := checkHealth(first.ActualListen); err != nil {
		t.Fatal(err)
	}
	if err := checkHealth(second.ActualListen); err != nil {
		t.Fatal(err)
	}

	list := ctl.Status().Forwards
	if len(list) != 2 {
		t.Fatalf("want 2 forwards, got %d", len(list))
	}
	running := 0
	for _, fw := range list {
		if fw.Running {
			running++
		}
	}
	if running != 2 {
		t.Fatalf("want 2 running, got %d", running)
	}

	if err := ctl.DeleteForward(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := checkHealth(first.ActualListen); err == nil {
		t.Fatal("deleted forward listener should be closed")
	}
	if err := checkHealth(second.ActualListen); err != nil {
		t.Fatalf("second forward should keep running: %v", err)
	}
}

func TestControlDuplicateListen(t *testing.T) {
	b := mustBundle(t)
	ctl := NewControl()
	defer ctl.Close()

	addr := freeAddr(t)
	req := ForwardRequest{
		Mode:           "http",
		Listen:         addr,
		Target:         "https://127.0.0.1:1",
		CertPath:       b.ClientCert,
		KeyPath:        b.ClientKey,
		CAPath:         b.CACert,
		TimeoutSeconds: 1,
	}
	if _, err := ctl.Add(req); err != nil {
		t.Fatal(err)
	}
	if _, err := ctl.Add(req); err == nil {
		t.Fatal("duplicate listen should be rejected")
	}
}

func TestControlInMemoryCert(t *testing.T) {
	b := mustBundle(t)
	certPEM, err := os.ReadFile(b.ClientCert)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := os.ReadFile(b.ClientKey)
	if err != nil {
		t.Fatal(err)
	}
	caPEM, err := os.ReadFile(b.CACert)
	if err != nil {
		t.Fatal(err)
	}

	ctl := NewControl()
	defer ctl.Close()
	fw, err := ctl.Add(ForwardRequest{
		Mode:           "tcp",
		Listen:         "127.0.0.1:0",
		Target:         "https://example.com:8443",
		CertPEM:        string(certPEM),
		KeyPEM:         string(keyPEM),
		CAPEM:          string(caPEM),
		ALPN:           "h2",
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	fw, err = ctl.StartForward(fw.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !fw.Running || fw.Mode != "tcp" {
		t.Fatalf("bad status: %+v", fw)
	}
	if fw.Cert == nil || fw.Cert.Subject == "" {
		t.Fatalf("missing in-memory cert: %+v", fw)
	}
}

func TestControlAPI(t *testing.T) {
	b := mustBundle(t)
	ctl := NewControl()
	srv := httptest.NewServer(ctl.Handler())
	defer srv.Close()
	defer ctl.Close()

	start := ForwardRequest{
		Name:           "API 转发",
		Mode:           "http",
		Listen:         "127.0.0.1:0",
		Target:         "https://127.0.0.1:1",
		CertPath:       b.ClientCert,
		KeyPath:        b.ClientKey,
		CAPath:         b.CACert,
		TimeoutSeconds: 1,
	}
	first := apiAdd(t, srv.URL, start)
	if first.Running || first.ID == "" {
		t.Fatalf("bad add: %+v", first)
	}
	second := apiAdd(t, srv.URL, start)
	if second.ID == first.ID {
		t.Fatalf("ids should differ: %s", first.ID)
	}

	status := apiStatus(t, srv.URL)
	if len(status) != 2 {
		t.Fatalf("want 2 forwards, got %d", len(status))
	}

	first = apiAction(t, srv.URL+"/api/forwards/"+first.ID+"/start")
	if !first.Running {
		t.Fatalf("start failed: %+v", first)
	}
	first = apiAction(t, srv.URL+"/api/forwards/"+first.ID+"/stop")
	if first.Running {
		t.Fatalf("stop failed: %+v", first)
	}

	apiDelete(t, srv.URL+"/api/forwards/"+first.ID)
	if status := apiStatus(t, srv.URL); len(status) != 1 {
		t.Fatalf("want 1 forward after delete, got %d", len(status))
	}
}

func TestControlIndex(t *testing.T) {
	ctl := NewControl()
	srv := httptest.NewServer(ctl.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body.Bytes(), []byte("mtlsproxy 控制台")) {
		t.Fatalf("unexpected index: %s", body.String())
	}
}

func TestControlRejectsNonJSON(t *testing.T) {
	ctl := NewControl()
	srv := httptest.NewServer(ctl.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/forwards", "text/plain", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestControlAddInvalidTarget(t *testing.T) {
	ctl := NewControl()
	if _, err := ctl.Add(ForwardRequest{Listen: "127.0.0.1:0", Target: "http://example.com"}); err == nil {
		t.Fatal("http target should be rejected")
	}
}

type apiResponse struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error"`
	Forward  ForwardStatus   `json:"forward"`
	Forwards []ForwardStatus `json:"forwards"`
}

func apiAdd(t *testing.T, base string, req ForwardRequest) ForwardStatus {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(base+"/api/forwards", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("add failed: %s", out.Error)
	}
	return out.Forward
}

func apiAction(t *testing.T, url string) ForwardStatus {
	t.Helper()
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("action failed: %s", out.Error)
	}
	return out.Forward
}

func apiDelete(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("delete failed: %s", out.Error)
	}
}

func apiStatus(t *testing.T, base string) []ForwardStatus {
	t.Helper()
	resp, err := http.Get(base + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatalf("status failed: %s", out.Error)
	}
	return out.Forwards
}

func checkHealth(addr string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/__mtls/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health status = %d", resp.StatusCode)
	}
	return nil
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}
