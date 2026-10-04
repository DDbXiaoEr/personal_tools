package main

import (
	"context"
	"encoding/json"
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

func main() {
	s := server.NewMCPServer(
		"ssh-mcp",
		"1.1.0",
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

	s.AddTool(
		mcp.NewTool("uploadfile_via_ssh",
			mcp.WithDescription("Upload a local file to a remote host over SFTP. Directories are not supported."),
			mcp.WithString("host",
				mcp.Required(),
				mcp.Description("Remote host IP or hostname"),
			),
			mcp.WithString("user",
				mcp.Required(),
				mcp.Description("SSH username"),
			),
			mcp.WithString("local_path",
				mcp.Required(),
				mcp.Description("Local file path to upload"),
			),
			mcp.WithString("remote_path",
				mcp.Required(),
				mcp.Description("Remote destination path. If it is a directory (exists or ends with /), the local filename is appended."),
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
			mcp.WithDescription("Download a remote file to the local filesystem over SFTP. Directories are not supported."),
			mcp.WithString("host",
				mcp.Required(),
				mcp.Description("Remote host IP or hostname"),
			),
			mcp.WithString("user",
				mcp.Required(),
				mcp.Description("SSH username"),
			),
			mcp.WithString("remote_path",
				mcp.Required(),
				mcp.Description("Remote file path to download"),
			),
			mcp.WithString("local_path",
				mcp.Required(),
				mcp.Description("Local destination path. If it is a directory, the remote filename is appended."),
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
		return mcp.NewToolResultError("host, user, and command are required"), nil
	}
	if isHighRisk(command) {
		return mcp.NewToolResultError(highRiskReason(command)), nil
	}

	cfg, errMsg := parseSSHConn(args, request.GetInt("port", 22), request.GetFloat("timeout", 30))
	if errMsg != "" {
		if cfg.host == "" || cfg.user == "" {
			return mcp.NewToolResultError("host, user, and command are required"), nil
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
		return mcp.NewToolResultError("host, user, local_path, and remote_path are required"), nil
	}

	cfg, errMsg := parseSSHConn(args, request.GetInt("port", 22), request.GetFloat("timeout", 30))
	if errMsg != "" {
		if cfg.host == "" || cfg.user == "" {
			return mcp.NewToolResultError("host, user, local_path, and remote_path are required"), nil
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
