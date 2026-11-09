package tableview

import "testing"

func TestMatchRow(t *testing.T) {
	row := []string{"Alice", "北京", "42"}
	cases := []struct {
		query string
		want  bool
	}{
		{"", true},
		{"alice", true},
		{"ALI", true},
		{"北京", true},
		{"42", true},
		{"bob", false},
		{"  alice  ", true},
	}
	for _, c := range cases {
		if got := MatchRow(row, c.query); got != c.want {
			t.Errorf("MatchRow(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}
