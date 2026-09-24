package inventory

import (
	"errors"
	"os"
	"path/filepath"
)

func Load(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			inv := &Inventory{Path: path, Format: formatFromPath(path, "")}
			return inv, nil
		}
		return nil, err
	}
	inv, err := Parse(string(data), formatFromPath(path, string(data)))
	if err != nil {
		return nil, err
	}
	inv.Path = path
	return inv, nil
}

func Parse(src string, format Format) (*Inventory, error) {
	if format == FormatYAML {
		return ParseYAML(src)
	}
	return ParseINI(src), nil
}

func (inv *Inventory) Encode() (string, error) {
	if inv.Format == FormatYAML {
		return inv.EncodeYAML()
	}
	return inv.EncodeINI(), nil
}

func (inv *Inventory) Save() error {
	if inv == nil || inv.Path == "" {
		return os.ErrInvalid
	}
	out, err := inv.Encode()
	if err != nil {
		return err
	}
	dir := filepath.Dir(inv.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if prev, err := os.ReadFile(inv.Path); err == nil && len(prev) > 0 {
		_ = os.WriteFile(inv.Path+".bak", prev, 0644)
	}
	tmp := inv.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, inv.Path); err != nil {
		return err
	}
	inv.Dirty = false
	return nil
}

func (inv *Inventory) Reload() error {
	next, err := Load(inv.Path)
	if err != nil {
		return err
	}
	*inv = *next
	return nil
}
