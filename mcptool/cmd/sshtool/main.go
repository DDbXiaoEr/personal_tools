package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var highRiskCommands = map[string]bool{
	"rm": true, "rmdir": true, "unlink": true,
	"dd": true, "mkfs": true, "mke2fs": true, "mkfs.ext4": true, "mkfs.xfs": true, "mkfs.btrfs": true,
	"shred": true, "truncate": true,
	"shutdown": true, "reboot": true, "halt": true, "poweroff": true, "init": true,
	"kill": true, "killall": true, "pkill": true, "pkillall": true,
	"sudo": true, "su": true, "doas": true,
	"passwd": true, "chpasswd": true, "usermod": true, "useradd": true, "userdel": true,
	"groupadd": true, "groupdel": true, "groupmod": true,
	"iptables": true, "nftables": true, "ufw": true, "firewall-cmd": true,
	"nc": true, "ncat": true, "socat": true,
	"chmod": true, "chown": true, "chattr": true, "setfacl": true,
	"mount": true, "umount": true, "swapoff": true, "swapon": true,
	"crontab": true, "at": true,
	"systemctl": true, "service": true,
	"visudo": true,
	"wget": true, "curl": true, "ftp": true, "scp": true, "rsync": true,
	"eval": true, "exec": true, "source": true,
	"nohup": true, "screen": true, "tmux": true,
}

var dangerousShellPatterns = []string{
	"`",
	"$(",
	"${",
}

func isHighRisk(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)

	for _, pattern := range dangerousShellPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	spaceIdx := strings.IndexByte(lower, ' ')
	firstWord := lower
	if spaceIdx != -1 {
		firstWord = lower[:spaceIdx]
	}

	slashIdx := strings.LastIndexByte(firstWord, '/')
	if slashIdx != -1 {
		firstWord = firstWord[slashIdx+1:]
	}

	if highRiskCommands[firstWord] {
		return true
	}

	return false
}

func highRiskReason(command string) string {
	trimmed := strings.TrimSpace(command)
	lower := strings.ToLower(trimmed)

	for _, pattern := range dangerousShellPatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Sprintf("command contains dangerous shell %q", pattern)
		}
	}

	spaceIdx := strings.IndexByte(lower, ' ')
	firstWord := lower
	if spaceIdx != -1 {
		firstWord = lower[:spaceIdx]
	}

	slashIdx := strings.LastIndexByte(firstWord, '/')
	if slashIdx != -1 {
		firstWord = firstWord[slashIdx+1:]
	}

	if highRiskCommands[firstWord] {
		return fmt.Sprintf("command %q is blocked for security reasons", firstWord)
	}

	return "command blocked"
}

type sshExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func main() {
	s := server.NewMCPServer(
		"ssh-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("runsshcommand_via_ssh",
			mcp.WithDescription("Execute a command on a remote host via SSH and return stdout, stderr, and exit code."),
			mcp.WithString("host",
				mcp.Required(),
				mcp.Description("Remote host IP or hostname"),
			),
			mcp.WithString("user",
				mcp.Required(),
				mcp.Description("SSH username"),
			),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("Command to execute on the remote host"),
			),
			mcp.WithNumber("port",
				mcp.Description("SSH port (default: 22)"),
			),
			mcp.WithString("password",
				mcp.Description("SSH password"),
			),
			mcp.WithString("key_path",
				mcp.Description("Path to SSH private key file (PEM format)"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Connection timeout in seconds (default: 30)"),
			),
		),
		sshExecHandler,
	)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func sshExecHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	host, _ := args["host"].(string)
	user, _ := args["user"].(string)
	command, _ := args["command"].(string)

	port := request.GetInt("port", 22)
	timeout := time.Duration(request.GetFloat("timeout", 30)) * time.Second

	password, pwTypeErr := argString(args, "password")
	keyPath, keyTypeErr := argString(args, "key_path")

	if host == "" || user == "" || command == "" {
		return mcp.NewToolResultError("host, user, and command are required"), nil
	}

	if isHighRisk(command) {
		return mcp.NewToolResultError(highRiskReason(command)), nil
	}

	if password == "" && keyPath == "" {
		if pwTypeErr != "" || keyTypeErr != "" {
			return mcp.NewToolResultError(fmt.Sprintf("invalid credentials: %s %s", pwTypeErr, keyTypeErr)), nil
		}
		return mcp.NewToolResultError("either password or key_path must be provided"), nil
	}

	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	if keyPath != "" {
		keyContent, err := os.ReadFile(keyPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read private key file %s: %v", keyPath, err)), nil
		}
		signer, err := parsePrivateKey(string(keyContent))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse private key: %v", err)), nil
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		config.Auth = []ssh.AuthMethod{ssh.Password(password)}
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("SSH connection failed: %v", err)), nil
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create session: %v", err)), nil
	}
	defer session.Close()

	var stdout, stderr strings.Builder
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)

	result := sshExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			return mcp.NewToolResultError(fmt.Sprintf("command execution failed: %v", err)), nil
		}
	}

	jsonBytes, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonBytes)), nil
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
