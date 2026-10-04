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
	host     string
	user     string
	password string
	keyPath  string
	port     int
	timeout  time.Duration
}

func parseSSHConn(requestArgs map[string]any, port int, timeoutSec float64) (sshConnArgs, string) {
	host, _ := requestArgs["host"].(string)
	user, _ := requestArgs["user"].(string)
	password, pwTypeErr := argString(requestArgs, "password")
	keyPath, keyTypeErr := argString(requestArgs, "key_path")

	if port <= 0 {
		port = 22
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	cfg := sshConnArgs{
		host:     host,
		user:     user,
		password: password,
		keyPath:  keyPath,
		port:     port,
		timeout:  time.Duration(timeoutSec) * time.Second,
	}

	if host == "" || user == "" {
		return cfg, "host and user are required"
	}
	if password == "" && keyPath == "" {
		if pwTypeErr != "" || keyTypeErr != "" {
			return cfg, fmt.Sprintf("invalid credentials: %s %s", pwTypeErr, keyTypeErr)
		}
		return cfg, "either password or key_path must be provided"
	}
	return cfg, ""
}

func dialSSH(cfg sshConnArgs) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            cfg.user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.timeout,
	}

	if cfg.keyPath != "" {
		keyContent, err := os.ReadFile(cfg.keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %s: %w", cfg.keyPath, err)
		}
		signer, err := parsePrivateKey(string(keyContent))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		config.Auth = []ssh.AuthMethod{ssh.Password(cfg.password)}
	}

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
