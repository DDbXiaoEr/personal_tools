package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type sshExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

var sshConfigFile = &sshConfig{}

func main() {
	configPath := flag.String("config", "", "Path to SSH config file (default: ~/.ssh/config)")
	flag.Parse()

	cfg, err := loadSSHConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load ssh config: %v", err)
	}
	sshConfigFile = cfg

	s := server.NewMCPServer(
		"ssh-mcp",
		"2.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("runsshcommand_via_ssh",
			mcp.WithDescription("Execute a command on a remote host via SSH. hostname is an SSH config Host alias (or a real hostname)."),
			mcp.WithString("hostname",
				mcp.Required(),
				mcp.Description("SSH config Host alias, or a hostname / IP if not listed in ssh config"),
			),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("Command to execute on the remote host"),
			),
			mcp.WithString("user",
				mcp.Description("SSH username (overrides ssh config User)"),
			),
			mcp.WithNumber("port",
				mcp.Description("SSH port (overrides ssh config Port, default: 22)"),
			),
			mcp.WithString("password",
				mcp.Description("SSH password (optional; otherwise IdentityFile / default keys are used)"),
			),
			mcp.WithString("key_path",
				mcp.Description("Path to SSH private key file (PEM format); overrides ssh config IdentityFile"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Connection timeout in seconds (overrides ssh config ConnectTimeout, default: 30)"),
			),
		),
		sshExecHandler,
	)

	s.AddTool(
		mcp.NewTool("uploadfile_via_ssh",
			mcp.WithDescription("Upload a local file to a remote host over SFTP. hostname is an SSH config Host alias. Directories are not supported."),
			mcp.WithString("hostname",
				mcp.Required(),
				mcp.Description("SSH config Host alias, or a hostname / IP if not listed in ssh config"),
			),
			mcp.WithString("local_path",
				mcp.Required(),
				mcp.Description("Local file path to upload"),
			),
			mcp.WithString("remote_path",
				mcp.Required(),
				mcp.Description("Remote destination path. If it is a directory (exists or ends with /), the local filename is appended."),
			),
			mcp.WithString("user",
				mcp.Description("SSH username (overrides ssh config User)"),
			),
			mcp.WithNumber("port",
				mcp.Description("SSH port (overrides ssh config Port, default: 22)"),
			),
			mcp.WithString("password",
				mcp.Description("SSH password (optional; otherwise IdentityFile / default keys are used)"),
			),
			mcp.WithString("key_path",
				mcp.Description("Path to SSH private key file (PEM format); overrides ssh config IdentityFile"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Connection timeout in seconds (overrides ssh config ConnectTimeout, default: 30)"),
			),
			mcp.WithBoolean("overwrite",
				mcp.Description("Overwrite the remote file if it already exists (default: false)"),
			),
			mcp.WithBoolean("create_dirs",
				mcp.Description("Create missing parent directories on the remote host (default: false)"),
			),
		),
		sshUploadHandler,
	)

	s.AddTool(
		mcp.NewTool("downloadfile_via_ssh",
			mcp.WithDescription("Download a remote file to the local filesystem over SFTP. hostname is an SSH config Host alias. Directories are not supported."),
			mcp.WithString("hostname",
				mcp.Required(),
				mcp.Description("SSH config Host alias, or a hostname / IP if not listed in ssh config"),
			),
			mcp.WithString("remote_path",
				mcp.Required(),
				mcp.Description("Remote file path to download"),
			),
			mcp.WithString("local_path",
				mcp.Required(),
				mcp.Description("Local destination path. If it is a directory, the remote filename is appended."),
			),
			mcp.WithString("user",
				mcp.Description("SSH username (overrides ssh config User)"),
			),
			mcp.WithNumber("port",
				mcp.Description("SSH port (overrides ssh config Port, default: 22)"),
			),
			mcp.WithString("password",
				mcp.Description("SSH password (optional; otherwise IdentityFile / default keys are used)"),
			),
			mcp.WithString("key_path",
				mcp.Description("Path to SSH private key file (PEM format); overrides ssh config IdentityFile"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Connection timeout in seconds (overrides ssh config ConnectTimeout, default: 30)"),
			),
			mcp.WithBoolean("overwrite",
				mcp.Description("Overwrite the local file if it already exists (default: false)"),
			),
			mcp.WithBoolean("create_dirs",
				mcp.Description("Create missing parent directories locally (default: false)"),
			),
		),
		sshDownloadHandler,
	)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func sshExecHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	command, _ := args["command"].(string)
	if strings.TrimSpace(command) == "" {
		return mcp.NewToolResultError("hostname and command are required"), nil
	}
	if isHighRisk(command) {
		return mcp.NewToolResultError(highRiskReason(command)), nil
	}

	cfg, errMsg := parseSSHConn(args)
	if errMsg != "" {
		if cfg.alias == "" {
			return mcp.NewToolResultError("hostname and command are required"), nil
		}
		return mcp.NewToolResultError(errMsg), nil
	}

	client, err := dialSSH(cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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

func sshUploadHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return sshTransferHandler(request, true)
}

func sshDownloadHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return sshTransferHandler(request, false)
}

func sshTransferHandler(request mcp.CallToolRequest, upload bool) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	localPath, _ := args["local_path"].(string)
	remotePath, _ := args["remote_path"].(string)
	if strings.TrimSpace(localPath) == "" || strings.TrimSpace(remotePath) == "" {
		return mcp.NewToolResultError("hostname, local_path, and remote_path are required"), nil
	}

	cfg, errMsg := parseSSHConn(args)
	if errMsg != "" {
		if cfg.alias == "" {
			return mcp.NewToolResultError("hostname, local_path, and remote_path are required"), nil
		}
		return mcp.NewToolResultError(errMsg), nil
	}

	overwrite := request.GetBool("overwrite", false)
	createDirs := request.GetBool("create_dirs", false)

	client, err := dialSSH(cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer client.Close()

	sftpClient, err := newSFTPClient(client)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer sftpClient.Close()

	var result sftpTransferResult
	if upload {
		result, err = uploadFile(sftpClient, localPath, remotePath, overwrite, createDirs)
	} else {
		result, err = downloadFile(sftpClient, localPath, remotePath, overwrite, createDirs)
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(marshalResult(result)), nil
}
