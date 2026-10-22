package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseKeyPairJSON(t *testing.T) {
	t.Run("object", func(t *testing.T) {
		entries, err := parseKeyPairJSON([]byte(`{"keyname":"demookey","keypath":"~/.ssh/demokey"}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].KeyName != "demookey" || entries[0].KeyPath != "~/.ssh/demokey" {
			t.Fatalf("got %+v", entries)
		}
	})
	t.Run("array", func(t *testing.T) {
		entries, err := parseKeyPairJSON([]byte(`[
			{"keyname":"a","keypath":"/tmp/a"},
			{"keyname":"b","keypath":"/tmp/b"}
		]`))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("len = %d", len(entries))
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, err := parseKeyPairJSON([]byte(`not json`)); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestLoadKeyPairs(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		got, err := loadKeyPairs("")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("file path", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "key-pair.json")
		if err := os.WriteFile(p, []byte(`{"keyname":"demookey","keypath":"~/.ssh/demokey"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := loadKeyPairs(p)
		if err != nil {
			t.Fatal(err)
		}
		if got["demookey"] != "~/.ssh/demokey" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("directory path", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "key-pair.json")
		if err := os.WriteFile(p, []byte(`[{"keyname":"a","keypath":"/tmp/a"}]`), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := loadKeyPairs(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got["a"] != "/tmp/a" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("duplicate keyname", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "key-pair.json")
		if err := os.WriteFile(p, []byte(`[{"keyname":"a","keypath":"/tmp/a"},{"keyname":"a","keypath":"/tmp/b"}]`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadKeyPairs(p); err == nil {
			t.Fatal("expected duplicate error")
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "key-pair.json")
		if err := os.WriteFile(p, []byte(`{"keyname":"a"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadKeyPairs(p); err == nil {
			t.Fatal("expected missing field error")
		}
	})
}

func TestLookupKeyName(t *testing.T) {
	prev := keyPairIndex
	t.Cleanup(func() { keyPairIndex = prev })

	keyPairIndex = map[string]string{}
	if _, err := lookupKeyName("demookey"); err == nil {
		t.Fatal("expected not loaded error")
	}

	keyPairIndex = map[string]string{"demookey": "~/.ssh/demokey"}
	got, err := lookupKeyName("demookey")
	if err != nil {
		t.Fatal(err)
	}
	if got != "~/.ssh/demokey" {
		t.Fatalf("got %q", got)
	}
	if _, err := lookupKeyName("missing"); err == nil {
		t.Fatal("expected unknown key error")
	}
}
