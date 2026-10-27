package mtls

import (
	"crypto/x509"
	"errors"
	"net"
	"strings"
)

func classify(err error) string {
	if err == nil {
		return ""
	}
	var ua x509.UnknownAuthorityError
	if errors.As(err, &ua) {
		return "unknown_authority"
	}
	var hn x509.HostnameError
	if errors.As(err, &hn) {
		return "hostname_mismatch"
	}
	var inv x509.CertificateInvalidError
	if errors.As(err, &inv) {
		switch inv.Reason {
		case x509.Expired:
			return "expired"
		case x509.NotAuthorizedToSign:
			return "not_authorized_to_sign"
		default:
			return "certificate_invalid"
		}
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	var op *net.OpError
	if errors.As(err, &op) {
		if op.Timeout() {
			return "timeout"
		}
		return "connection"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "certificate required"),
		strings.Contains(msg, "bad certificate"),
		strings.Contains(msg, "unknown certificate authority"),
		strings.Contains(msg, "tls: unknown certificate"):
		return "bad_client_certificate"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "timeout"
	case strings.Contains(msg, "tls:"):
		return "handshake"
	default:
		return "other"
	}
}
