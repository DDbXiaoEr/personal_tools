package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type TestBundle struct {
	Dir        string
	CACert     string
	CAKey      string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
	OtherCA    string
	Expired    string
	ExpiredKey string
}

func GenerateTestBundle(dir string) (TestBundle, error) {
	now := time.Now().Add(-time.Hour)
	b := TestBundle{Dir: dir}

	caCert, caKey, caPEM, caKeyPEM, err := makeCA("test-ca", now, now.Add(24*time.Hour))
	if err != nil {
		return b, err
	}
	_, _, otherPEM, _, err := makeCA("other-ca", now, now.Add(24*time.Hour))
	if err != nil {
		return b, err
	}
	srvCert, srvKey, err := makeLeaf(caCert, caKey, "server", []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, now, now.Add(24*time.Hour), x509.ExtKeyUsageServerAuth)
	if err != nil {
		return b, err
	}
	cliCert, cliKey, err := makeLeaf(caCert, caKey, "client", nil, nil, now, now.Add(24*time.Hour), x509.ExtKeyUsageClientAuth)
	if err != nil {
		return b, err
	}
	expCert, expKey, err := makeLeaf(caCert, caKey, "expired-client", nil, nil, now.Add(-48*time.Hour), now.Add(-24*time.Hour), x509.ExtKeyUsageClientAuth)
	if err != nil {
		return b, err
	}

	b.CACert = filepath.Join(dir, "ca.crt")
	b.CAKey = filepath.Join(dir, "ca.key")
	b.ServerCert = filepath.Join(dir, "server.crt")
	b.ServerKey = filepath.Join(dir, "server.key")
	b.ClientCert = filepath.Join(dir, "client.crt")
	b.ClientKey = filepath.Join(dir, "client.key")
	b.OtherCA = filepath.Join(dir, "other-ca.crt")
	b.Expired = filepath.Join(dir, "expired.crt")
	b.ExpiredKey = filepath.Join(dir, "expired.key")

	files := map[string][]byte{
		b.CACert:     caPEM,
		b.CAKey:      caKeyPEM,
		b.ServerCert: srvCert,
		b.ServerKey:  srvKey,
		b.ClientCert: cliCert,
		b.ClientKey:  cliKey,
		b.OtherCA:    otherPEM,
		b.Expired:    expCert,
		b.ExpiredKey: expKey,
	}
	for p, data := range files {
		if err := os.WriteFile(p, data, 0o600); err != nil {
			return b, err
		}
	}
	return b, nil
}

func StartTestServer(certFile, keyFile, caFile string, requireClient bool) (addr string, closeFn func(), err error) {
	cfg, err := serverTLS(certFile, keyFile, caFile, requireClient)
	if err != nil {
		return "", nil, err
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		return "", nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 1)
				_, _ = conn.Read(buf)
			}(c)
		}
	}()
	return ln.Addr().String(), func() { ln.Close(); <-done }, nil
}

func StartTestEcho(certFile, keyFile, caFile string, requireClient bool) (addr string, closeFn func(), err error) {
	cfg, err := serverTLS(certFile, keyFile, caFile, requireClient)
	if err != nil {
		return "", nil, err
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		return "", nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()
	return ln.Addr().String(), func() { ln.Close(); <-done }, nil
}

func StartTestHTTPS(certFile, keyFile, caFile string, requireClient bool, status int, body string) (addr string, closeFn func(), err error) {
	cfg, err := serverTLS(certFile, keyFile, caFile, requireClient)
	if err != nil {
		return "", nil, err
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	srv := &http.Server{Handler: mux}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		_ = srv.Close()
		<-done
	}, nil
}

func serverTLS(certFile, keyFile, caFile string, requireClient bool) (*tls.Config, error) {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS12,
	}
	if requireClient {
		pool, err := LoadCertPool(caFile)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

func makeCA(cn string, notBefore, notAfter time.Time) (*x509.Certificate, *ecdsa.PrivateKey, []byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"mtlsproxy-test"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return cert, key, certPEM, keyPEM, nil
}

func makeLeaf(ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, dns []string, ips []net.IP, notBefore, notAfter time.Time, eku x509.ExtKeyUsage) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"mtlsproxy-test"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{eku},
		DNSNames:              dns,
		IPAddresses:           ips,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}
