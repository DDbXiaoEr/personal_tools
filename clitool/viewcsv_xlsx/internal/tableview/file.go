package tableview

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Sheet struct {
	Name string
	Rows [][]string
}

type File struct {
	Path   string
	Name   string
	Sheets []Sheet
}

func Expand(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	for _, pattern := range patterns {
		if hasGlob(pattern) {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("没有匹配 %q 的文件", pattern)
			}
			sort.Strings(matches)
			for _, m := range matches {
				if !seen[m] {
					seen[m] = true
					files = append(files, m)
				}
			}
			continue
		}
		if !seen[pattern] {
			seen[pattern] = true
			files = append(files, pattern)
		}
	}
	return files, nil
}

func hasGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func isXLSX(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return true
	default:
		return false
	}
}

func Load(path, delimiter, sheet string) (*File, error) {
	if isXLSX(path) {
		return loadXLSX(path, sheet)
	}
	return loadCSV(path, delimiter)
}

func loadCSV(path, delimiter string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	if len(delimiter) > 0 {
		reader.Comma = []rune(delimiter)[0]
	}
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	sanitizeRows(rows)
	return &File{
		Path:   path,
		Name:   filepath.Base(path),
		Sheets: []Sheet{{Name: filepath.Base(path), Rows: rows}},
	}, nil
}

func loadXLSX(path, sheet string) (*File, error) {
	xf, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer xf.Close()

	names := xf.GetSheetList()
	if len(names) == 0 {
		return &File{Path: path, Name: filepath.Base(path)}, nil
	}
	selected, err := SelectSheets(names, sheet)
	if err != nil {
		return nil, err
	}
	out := &File{Path: path, Name: filepath.Base(path)}
	for _, name := range selected {
		rows, err := xf.GetRows(name)
		if err != nil {
			return nil, err
		}
		sanitizeRows(rows)
		out.Sheets = append(out.Sheets, Sheet{Name: name, Rows: rows})
	}
	return out, nil
}

func sanitizeRows(rows [][]string) {
	for i := range rows {
		for j := range rows[i] {
			rows[i][j] = Sanitize(rows[i][j])
		}
	}
}

func SelectSheets(sheets []string, sheet string) ([]string, error) {
	if sheet == "" {
		return sheets, nil
	}
	for _, name := range sheets {
		if name == sheet {
			return []string{name}, nil
		}
	}
	if idx, err := strconv.Atoi(sheet); err == nil {
		if idx < 1 || idx > len(sheets) {
			return nil, fmt.Errorf("工作表序号 %d 超出范围 (1-%d)", idx, len(sheets))
		}
		return []string{sheets[idx-1]}, nil
	}
	return nil, fmt.Errorf("找不到工作表 %q", sheet)
}
