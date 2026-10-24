package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshConnArgs struct {
	alias    string
	host     string
	user     string
	password string
	keyPaths []string
	port     int
	timeout  time.Duration
}

var lookupDefaultIdentities = defaultIdentityFiles

func parseSSHConn(requestArgs map[string]any) (sshConnArgs, string) {
	hostname, hostTypeErr := argString(requestArgs, "hostname")
	if hostname == "" {
		if fallback, _ := argString(requestArgs, "host"); fallback != "" {
			hostname = fallback
		}
	}
	user, userTypeErr := argString(requestArgs, "user")
	password, pwTypeErr := argString(requestArgs, "password")
	keyPath, keyTypeErr := argString(requestArgs, "key_path")
	port, portSet, portErr := argOptionalInt(requestArgs, "port")
	timeoutSec, timeoutSet, timeoutErr := argOptionalFloat(requestArgs, "timeout")

	cfg := sshConnArgs{
		alias:    hostname,
		host:     hostname,
		user:     user,
		password: password,
		port:     22,
		timeout:  30 * time.Second,
	}

	if hostname == "" {
		return cfg, "hostname is required"
	}
	if hostTypeErr != "" || userTypeErr != "" || pwTypeErr != "" || keyTypeErr != "" || portErr != "" || timeoutErr != "" {
		return cfg, fmt.Sprintf("invalid arguments: %s", firstNonEmpty(hostTypeErr, userTypeErr, pwTypeErr, keyTypeErr, portErr, timeoutErr))
	}

	resolved := sshConfigFile.resolve(hostname)
	cfg.host = resolved.HostName
	if user == "" {
		cfg.user = resolved.User
	}
	if cfg.user == "" {
		return cfg, "user is required (set User in ssh config or pass user)"
	}

	if portSet {
		if port <= 0 {
			port = 22
		}
		cfg.port = port
	} else {
		cfg.port = resolved.Port
	}

	if timeoutSet {
		if timeoutSec <= 0 {
			timeoutSec = 30
		}
		cfg.timeout = time.Duration(timeoutSec) * time.Second
	} else if resolved.ConnectTimeout > 0 {
		cfg.timeout = resolved.ConnectTimeout
	}

	paths, err := collectKeyPaths(resolved, keyPath)
	if err != nil {
		return cfg, err.Error()
	}
	cfg.keyPaths = paths

	if password == "" && len(cfg.keyPaths) == 0 {
		return cfg, "no identity file found; set IdentityFile in ssh config or pass password / key_path"
	}
	return cfg, ""
}

func collectKeyPaths(resolved resolvedHost, extra string) ([]string, error) {
	var raw []string
	if extra != "" {
		raw = []string{extra}
	} else {
		raw = append(raw, resolved.IdentityFiles...)
		if !resolved.IdentitiesOnly {
			raw = append(raw, lookupDefaultIdentities()...)
		}
	}

	seen := map[string]bool{}
	var out []string
	for i, p := range raw {
		expanded, err := expandLocalPath(p)
		if err != nil {
			return nil, err
		}
		if seen[expanded] {
			continue
		}
		seen[expanded] = true
		if _, err := os.Stat(expanded); err != nil {
			if extra != "" && i == 0 {
				return nil, fmt.Errorf("failed to read private key file %s: %w", expanded, err)
			}
			continue
		}
		out = append(out, expanded)
	}
	return out, nil
}

func dialSSH(cfg sshConnArgs) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            cfg.user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.timeout,
	}

	var auth []ssh.AuthMethod
	if len(cfg.keyPaths) > 0 {
		var signers []ssh.Signer
		var lastErr error
		for _, p := range cfg.keyPaths {
			keyContent, err := os.ReadFile(p)
			if err != nil {
				lastErr = fmt.Errorf("failed to read private key file %s: %w", p, err)
				continue
			}
			signer, err := parsePrivateKey(string(keyContent))
			if err != nil {
				lastErr = fmt.Errorf("failed to parse private key %s: %w", p, err)
				continue
			}
			signers = append(signers, signer)
		}
		if len(signers) > 0 {
			auth = append(auth, ssh.PublicKeys(signers...))
		} else if cfg.password == "" {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("no usable identity file for %s", cfg.alias)
		}
	}
	if cfg.password != "" {
		auth = append(auth, ssh.Password(cfg.password))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("no authentication method available for %s", cfg.alias)
	}
	config.Auth = auth

	addr := net.JoinHostPort(cfg.host, fmt.Sprintf("%d", cfg.port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	return client, nil
}

func parsePrivateKey(keyContent string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(keyContent))
}

func argString(args map[string]any, key string) (string, string) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", ""
	}
	switch val := v.(type) {
	case string:
		return val, ""
	case json.Number:
		return val.String(), ""
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), ""
	case int:
		return strconv.Itoa(val), ""
	case bool:
		return strconv.FormatBool(val), ""
	default:
		return "", fmt.Sprintf("argument %q must be a string, got %T", key, v)
	}
}

func argOptionalInt(args map[string]any, key string) (int, bool, string) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false, ""
	}
	switch val := v.(type) {
	case int:
		return val, true, ""
	case int64:
		return int(val), true, ""
	case float64:
		return int(val), true, ""
	case json.Number:
		n, err := val.Int64()
		if err != nil {
			return 0, true, fmt.Sprintf("argument %q must be a number", key)
		}
		return int(n), true, ""
	case string:
		if val == "" {
			return 0, false, ""
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return 0, true, fmt.Sprintf("argument %q must be a number", key)
		}
		return n, true, ""
	default:
		return 0, true, fmt.Sprintf("argument %q must be a number, got %T", key, v)
	}
}

func argOptionalFloat(args map[string]any, key string) (float64, bool, string) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false, ""
	}
	switch val := v.(type) {
	case float64:
		return val, true, ""
	case int:
		return float64(val), true, ""
	case int64:
		return float64(val), true, ""
	case json.Number:
		n, err := val.Float64()
		if err != nil {
			return 0, true, fmt.Sprintf("argument %q must be a number", key)
		}
		return n, true, ""
	case string:
		if val == "" {
			return 0, false, ""
		}
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, true, fmt.Sprintf("argument %q must be a number", key)
		}
		return n, true, ""
	default:
		return 0, true, fmt.Sprintf("argument %q must be a number, got %T", key, v)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
