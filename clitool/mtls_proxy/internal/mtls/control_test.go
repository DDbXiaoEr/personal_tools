package mtls

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestControlStartStopHTTP(t *testing.T) {
	b := mustBundle(t)
	ctl := NewControl()

	st, err := ctl.Start(StartRequest{
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
	if !st.Running || st.Mode != "http" || st.Listen == "" {
		t.Fatalf("bad status: %+v", st)
	}
	if st.Cert == nil || st.Cert.Subject == "" {
		t.Fatalf("missing cert: %+v", st)
	}

	resp, err := http.Get("http://" + st.Listen + "/__mtls/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}

	if _, err := ctl.Start(StartRequest{Listen: "127.0.0.1:0", Target: "https://127.0.0.1:1"}); err == nil {
		t.Fatal("second start should fail while running")
	}

	stopped := ctl.Stop()
	if stopped.Running {
		t.Fatalf("still running: %+v", stopped)
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
	st, err := ctl.Start(StartRequest{
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
	defer ctl.Stop()
	if !st.Running || st.Mode != "tcp" {
		t.Fatalf("bad status: %+v", st)
	}
	if st.Cert == nil || st.Cert.Subject == "" {
		t.Fatalf("missing in-memory cert: %+v", st)
	}
}

func TestControlAPI(t *testing.T) {
	b := mustBundle(t)
	ctl := NewControl()
	srv := httptest.NewServer(ctl.Handler())
	defer srv.Close()
	defer ctl.Stop()

	start := StartRequest{
		Mode:           "http",
		Listen:         "127.0.0.1:0",
		Target:         "https://127.0.0.1:1",
		CertPath:       b.ClientCert,
		KeyPath:        b.ClientKey,
		CAPath:         b.CACert,
		TimeoutSeconds: 1,
	}
	payload, err := json.Marshal(start)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(srv.URL+"/api/start", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var started struct {
		OK     bool          `json:"ok"`
		Error  string        `json:"error"`
		Status ControlStatus `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	if !started.OK || !started.Status.Running {
		t.Fatalf("start failed: %+v", started)
	}

	statusResp, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer statusResp.Body.Close()
	var status struct {
		OK     bool          `json:"ok"`
		Status ControlStatus `json:"status"`
	}
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.OK || !status.Status.Running {
		t.Fatalf("status: %+v", status)
	}

	stopResp, err := http.Post(srv.URL+"/api/stop", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stopResp.Body.Close()
	var stopped struct {
		OK     bool          `json:"ok"`
		Status ControlStatus `json:"status"`
	}
	if err := json.NewDecoder(stopResp.Body).Decode(&stopped); err != nil {
		t.Fatal(err)
	}
	if !stopped.OK || stopped.Status.Running {
		t.Fatalf("stop failed: %+v", stopped)
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

	resp, err := http.Post(srv.URL+"/api/start", "text/plain", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestControlStartInvalidTarget(t *testing.T) {
	ctl := NewControl()
	if _, err := ctl.Start(StartRequest{Listen: "127.0.0.1:0", Target: "http://example.com"}); err == nil {
		t.Fatal("http target should be rejected")
	}
}
