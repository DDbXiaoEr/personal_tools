package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const keyPairFileName = "key-pair.json"

type keyPairEntry struct {
	KeyName string `json:"keyname"`
	KeyPath string `json:"keypath"`
}

var keyPairIndex = map[string]string{}

func loadKeyPairs(path string) (map[string]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return map[string]string{}, nil
	}

	resolved, err := expandLocalPath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("key-pair path: %w", err)
	}
	if info.IsDir() {
		resolved = filepath.Join(resolved, keyPairFileName)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", resolved, err)
	}

	entries, err := parseKeyPairJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", resolved, err)
	}

	index := make(map[string]string, len(entries))
	for _, e := range entries {
		name := strings.TrimSpace(e.KeyName)
		keyPath := strings.TrimSpace(e.KeyPath)
		if name == "" || keyPath == "" {
			return nil, fmt.Errorf("%s: keyname and keypath are required", resolved)
		}
		if _, exists := index[name]; exists {
			return nil, fmt.Errorf("%s: duplicate keyname %q", resolved, name)
		}
		index[name] = keyPath
	}
	return index, nil
}

func parseKeyPairJSON(data []byte) ([]keyPairEntry, error) {
	var entries []keyPairEntry
	if err := json.Unmarshal(data, &entries); err == nil {
		return entries, nil
	}

	var one keyPairEntry
	if err := json.Unmarshal(data, &one); err != nil {
		return nil, fmt.Errorf("invalid JSON, expected {\"keyname\":\"...\",\"keypath\":\"...\"} or an array of those objects")
	}
	return []keyPairEntry{one}, nil
}

func lookupKeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("key_name is empty")
	}
	if len(keyPairIndex) == 0 {
		return "", fmt.Errorf("key_name %q cannot be resolved: key-pair.json not loaded (pass -key-pair)", name)
	}
	p, ok := keyPairIndex[name]
	if !ok {
		return "", fmt.Errorf("unknown key_name %q", name)
	}
	return p, nil
}
