package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mtlsproxy/internal/mtls"
)

func main() {
	listen := flag.String("listen", "", "监听地址，如 127.0.0.1:8080；设置后进入代理模式")
	target := flag.String("target", "", "目标 mTLS 服务，如 https://api.example.com:8443")
	cert := flag.String("cert", "", "客户端证书 PEM")
	key := flag.String("key", "", "客户端私钥 PEM；省略则与 -cert 同一文件")
	ca := flag.String("ca", "", "校验服务端的 CA PEM")
	insecure := flag.Bool("insecure", false, "跳过服务端证书校验")
	sni := flag.String("sni", "", "覆盖 TLS ServerName")
	timeout := flag.Duration("timeout", 10*time.Second, "握手超时")
	method := flag.String("method", "", "额外发 HTTP 请求，如 GET / HEAD")
	path := flag.String("path", "", "HTTP 路径，默认 /")
	jsonOut := flag.Bool("json", false, "JSON 输出")
	inspect := flag.Bool("inspect", false, "只检查本地证书，不连接服务端")
	tcpMode := flag.Bool("tcp", false, "TCP 隧道（gRPC / 裸 TCP）；默认 HTTP 反向代理")
	alpn := flag.String("alpn", "", "TLS ALPN，逗号分隔；gRPC 用 h2")
	flag.Parse()

	if *inspect {
		if *cert == "" {
			usage("inspect 需要 -cert")
		}
		res := mtls.Inspect(*cert, *key, *ca)
		printOut(res, mtls.FormatInspect(res), *jsonOut)
		if !res.OK {
			os.Exit(1)
		}
		return
	}

	if *target == "" {
		usage("需要 -target")
	}
	cfg := mtls.Config{
		Target:     *target,
		CertFile:   *cert,
		KeyFile:    *key,
		CAFile:     *ca,
		Insecure:   *insecure,
		ServerName: *sni,
		Timeout:    *timeout,
		Method:     *method,
		Path:       *path,
		ALPN:       splitCSV(*alpn),
	}

	if *listen != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if *tcpMode {
			fmt.Fprintf(os.Stderr, "mtlsproxy TCP 隧道 %s -> %s（明文接入，工具代持客户端证书）\n", *listen, *target)
			if err := mtls.ServeTCP(ctx, *listen, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
				os.Exit(1)
			}
			return
		}
		h, err := mtls.NewHandler(cfg, *jsonOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "mtlsproxy 监听 %s，浏览器访问该地址即可打开 %s（工具代持客户端证书）\n", *listen, *target)
		if err := mtls.Serve(ctx, *listen, h); err != nil {
			fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	res := mtls.Probe(cfg)
	printOut(res, mtls.FormatProbe(res), *jsonOut)
	if !res.OK {
		os.Exit(1)
	}
}

func printOut(v any, text string, jsonOut bool) {
	if jsonOut {
		s, err := mtls.FormatJSON(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "编码 JSON 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(s)
		return
	}
	fmt.Print(text)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
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

func usage(msg string) {
	fmt.Fprintf(os.Stderr, "%s\n\n", msg)
	fmt.Fprintf(os.Stderr, `用法:
  mtlsproxy -target https://host:port -cert client.crt -key client.key -ca ca.crt
  mtlsproxy -listen 127.0.0.1:8000 -target https://host:port -cert client.crt -key client.key -ca ca.crt
  mtlsproxy -tcp -listen 127.0.0.1:8000 -target host:50051 -alpn h2 -cert client.crt -key client.key -ca ca.crt
  mtlsproxy -inspect -cert client.crt -key client.key -ca ca.crt

-listen 默认 HTTP 反向代理：浏览器访问监听地址即可打开目标页面。
-tcp 为字节流隧道，适用于 gRPC / 裸 TCP；客户端连明文端口，工具用证书做 mTLS。
HTTP 模式 /__mtls/check 只探测握手，/__mtls/health 返回 ok。
`)
	flag.PrintDefaults()
	os.Exit(2)
}
