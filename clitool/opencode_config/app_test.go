package main

import (
	"reflect"
	"testing"
)

func TestApplyPickedFileKeepsArgs(t *testing.T) {
	f := &form{}
	f.add(newArea("command", "", 3))
	f.fields[0].setArea("/old/bin\n--arg1\n--arg2")

	m := &model{form: *f, pickTarget: 0}
	m.applyPickedFile("/new/bin")

	got := parseLines(m.form.fields[0].areaText())
	want := []string{"/new/bin", "--arg1", "--arg2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestApplyPickedFileEmpty(t *testing.T) {
	f := &form{}
	f.add(newArea("command", "", 3))

	m := &model{form: *f, pickTarget: 0}
	m.applyPickedFile("/usr/local/bin/mcp")

	got := parseLines(m.form.fields[0].areaText())
	want := []string{"/usr/local/bin/mcp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseKV(t *testing.T) {
	kvs := parseKV("A=1\nB = two\n# comment\nC", "=")
	if len(kvs) != 3 {
		t.Fatalf("got %v", kvs)
	}
	if kvs[1].Key != "B" || kvs[1].Value != "two" {
		t.Errorf("kv[1] = %+v", kvs[1])
	}
	if kvs[2].Key != "C" || kvs[2].Value != "" {
		t.Errorf("kv[2] = %+v", kvs[2])
	}
}
