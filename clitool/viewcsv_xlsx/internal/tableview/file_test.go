package tableview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLoadCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.csv")
	if err := os.WriteFile(path, []byte("n,v\n一,1\nb,2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path, ",", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "a.csv" || len(f.Sheets) != 1 {
		t.Fatalf("file = %+v", f)
	}
	if got := f.Sheets[0].Rows[0][0]; got != "n" {
		t.Fatalf("header = %q", got)
	}
	if got := f.Sheets[0].Rows[1][0]; got != "一" {
		t.Fatalf("cell = %q", got)
	}
}

func TestLoadTSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tsv")
	if err := os.WriteFile(path, []byte("a\tb\n1\t2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path, "\t", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Sheets[0].Rows[0][1] != "b" {
		t.Fatalf("rows = %#v", f.Sheets[0].Rows)
	}
}

func TestLoadXLSXSheet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.xlsx")
	xf := excelize.NewFile()
	if err := xf.SetCellValue("Sheet1", "A1", "x"); err != nil {
		t.Fatal(err)
	}
	idx, err := xf.NewSheet("用户")
	if err != nil {
		t.Fatal(err)
	}
	xf.SetActiveSheet(idx)
	if err := xf.SetCellValue("用户", "A1", "name"); err != nil {
		t.Fatal(err)
	}
	if err := xf.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	all, err := Load(path, ",", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Sheets) != 2 {
		t.Fatalf("sheets = %d", len(all.Sheets))
	}

	one, err := Load(path, ",", "用户")
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Sheets) != 1 || one.Sheets[0].Name != "用户" {
		t.Fatalf("sheet = %#v", one.Sheets)
	}

	byIdx, err := Load(path, ",", "2")
	if err != nil {
		t.Fatal(err)
	}
	if byIdx.Sheets[0].Name != "用户" {
		t.Fatalf("idx sheet = %q", byIdx.Sheets[0].Name)
	}
}

func TestSelectSheetsError(t *testing.T) {
	if _, err := SelectSheets([]string{"a"}, "nope"); err == nil {
		t.Fatal("want missing sheet error")
	}
	if _, err := SelectSheets([]string{"a"}, "9"); err == nil {
		t.Fatal("want range error")
	}
}

func TestExpandGlob(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.csv", "a.csv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Expand([]string{filepath.Join(dir, "*.csv")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if !strings.HasSuffix(got[0], "a.csv") || !strings.HasSuffix(got[1], "b.csv") {
		t.Fatalf("order = %#v", got)
	}
}

func TestColWidths(t *testing.T) {
	w := ColWidths([][]string{{"一", "ab"}, {"x", "yyyy"}}, 0)
	if w[0] != 2 || w[1] != 4 {
		t.Fatalf("widths = %#v", w)
	}
	w = ColWidths([][]string{{"yyyy"}}, 2)
	if w[0] != 2 {
		t.Fatalf("capped = %#v", w)
	}
}

func TestPadCell(t *testing.T) {
	if got := PadCell("一", 4); CellWidth(got) != 4 {
		t.Fatalf("pad = %q width=%d", got, CellWidth(got))
	}
}

func TestSanitize(t *testing.T) {
	got := Sanitize("a\x00b\nc\td")
	if strings.ContainsRune(got, 0) || strings.Contains(got, "\n") {
		t.Fatalf("got %q", got)
	}
	if CellWidth(got) != CellWidth("ab c    d") {
		t.Fatalf("width of %q", got)
	}
}
