package opencode

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestModelsURL(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com/v1":         "https://api.example.com/v1/models",
		"https://api.example.com/v1/":        "https://api.example.com/v1/models",
		"https://api.example.com/v1/models":  "https://api.example.com/v1/models",
		"https://api.example.com/v1/models/": "https://api.example.com/v1/models",
		"http://localhost:1234":              "http://localhost:1234/models",
	}
	for in, want := range cases {
		got, err := ModelsURL(in)
		if err != nil {
			t.Errorf("ModelsURL(%q) err = %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ModelsURL(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ModelsURL(""); err == nil {
		t.Error("empty baseURL should fail")
	}
	if _, err := ModelsURL("not-a-url"); err == nil {
		t.Error("invalid baseURL should fail")
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("OC_TEST_KEY", "secret-from-env")
	got, err := ResolveAPIKey("{env:OC_TEST_KEY}")
	if err != nil || got != "secret-from-env" {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = ResolveAPIKey("plain-key")
	if err != nil || got != "plain-key" {
		t.Fatalf("plain = %q %v", got, err)
	}
	if _, err := ResolveAPIKey("{env:OC_TEST_MISSING}"); err == nil {
		t.Fatal("missing env should fail")
	}
}

func TestParseModelsResponse(t *testing.T) {
	openai := `{"object":"list","data":[{"id":"gpt-4o","object":"model"},{"id":"gpt-4o-mini","name":"GPT-4o mini"}]}`
	got, err := parseModelsResponse([]byte(openai))
	if err != nil {
		t.Fatal(err)
	}
	want := []RemoteModel{{ID: "gpt-4o", Name: "gpt-4o"}, {ID: "gpt-4o-mini", Name: "GPT-4o mini"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openai: got %+v want %+v", got, want)
	}

	named := `{"models":[{"id":"claude-3","display_name":"Claude 3"}]}`
	got, err = parseModelsResponse([]byte(named))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "claude-3" || got[0].Name != "Claude 3" {
		t.Fatalf("named: %+v", got)
	}

	arr := `[{"id":"a"},{"model":"b","name":"Bee"}]`
	got, err = parseModelsResponse([]byte(arr))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" || got[1].Name != "Bee" {
		t.Fatalf("arr: %+v", got)
	}

	ids := `{"data":["m1","m2"]}`
	got, err = parseModelsResponse([]byte(ids))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "m1" || got[1].ID != "m2" {
		t.Fatalf("ids: %+v", got)
	}

	obj := `{"data":{"x":{"name":"X"},"y":{}}}`
	got, err = parseModelsResponse([]byte(obj))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "x" || got[0].Name != "X" || got[1].ID != "y" {
		t.Fatalf("obj: %+v", got)
	}
}

func TestFetchModels(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		io.WriteString(w, `{"data":[{"id":"gpt-4o","name":"GPT-4o"},{"id":"gpt-4o"}]}`)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	models, err := fetchModels(client, srv.URL+"/v1", "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotPath != "/v1/models" {
		t.Errorf("path = %q", gotPath)
	}
	if len(models) != 1 || models[0].ID != "gpt-4o" || models[0].Name != "GPT-4o" {
		t.Fatalf("models = %+v", models)
	}
}

func TestFetchModelsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"nope"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	_, err := fetchModels(client, srv.URL, "k")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeRemoteModels(t *testing.T) {
	p := ParseProvider("local", json.RawMessage(`{"models":{"keep":{"name":"Keep","variants":{"fast":{}}}}}`))
	added, updated := p.MergeRemoteModels([]RemoteModel{
		{ID: "keep", Name: "Ignored"},
		{ID: "new", Name: "New Model"},
		{ID: "empty-name", Name: ""},
	})
	if added != 2 || updated != 0 {
		t.Fatalf("added=%d updated=%d", added, updated)
	}
	keep := p.FindModel("keep")
	if keep == nil || keep.Name != "Keep" || len(keep.Variants) != 1 {
		t.Fatalf("keep overwritten: %+v", keep)
	}
	if p.FindModel("new") == nil || p.FindModel("new").Name != "New Model" {
		t.Fatal("new missing")
	}
	if p.FindModel("empty-name").Name != "empty-name" {
		t.Fatal("empty name not filled")
	}

	p2 := ParseProvider("x", json.RawMessage(`{"models":{"a":{}}}`))
	added, updated = p2.MergeRemoteModels([]RemoteModel{{ID: "a", Name: "Alpha"}})
	if added != 0 || updated != 1 || p2.FindModel("a").Name != "Alpha" {
		t.Fatalf("fill name: added=%d updated=%d name=%q", added, updated, p2.FindModel("a").Name)
	}

	raw, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got := ParseProvider("local", raw)
	if got.FindModel("keep") == nil || got.FindModel("keep").Name != "Keep" || got.FindVariant("keep", "fast") == nil {
		t.Fatalf("keep lost after encode: %+v", got.Models)
	}
	if got.FindModel("new") == nil || got.FindModel("new").Name != "New Model" {
		t.Fatalf("new lost after encode: %+v", got.Models)
	}
}
