package main

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
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

func TestUploadDirectoryRecursive(t *testing.T) {
	root := t.TempDir()
	localDir := filepath.Join(root, "project")
	writeFile(t, filepath.Join(localDir, "a.txt"), "a")
	writeFile(t, filepath.Join(localDir, "sub", "b.txt"), "bb")
	if err := os.MkdirAll(filepath.Join(localDir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	remoteRoot := filepath.Join(root, "remote")
	if err := os.MkdirAll(remoteRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	client := startTestSFTP(t)

	up, err := uploadPath(client, localDir, filepath.ToSlash(remoteRoot)+"/", false, false)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(remoteRoot, "project")
	if up.RemotePath != filepath.ToSlash(wantRoot) {
		t.Fatalf("got %s want %s", up.RemotePath, filepath.ToSlash(wantRoot))
	}
	if up.Files != 2 {
		t.Fatalf("files = %d", up.Files)
	}
	for name, want := range map[string]string{
		"a.txt":     "a",
		"sub/b.txt": "bb",
		"empty/":    "",
	} {
		p := filepath.Join(wantRoot, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if info, err := os.Stat(p); err != nil || !info.IsDir() {
				t.Fatalf("expected directory %s: %v", p, err)
			}
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Fatalf("%s = %q", name, b)
		}
	}

	if up.Bytes != 3 {
		t.Fatalf("bytes = %d", up.Bytes)
	}

	_, err = uploadPath(client, localDir, filepath.ToSlash(remoteRoot)+"/", false, false)
	if err == nil {
		t.Fatal("expected overwrite protection")
	}

	if _, err := uploadPath(client, localDir, filepath.ToSlash(remoteRoot)+"/", true, false); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadDirectoryRecursive(t *testing.T) {
	root := t.TempDir()
	remoteDir := filepath.Join(root, "remote", "project")
	writeFile(t, filepath.Join(remoteDir, "a.txt"), "a")
	writeFile(t, filepath.Join(remoteDir, "sub", "b.txt"), "bb")
	if err := os.MkdirAll(filepath.Join(remoteDir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	localDest := filepath.Join(root, "local")
	client := startTestSFTP(t)

	down, err := downloadPath(client, localDest, filepath.ToSlash(remoteDir), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if down.LocalPath != localDest {
		t.Fatalf("got %s want %s", down.LocalPath, localDest)
	}
	if down.Files != 2 || down.Bytes != 3 {
		t.Fatalf("files = %d bytes = %d", down.Files, down.Bytes)
	}
	for name, want := range map[string]string{
		"a.txt":     "a",
		"sub/b.txt": "bb",
		"empty/":    "",
	} {
		p := filepath.Join(localDest, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if info, err := os.Stat(p); err != nil || !info.IsDir() {
				t.Fatalf("expected directory %s: %v", p, err)
			}
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Fatalf("%s = %q", name, b)
		}
	}

	existing := filepath.Join(root, "existing")
	if err := os.MkdirAll(filepath.Join(existing, "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existing, "project", "a.txt"), "old")

	_, err = downloadPath(client, existing, filepath.ToSlash(remoteDir), false, false)
	if err == nil {
		t.Fatal("expected local overwrite protection")
	}

	down, err = downloadPath(client, existing, filepath.ToSlash(remoteDir), true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(existing, "project")
	if down.LocalPath != want {
		t.Fatalf("got %s want %s", down.LocalPath, want)
	}
}

func TestRejectDirectoriesInSingleFileHelpers(t *testing.T) {
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
