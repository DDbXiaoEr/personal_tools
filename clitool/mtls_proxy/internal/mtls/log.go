package mtls

import (
	"os"
	"path/filepath"
)

// DefaultLogPath 返回终端控制台的日志文件路径。
func DefaultLogPath() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "mtlsproxy", "mtlsproxy.log")
	}
	if dir, err := os.UserHomeDir(); err == nil && dir != "" {
		return filepath.Join(dir, ".mtlsproxy.log")
	}
	return ""
}
