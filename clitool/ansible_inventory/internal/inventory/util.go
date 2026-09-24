package inventory

import (
	"fmt"
	"path/filepath"
	"strings"
)

func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func formatFromPath(path, content string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yml", ".yaml":
		return FormatYAML
	case ".ini":
		return FormatINI
	}
	s := strings.TrimSpace(content)
	if s == "" {
		return FormatINI
	}
	if strings.HasPrefix(s, "---") {
		return FormatYAML
	}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			return FormatINI
		}
		if strings.HasSuffix(t, ":") || strings.HasPrefix(t, "- ") {
			return FormatYAML
		}
		return FormatINI
	}
	return FormatINI
}
