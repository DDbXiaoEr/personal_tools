package tableview

import "strings"

func MatchRow(row []string, query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, cell := range row {
		if strings.Contains(strings.ToLower(cell), q) {
			return true
		}
	}
	return false
}
