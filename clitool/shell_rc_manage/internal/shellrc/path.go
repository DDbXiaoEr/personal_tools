package shellrc

import "strings"

func SplitPATH(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ":") {
		p = strings.TrimSpace(p)
		if p == "" || p == "$PATH" || p == "${PATH}" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func looksLikeAssembledPATH(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	for _, p := range strings.Split(v, ":") {
		p = strings.TrimSpace(p)
		if p == "$PATH" || p == "${PATH}" {
			return true
		}
	}
	return false
}

func IsPATHItem(it *Item) bool {
	if it == nil {
		return false
	}
	if it.Style == StylePath {
		return true
	}
	if it.Name != PathVarName {
		return false
	}
	return it.Style == StyleExport || it.Style == StyleAssign
}

func ConvertPATHItems(items []*Item) []*Item {
	if len(items) == 0 {
		return items
	}
	seen := map[string]bool{}
	out := make([]*Item, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.Style == StylePath {
			parts := SplitPATH(it.Value)
			if len(parts) == 0 {
				continue
			}
			for _, p := range parts {
				if seen[p] {
					continue
				}
				seen[p] = true
				out = append(out, &Item{
					Category: it.Category,
					Name:     PathVarName,
					Value:    p,
					Style:    StylePath,
				})
			}
			continue
		}
		if IsPATHItem(it) && looksLikeAssembledPATH(it.Value) {
			parts := SplitPATH(it.Value)
			if allPATHSeen(parts, seen) {
				continue
			}
			for _, p := range parts {
				if seen[p] {
					continue
				}
				seen[p] = true
				out = append(out, &Item{
					Category: it.Category,
					Name:     PathVarName,
					Value:    p,
					Style:    StylePath,
				})
			}
			continue
		}
		out = append(out, it)
	}
	return out
}

func allPATHSeen(parts []string, seen map[string]bool) bool {
	if len(parts) == 0 {
		return true
	}
	for _, p := range parts {
		if !seen[p] {
			return false
		}
	}
	return true
}

func PATHEntries(items []*Item) []string {
	var out []string
	seen := map[string]bool{}
	for _, cat := range CategoryOrder(items) {
		for _, it := range ItemsInCategory(items, cat) {
			if it == nil || it.Style != StylePath {
				continue
			}
			for _, p := range SplitPATH(it.Value) {
				if seen[p] {
					continue
				}
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

func AssemblePATH(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ":") + ":$PATH"
}
