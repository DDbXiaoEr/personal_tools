package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sftpTransferResult struct {
	Action     string `json:"action"`
	LocalPath  string `json:"local_path"`
	RemotePath string `json:"remote_path"`
	Bytes      int64  `json:"bytes"`
}

func newSFTPClient(sshClient *ssh.Client) (*sftp.Client, error) {
	c, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("failed to start SFTP: %w", err)
	}
	return c, nil
}

func expandLocalPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		if p == "~" {
			return home, nil
		}
		p = filepath.Join(home, p[2:])
	}
	return filepath.Abs(p)
}

func remoteJoin(dir, name string) string {
	if dir == "" || dir == "." {
		return path.Clean("/" + name)
	}
	return path.Join(dir, name)
}

func looksLikeRemoteDir(p string) bool {
	return strings.HasSuffix(p, "/")
}

func ensureLocalParent(localPath string) error {
	dir := filepath.Dir(localPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func ensureRemoteParent(client *sftp.Client, remotePath string) error {
	dir := path.Dir(remotePath)
	if dir == "" || dir == "." || dir == "/" {
		return nil
	}
	return client.MkdirAll(dir)
}

func resolveUploadDest(client *sftp.Client, remotePath, localPath string) (string, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return "", fmt.Errorf("remote_path is empty")
	}
	base := filepath.Base(localPath)
	if looksLikeRemoteDir(remotePath) {
		return remoteJoin(strings.TrimRight(remotePath, "/"), base), nil
	}
	info, err := client.Stat(remotePath)
	if err == nil && info.IsDir() {
		return remoteJoin(remotePath, base), nil
	}
	return remotePath, nil
}

func resolveDownloadDest(localPath, remotePath string) (string, error) {
	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		return filepath.Join(localPath, path.Base(remotePath)), nil
	}
	if err == nil {
		return localPath, nil
	}
	if os.IsNotExist(err) && (strings.HasSuffix(localPath, string(os.PathSeparator)) || strings.HasSuffix(localPath, "/")) {
		return filepath.Join(localPath, path.Base(remotePath)), nil
	}
	return localPath, nil
}

func uploadFile(client *sftp.Client, localPath, remotePath string, overwrite, createDirs bool) (sftpTransferResult, error) {
	localPath, err := expandLocalPath(localPath)
	if err != nil {
		return sftpTransferResult{}, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("local file: %w", err)
	}
	if info.IsDir() {
		return sftpTransferResult{}, fmt.Errorf("local_path is a directory, expected a file")
	}

	dest, err := resolveUploadDest(client, remotePath, localPath)
	if err != nil {
		return sftpTransferResult{}, err
	}

	if !overwrite {
		if _, err := client.Stat(dest); err == nil {
			return sftpTransferResult{}, fmt.Errorf("remote file already exists: %s", dest)
		}
	}

	if createDirs {
		if err := ensureRemoteParent(client, dest); err != nil {
			return sftpTransferResult{}, fmt.Errorf("failed to create remote directories: %w", err)
		}
	}

	src, err := os.Open(localPath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to open local file: %w", err)
	}
	defer src.Close()

	dst, err := client.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to create remote file %s: %w", dest, err)
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		_ = client.Remove(dest)
		return sftpTransferResult{}, err
	}

	_ = client.Chmod(dest, info.Mode().Perm())

	return sftpTransferResult{
		Action:     "upload",
		LocalPath:  localPath,
		RemotePath: dest,
		Bytes:      n,
	}, nil
}

func downloadFile(client *sftp.Client, localPath, remotePath string, overwrite, createDirs bool) (sftpTransferResult, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return sftpTransferResult{}, fmt.Errorf("remote_path is empty")
	}

	info, err := client.Stat(remotePath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("remote file: %w", err)
	}
	if info.IsDir() {
		return sftpTransferResult{}, fmt.Errorf("remote_path is a directory, expected a file")
	}

	localPath, err = expandLocalPath(localPath)
	if err != nil {
		return sftpTransferResult{}, err
	}
	dest, err := resolveDownloadDest(localPath, remotePath)
	if err != nil {
		return sftpTransferResult{}, err
	}

	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return sftpTransferResult{}, fmt.Errorf("local file already exists: %s", dest)
		}
	}

	if createDirs {
		if err := ensureLocalParent(dest); err != nil {
			return sftpTransferResult{}, fmt.Errorf("failed to create local directories: %w", err)
		}
	}

	src, err := client.Open(remotePath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to create local file: %w", err)
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		_ = os.Remove(dest)
		return sftpTransferResult{}, err
	}

	return sftpTransferResult{
		Action:     "download",
		LocalPath:  dest,
		RemotePath: remotePath,
		Bytes:      n,
	}, nil
}

func marshalResult(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}
