package mtls

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func Serve(ctx context.Context, addr string, h http.Handler) error {
	addr, err := normalizeListen(addr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听失败: %w", err)
	}
	return ServeListener(ctx, ln, h)
}

func ServeListener(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	log.Printf("监听 %s ，明文转发到目标（浏览器不用装证书）", ln.Addr().String())
	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func normalizeListen(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", fmt.Errorf("未指定监听地址")
	}
	if !strings.Contains(addr, ":") {
		return "127.0.0.1:" + addr, nil
	}
	return addr, nil
}
