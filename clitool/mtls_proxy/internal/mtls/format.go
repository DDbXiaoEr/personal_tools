package mtls

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func FormatProbe(res Result) string {
	var b strings.Builder
	if res.OK {
		fmt.Fprintf(&b, "结果: 成功\n")
	} else {
		fmt.Fprintf(&b, "结果: 失败\n")
		if res.ErrorClass != "" {
			fmt.Fprintf(&b, "类别: %s\n", res.ErrorClass)
		}
		if res.Error != "" {
			fmt.Fprintf(&b, "错误: %s\n", res.Error)
		}
	}
	fmt.Fprintf(&b, "目标: %s\n", res.Target)
	fmt.Fprintf(&b, "耗时: %dms\n", res.DurationMS)
	if res.TLSVersion != "" {
		fmt.Fprintf(&b, "TLS: %s  %s\n", res.TLSVersion, res.CipherSuite)
	}
	if res.ServerName != "" {
		fmt.Fprintf(&b, "SNI: %s\n", res.ServerName)
	}
	if res.ClientCert != nil {
		fmt.Fprintf(&b, "\n客户端证书:\n")
		writeCert(&b, *res.ClientCert)
	}
	if len(res.PeerCerts) > 0 {
		fmt.Fprintf(&b, "\n服务端证书:\n")
		writeCert(&b, res.PeerCerts[0])
		for i := 1; i < len(res.PeerCerts); i++ {
			fmt.Fprintf(&b, "\n证书链[%d]:\n", i)
			writeCert(&b, res.PeerCerts[i])
		}
	}
	if res.HTTPStatus > 0 {
		fmt.Fprintf(&b, "\nHTTP: %d %s\n", res.HTTPStatus, res.HTTPProto)
	}
	return b.String()
}

func FormatInspect(res InspectResult) string {
	var b strings.Builder
	if res.OK {
		fmt.Fprintf(&b, "结果: 成功\n")
	} else {
		fmt.Fprintf(&b, "结果: 失败\n")
		if res.ErrorClass != "" {
			fmt.Fprintf(&b, "类别: %s\n", res.ErrorClass)
		}
		if res.Error != "" {
			fmt.Fprintf(&b, "错误: %s\n", res.Error)
		}
	}
	fmt.Fprintf(&b, "私钥匹配: %s\n", boolCN(res.KeyOK))
	if res.ClientCert != nil || len(res.Chain) > 0 {
		fmt.Fprintf(&b, "CA 校验: %s\n", boolCN(res.CAVerified))
	}
	if res.ClientCert != nil {
		fmt.Fprintf(&b, "\n客户端证书:\n")
		writeCert(&b, *res.ClientCert)
	}
	for i := 1; i < len(res.Chain); i++ {
		fmt.Fprintf(&b, "\n证书链[%d]:\n", i)
		writeCert(&b, res.Chain[i])
	}
	return b.String()
}

func FormatJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func writeCert(b *strings.Builder, c CertInfo) {
	fmt.Fprintf(b, "  主体: %s\n", c.Subject)
	fmt.Fprintf(b, "  颁发者: %s\n", c.Issuer)
	fmt.Fprintf(b, "  序列号: %s\n", c.Serial)
	fmt.Fprintf(b, "  有效期: %s ~ %s\n", c.NotBefore.Format(time.RFC3339), c.NotAfter.Format(time.RFC3339))
	if c.Expired {
		fmt.Fprintf(b, "  状态: 已过期\n")
	} else if c.NotYetValid {
		fmt.Fprintf(b, "  状态: 尚未生效\n")
	}
	if len(c.DNSNames) > 0 {
		fmt.Fprintf(b, "  DNS: %s\n", strings.Join(c.DNSNames, ", "))
	}
	if len(c.IPAddrs) > 0 {
		fmt.Fprintf(b, "  IP: %s\n", strings.Join(c.IPAddrs, ", "))
	}
	if c.IsCA {
		fmt.Fprintf(b, "  CA: 是\n")
	}
}

func boolCN(v bool) string {
	if v {
		return "是"
	}
	return "否"
}
