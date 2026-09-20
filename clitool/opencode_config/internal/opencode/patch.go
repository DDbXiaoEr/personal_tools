package opencode

import (
	"bytes"
	"encoding/json"

	"github.com/tailscale/hujson"
)

func bytesEqual(a, b []byte) bool {
	return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
}

func rawMapEqual(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !bytesEqual(av, bv) {
			return false
		}
	}
	return true
}

func memberName(m hujson.ObjectMember) string {
	if lit, ok := m.Name.Value.(hujson.Literal); ok {
		return lit.String()
	}
	return ""
}

func findMember(obj *hujson.Object, name string) *hujson.ObjectMember {
	for i := range obj.Members {
		if memberName(obj.Members[i]) == name {
			return &obj.Members[i]
		}
	}
	return nil
}

func removeMember(obj *hujson.Object, name string) {
	for i := range obj.Members {
		if memberName(obj.Members[i]) == name {
			obj.Members = append(obj.Members[:i], obj.Members[i+1:]...)
			return
		}
	}
}
