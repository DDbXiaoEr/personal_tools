package main

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

func startTestSFTP(t *testing.T) *sftp.Client {
	t.Helper()
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	server, err := sftp.NewServer(struct {
		io.Reader
		io.WriteCloser
	}{sr, sw})
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()
	client, err := sftp.NewClientPipe(cr, cw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})
	return client
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUploadDownload(t *testing.T) {
	root := t.TempDir()
	remoteRoot := filepath.Join(root, "remote")
	localRoot := filepath.Join(root, "local")
	if err := os.MkdirAll(remoteRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	client := startTestSFTP(t)
	src := filepath.Join(localRoot, "hello.txt")
	writeFile(t, src, "hello sftp")
	remoteFile := filepath.ToSlash(filepath.Join(remoteRoot, "hello.txt"))

	up, err := uploadFile(client, src, remoteFile, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if up.Bytes != int64(len("hello sftp")) {
		t.Fatalf("bytes = %d", up.Bytes)
	}

	_, err = uploadFile(client, src, remoteFile, false, false)
	if err == nil {
		t.Fatal("expected overwrite protection")
	}

	up, err = uploadFile(client, src, remoteFile, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if up.RemotePath != remoteFile {
		t.Fatalf("remote path = %s", up.RemotePath)
	}

	dst := filepath.Join(localRoot, "out.txt")
	down, err := downloadFile(client, dst, remoteFile, false, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello sftp" {
		t.Fatalf("got %q", b)
	}
	if down.Bytes != int64(len(b)) {
		t.Fatalf("download bytes = %d", down.Bytes)
	}

	_, err = downloadFile(client, dst, remoteFile, false, false)
	if err == nil {
		t.Fatal("expected local overwrite protection")
	}
}

func TestUploadToDirectoryAndCreateDirs(t *testing.T) {
	root := t.TempDir()
	remoteRoot := filepath.Join(root, "remote")
	inbox := filepath.Join(remoteRoot, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	client := startTestSFTP(t)

	src := filepath.Join(root, "note.txt")
	writeFile(t, src, "note")

	up, err := uploadFile(client, src, filepath.ToSlash(inbox)+"/", false, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.ToSlash(filepath.Join(inbox, "note.txt"))
	if up.RemotePath != want {
		t.Fatalf("got %s want %s", up.RemotePath, want)
	}

	nested := filepath.ToSlash(filepath.Join(remoteRoot, "nested", "dir", "note.txt"))
	_, err = uploadFile(client, src, nested, false, false)
	if err == nil {
		t.Fatal("expected missing parent dir error")
	}

	up, err = uploadFile(client, src, nested, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if up.RemotePath != nested {
		t.Fatalf("got %s", up.RemotePath)
	}
}

func TestRejectDirectories(t *testing.T) {
	root := t.TempDir()
	remoteDir := filepath.Join(root, "remote", "dir")
	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	client := startTestSFTP(t)

	_, err := uploadFile(client, root, filepath.ToSlash(filepath.Join(root, "x")), false, false)
	if err == nil {
		t.Fatal("expected local dir rejection")
	}

	dst := filepath.Join(root, "out")
	_, err = downloadFile(client, dst, filepath.ToSlash(remoteDir), false, true)
	if err == nil {
		t.Fatal("expected remote dir rejection")
	}
}

func TestDownloadIntoDirectory(t *testing.T) {
	root := t.TempDir()
	remoteRoot := filepath.Join(root, "remote")
	localDir := filepath.Join(root, "local")
	if err := os.MkdirAll(remoteRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	client := startTestSFTP(t)

	src := filepath.Join(root, "a.txt")
	writeFile(t, src, "abc")
	remoteFile := filepath.ToSlash(filepath.Join(remoteRoot, "a.txt"))
	if _, err := uploadFile(client, src, remoteFile, false, false); err != nil {
		t.Fatal(err)
	}

	down, err := downloadFile(client, localDir, remoteFile, false, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(localDir, "a.txt")
	if down.LocalPath != want {
		t.Fatalf("got %s want %s", down.LocalPath, want)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "abc" {
		t.Fatalf("got %q", b)
	}
}

func TestRemoteJoin(t *testing.T) {
	if got := remoteJoin("/tmp/inbox", "a.txt"); got != "/tmp/inbox/a.txt" {
		t.Fatalf("got %s", got)
	}
	if got := remoteJoin("", "a.txt"); got != "/a.txt" {
		t.Fatalf("got %s", got)
	}
	if !looksLikeRemoteDir("/tmp/inbox/") {
		t.Fatal("expected trailing slash to look like a dir")
	}
	if path.Base("/tmp/a.txt") != "a.txt" {
		t.Fatal("path.Base")
	}
}
