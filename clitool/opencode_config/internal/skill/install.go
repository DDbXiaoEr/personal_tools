package skill

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxZipBytes = 50 << 20

var httpClient = &http.Client{Timeout: 60 * time.Second}

func ValidName(name string) bool {
	return nameRe.MatchString(name)
}

func Remove(skillsDir, dirName string) error {
	if dirName == "" || dirName == "." || dirName == ".." || strings.Contains(dirName, string(os.PathSeparator)) {
		return fmt.Errorf("非法技能目录名")
	}
	dest := filepath.Join(skillsDir, dirName)
	if filepath.Dir(dest) != filepath.Clean(skillsDir) {
		return fmt.Errorf("非法技能目录名")
	}
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("技能目录不存在: %s", dirName)
		}
		return err
	}
	return os.RemoveAll(dest)
}

func Install(skillsDir, name, uri string) error {
	name = strings.TrimSpace(name)
	uri = strings.TrimSpace(uri)
	if name == "" {
		return fmt.Errorf("目录名不能为空")
	}
	if !ValidName(name) {
		return fmt.Errorf("目录名只能是小写字母、数字和连字符")
	}
	if uri == "" {
		return fmt.Errorf("URI 不能为空")
	}
	if err := os.MkdirAll(skillsDir, 0700); err != nil {
		return err
	}
	dest := filepath.Join(skillsDir, name)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("技能目录 %s 已存在", name)
	}

	tmp, err := os.MkdirTemp("", "occonfig-skill-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	zipPath, err := fetchZip(tmp, uri)
	if err != nil {
		return err
	}
	extractDir := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(extractDir, 0700); err != nil {
		return err
	}
	if err := unzip(zipPath, extractDir); err != nil {
		return err
	}
	root, err := findSkillRoot(extractDir)
	if err != nil {
		return err
	}
	if err := copyDir(root, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return nil
}

func fetchZip(tmp, uri string) (string, error) {
	if p, ok := localPath(uri); ok {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("读取 zip 失败: %w", err)
		}
		return p, nil
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("无效 URI: %s", uri)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("仅支持 http(s) 或本地 zip 路径")
	}
	resp, err := httpClient.Get(uri)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	out := filepath.Join(tmp, "skill.zip")
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxZipBytes+1))
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	if n > maxZipBytes {
		return "", fmt.Errorf("zip 超过 %dMB 限制", maxZipBytes>>20)
	}
	return out, nil
}

func localPath(uri string) (string, bool) {
	if strings.HasPrefix(uri, "file://") {
		u, err := url.Parse(uri)
		if err != nil {
			return "", false
		}
		return u.Path, true
	}
	if strings.Contains(uri, "://") {
		return "", false
	}
	return uri, true
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("不是有效的 zip: %w", err)
	}
	defer r.Close()
	dest = filepath.Clean(dest)
	prefix := dest + string(os.PathSeparator)
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "__MACOSX") {
			continue
		}
		path := filepath.Join(dest, name)
		if path != dest && !strings.HasPrefix(path, prefix) {
			return fmt.Errorf("非法 zip 路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func findSkillRoot(extractDir string) (string, error) {
	var found []string
	err := filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == "__MACOSX" || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.EqualFold(info.Name(), "SKILL.md") {
			found = append(found, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		return "", fmt.Errorf("zip 中未找到 SKILL.md")
	}
	best := found[0]
	bestDepth := strings.Count(best, string(os.PathSeparator))
	for _, p := range found[1:] {
		d := strings.Count(p, string(os.PathSeparator))
		if d < bestDepth {
			best, bestDepth = p, d
		}
	}
	return best, nil
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, in, info.Mode())
	})
}
