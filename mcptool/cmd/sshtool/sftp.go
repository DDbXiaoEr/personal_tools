package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
	Files      int    `json:"files,omitempty"`
	Skipped    int    `json:"skipped,omitempty"`
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

func uploadPath(client *sftp.Client, localPath, remotePath string, overwrite, createDirs bool) (sftpTransferResult, error) {
	localPath, err := expandLocalPath(localPath)
	if err != nil {
		return sftpTransferResult{}, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("local path: %w", err)
	}
	if info.IsDir() {
		return uploadDir(client, localPath, remotePath, overwrite)
	}
	return uploadFile(client, localPath, remotePath, overwrite, createDirs)
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

	n, err := writeRemoteFile(client, localPath, dest, info.Mode())
	if err != nil {
		return sftpTransferResult{}, err
	}

	return sftpTransferResult{
		Action:     "upload",
		LocalPath:  localPath,
		RemotePath: dest,
		Bytes:      n,
	}, nil
}

func writeRemoteFile(client *sftp.Client, localPath, dest string, mode os.FileMode) (int64, error) {
	src, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open local file: %w", err)
	}
	defer src.Close()

	dst, err := client.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return 0, fmt.Errorf("failed to create remote file %s: %w", dest, err)
	}

	n, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = client.Remove(dest)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = client.Remove(dest)
		return 0, closeErr
	}

	_ = client.Chmod(dest, mode.Perm())
	return n, nil
}

type sftpPathEntry struct {
	rel  string
	mode os.FileMode
}

func uploadDir(client *sftp.Client, localDir, remotePath string, overwrite bool) (sftpTransferResult, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return sftpTransferResult{}, fmt.Errorf("remote_path is empty")
	}

	base := filepath.Base(filepath.Clean(localDir))
	dest := remotePath
	if looksLikeRemoteDir(remotePath) {
		dest = remoteJoin(strings.TrimRight(remotePath, "/"), base)
	} else if info, err := client.Stat(remotePath); err == nil {
		if !info.IsDir() {
			return sftpTransferResult{}, fmt.Errorf("remote_path is a file, expected a directory: %s", remotePath)
		}
		dest = remoteJoin(remotePath, base)
	}

	var dirs, files []sftpPathEntry
	skipped := 0
	err := filepath.WalkDir(localDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == localDir {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localDir, p)
		if err != nil {
			return err
		}
		entry := sftpPathEntry{rel: filepath.ToSlash(rel), mode: info.Mode()}
		switch {
		case d.Type()&os.ModeSymlink != 0:
			skipped++
		case d.IsDir():
			dirs = append(dirs, entry)
		case info.Mode().IsRegular():
			files = append(files, entry)
		default:
			skipped++
		}
		return nil
	})
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to read local directory: %w", err)
	}

	if !overwrite {
		for _, f := range files {
			target := remoteJoin(dest, f.rel)
			if _, err := client.Stat(target); err == nil {
				return sftpTransferResult{}, fmt.Errorf("remote file already exists: %s", target)
			}
		}
	}

	if err := client.MkdirAll(dest); err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to create remote directory %s: %w", dest, err)
	}
	for _, d := range dirs {
		if err := client.MkdirAll(remoteJoin(dest, d.rel)); err != nil {
			return sftpTransferResult{}, fmt.Errorf("failed to create remote directory %s: %w", remoteJoin(dest, d.rel), err)
		}
	}

	var total int64
	for _, f := range files {
		n, err := writeRemoteFile(client, filepath.Join(localDir, filepath.FromSlash(f.rel)), remoteJoin(dest, f.rel), f.mode)
		if err != nil {
			return sftpTransferResult{}, err
		}
		total += n
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		_ = client.Chmod(remoteJoin(dest, dirs[i].rel), dirs[i].mode.Perm())
	}
	if info, err := os.Stat(localDir); err == nil {
		_ = client.Chmod(dest, info.Mode().Perm())
	}

	return sftpTransferResult{
		Action:     "upload",
		LocalPath:  localDir,
		RemotePath: dest,
		Bytes:      total,
		Files:      len(files),
		Skipped:    skipped,
	}, nil
}

func downloadPath(client *sftp.Client, localPath, remotePath string, overwrite, createDirs bool) (sftpTransferResult, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return sftpTransferResult{}, fmt.Errorf("remote_path is empty")
	}

	info, err := client.Stat(remotePath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("remote path: %w", err)
	}
	if info.IsDir() {
		return downloadDir(client, localPath, remotePath, overwrite)
	}
	return downloadFile(client, localPath, remotePath, overwrite, createDirs)
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

	n, err := readRemoteFile(client, remotePath, dest, info.Mode())
	if err != nil {
		return sftpTransferResult{}, err
	}

	return sftpTransferResult{
		Action:     "download",
		LocalPath:  dest,
		RemotePath: remotePath,
		Bytes:      n,
	}, nil
}

func readRemoteFile(client *sftp.Client, remotePath, dest string, mode os.FileMode) (int64, error) {
	src, err := client.Open(remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return 0, fmt.Errorf("failed to create local file: %w", err)
	}

	n, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return 0, closeErr
	}
	return n, nil
}

func walkRemoteDir(client *sftp.Client, root string, visit func(rel string, info os.FileInfo)) error {
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		entries, err := client.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childRel := entry.Name()
			if rel != "" {
				childRel = path.Join(rel, entry.Name())
			}
			visit(childRel, entry)
			if entry.IsDir() && entry.Mode()&os.ModeSymlink == 0 {
				if err := walk(path.Join(dir, entry.Name()), childRel); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(root, "")
}

func downloadDir(client *sftp.Client, localPath, remotePath string, overwrite bool) (sftpTransferResult, error) {
	info, err := client.Stat(remotePath)
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("remote directory: %w", err)
	}
	if !info.IsDir() {
		return sftpTransferResult{}, fmt.Errorf("remote_path is not a directory: %s", remotePath)
	}

	localPath, err = expandLocalPath(localPath)
	if err != nil {
		return sftpTransferResult{}, err
	}
	dest := localPath
	if st, err := os.Stat(localPath); err == nil {
		if !st.IsDir() {
			return sftpTransferResult{}, fmt.Errorf("local_path is a file, expected a directory: %s", localPath)
		}
		dest = filepath.Join(localPath, path.Base(remotePath))
	} else if !os.IsNotExist(err) {
		return sftpTransferResult{}, fmt.Errorf("local path: %w", err)
	} else if strings.HasSuffix(localPath, string(os.PathSeparator)) || strings.HasSuffix(localPath, "/") {
		dest = filepath.Join(localPath, path.Base(remotePath))
	}

	var dirs, files []sftpPathEntry
	skipped := 0
	err = walkRemoteDir(client, remotePath, func(rel string, fi os.FileInfo) {
		entry := sftpPathEntry{rel: rel, mode: fi.Mode()}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			skipped++
		case fi.IsDir():
			dirs = append(dirs, entry)
		case fi.Mode().IsRegular():
			files = append(files, entry)
		default:
			skipped++
		}
	})
	if err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to read remote directory: %w", err)
	}

	if !overwrite {
		for _, f := range files {
			target := filepath.Join(dest, filepath.FromSlash(f.rel))
			if _, err := os.Stat(target); err == nil {
				return sftpTransferResult{}, fmt.Errorf("local file already exists: %s", target)
			}
		}
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return sftpTransferResult{}, fmt.Errorf("failed to create local directory %s: %w", dest, err)
	}
	for _, d := range dirs {
		target := filepath.Join(dest, filepath.FromSlash(d.rel))
		if err := os.MkdirAll(target, 0o755); err != nil {
			return sftpTransferResult{}, fmt.Errorf("failed to create local directory %s: %w", target, err)
		}
	}

	var total int64
	for _, f := range files {
		n, err := readRemoteFile(client, remoteJoin(remotePath, f.rel), filepath.Join(dest, filepath.FromSlash(f.rel)), f.mode)
		if err != nil {
			return sftpTransferResult{}, err
		}
		total += n
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Chmod(filepath.Join(dest, filepath.FromSlash(dirs[i].rel)), dirs[i].mode.Perm())
	}
	_ = os.Chmod(dest, info.Mode().Perm())

	return sftpTransferResult{
		Action:     "download",
		LocalPath:  dest,
		RemotePath: remotePath,
		Bytes:      total,
		Files:      len(files),
		Skipped:    skipped,
	}, nil
}

func marshalResult(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}
