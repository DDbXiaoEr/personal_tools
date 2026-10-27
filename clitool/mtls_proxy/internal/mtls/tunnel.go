package mtls

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type closeWriter interface {
	CloseWrite() error
}

func ServeTCP(ctx context.Context, addr string, cfg Config) error {
	addr, err := normalizeListen(addr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听失败: %w", err)
	}
	return ServeTCPListener(ctx, ln, cfg)
}

func ServeTCPListener(ctx context.Context, ln net.Listener, cfg Config) error {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	hostport, sni, _, err := parseTarget(cfg.Target)
	if err != nil {
		ln.Close()
		return err
	}
	if cfg.ServerName != "" {
		sni = cfg.ServerName
	}
	tlsCfg, _, err := buildTLSConfig(cfg, sni, time.Now())
	if err != nil {
		ln.Close()
		return err
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	log.Printf("TCP 隧道 %s -> %s（明文接入，工具代持客户端证书）", ln.Addr().String(), hostport)

	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go handleTCP(c, hostport, tlsCfg, cfg.Timeout)
	}
}

func handleTCP(local net.Conn, hostport string, tlsCfg *tls.Config, timeout time.Duration) {
	defer local.Close()
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	d := &tls.Dialer{NetDialer: &net.Dialer{}, Config: tlsCfg}
	remote, err := d.DialContext(dialCtx, "tcp", hostport)
	if err != nil {
		log.Printf("%s 隧道握手失败: %v", local.RemoteAddr(), err)
		return
	}
	defer remote.Close()
	log.Printf("%s -> %s 已接通", local.RemoteAddr(), hostport)
	pipe(local, remote)
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyClose(a, b)
	}()
	go func() {
		defer wg.Done()
		copyClose(b, a)
	}()
	wg.Wait()
}

func copyClose(dst, src net.Conn) {
	_, _ = io.Copy(dst, src)
	if cw, ok := dst.(closeWriter); ok {
		_ = cw.CloseWrite()
	}
}
