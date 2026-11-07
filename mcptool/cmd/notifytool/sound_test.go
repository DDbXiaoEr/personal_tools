package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func saveSoundGlobals(t *testing.T) {
	t.Helper()
	oldGOOS, oldLookup, oldExists, oldRun, oldPlay := soundGOOS, soundLookup, soundExists, soundRun, playNotificationSound
	t.Cleanup(func() {
		soundGOOS, soundLookup, soundExists, soundRun, playNotificationSound = oldGOOS, oldLookup, oldExists, oldRun, oldPlay
	})
}

func TestRequestSound(t *testing.T) {
	oldConfig := appConfig
	defer func() { appConfig = oldConfig }()

	cases := []struct {
		name        string
		defaultOn   bool
		defaultFile string
		args        map[string]any
		wantOn      bool
		wantFile    string
	}{
		{
			name:     "default off",
			args:     map[string]any{"content": "hi"},
			wantOn:   false,
			wantFile: "",
		},
		{
			name:      "startup default on",
			defaultOn: true,
			args:      map[string]any{"content": "hi"},
			wantOn:    true,
		},
		{
			name:     "tool enables sound",
			args:     map[string]any{"content": "hi", "sound": true},
			wantOn:   true,
			wantFile: "",
		},
		{
			name:      "tool disables startup default",
			defaultOn: true,
			args:      map[string]any{"content": "hi", "sound": false},
			wantOn:    false,
		},
		{
			name:     "string bool accepted",
			args:     map[string]any{"content": "hi", "sound": "yes"},
			wantOn:   true,
			wantFile: "",
		},
		{
			name:      "tool file overrides startup file",
			defaultOn: true,
			args:      map[string]any{"sound_file": " /tmp/a.aiff "},
			wantOn:    true,
			wantFile:  "/tmp/a.aiff",
		},
		{
			name:        "blank tool file keeps startup file",
			defaultOn:   true,
			defaultFile: "/tmp/default.aiff",
			args:        map[string]any{"sound_file": "   "},
			wantOn:      true,
			wantFile:    "/tmp/default.aiff",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appConfig = config{sound: tc.defaultOn, soundFile: tc.defaultFile}
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tc.args
			gotOn, gotFile := requestSound(req)
			if gotOn != tc.wantOn {
				t.Fatalf("enabled = %v, want %v", gotOn, tc.wantOn)
			}
			if gotFile != tc.wantFile {
				t.Fatalf("file = %q, want %q", gotFile, tc.wantFile)
			}
		})
	}
}

func TestParseBoolArg(t *testing.T) {
	cases := []struct {
		in     any
		want   bool
		wantOK bool
	}{
		{true, true, true},
		{false, false, true},
		{" TRUE ", true, true},
		{"off", false, true},
		{float64(1), true, true},
		{float64(0), false, true},
		{"maybe", false, false},
		{123, false, false},
		{nil, false, false},
	}
	for _, tc := range cases {
		got, ok := parseBoolArg(tc.in)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("parseBoolArg(%v) = %v,%v want %v,%v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestDefaultSoundFile(t *testing.T) {
	saveSoundGlobals(t)
	soundExists = func(path string) bool { return path == "/usr/share/sounds/freedesktop/stereo/bell.oga" }

	if got := defaultSoundFile("darwin"); got != "/System/Library/Sounds/Glass.aiff" {
		t.Fatalf("darwin default = %q", got)
	}
	if got := defaultSoundFile("linux"); got != "/usr/share/sounds/freedesktop/stereo/bell.oga" {
		t.Fatalf("linux default = %q", got)
	}
	if got := defaultSoundFile("windows"); got != "" {
		t.Fatalf("windows default = %q", got)
	}
}

func TestSoundCommand(t *testing.T) {
	saveSoundGlobals(t)
	soundLookup = func(name string) (string, error) {
		if name == "ffplay" {
			return "/usr/bin/ffplay", nil
		}
		return "", context.Canceled
	}

	name, args := soundCommand("darwin", "/tmp/a.aiff")
	if name != "afplay" || len(args) != 1 || args[0] != "/tmp/a.aiff" {
		t.Fatalf("darwin = %q %v", name, args)
	}

	name, args = soundCommand("linux", "/tmp/a.oga")
	if name != "/usr/bin/ffplay" || len(args) != 5 || args[4] != "/tmp/a.oga" {
		t.Fatalf("linux = %q %v", name, args)
	}

	name, _ = soundCommand("windows", "/tmp/a.wav")
	if name != "" {
		t.Fatalf("windows = %q", name)
	}
}

func TestPlaySound(t *testing.T) {
	saveSoundGlobals(t)

	var gotName string
	var gotArgs []string
	soundGOOS = "darwin"
	soundExists = func(path string) bool { return path == "/tmp/a.aiff" }
	soundRun = func(_ context.Context, name string, args ...string) error {
		gotName, gotArgs = name, args
		return nil
	}

	if err := playSound(false, "/tmp/a.aiff"); err != nil {
		t.Fatalf("disabled playSound: %v", err)
	}
	if gotName != "" {
		t.Fatalf("disabled sound should not run command, got %q", gotName)
	}

	if err := playSound(true, "/tmp/a.aiff"); err != nil {
		t.Fatalf("playSound: %v", err)
	}
	if gotName != "afplay" || len(gotArgs) != 1 || gotArgs[0] != "/tmp/a.aiff" {
		t.Fatalf("command = %q %v", gotName, gotArgs)
	}

	if err := playSound(true, "/tmp/missing.aiff"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing file error = %v", err)
	}

	soundGOOS = "windows"
	if err := playSound(true, "/tmp/a.aiff"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unsupported platform error = %v", err)
	}
}

func TestHandlersTriggerSoundOnlyOnSuccess(t *testing.T) {
	saveSoundGlobals(t)

	oldConfig, oldSender := appConfig, defaultSender
	defer func() { appConfig, defaultSender = oldConfig, oldSender }()

	var played []string
	playNotificationSound = func(enabled bool, file string) {
		if enabled {
			played = append(played, file)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	appConfig = config{dingtalkKey: "tok"}
	defaultSender = newSender(2*time.Second, "", srv.URL, "")

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"content": "done", "sound": true}
	if _, err := dingtalkHandler(context.Background(), req); err != nil {
		t.Fatalf("dingtalkHandler: %v", err)
	}
	if len(played) != 1 {
		t.Fatalf("played = %v, want 1", played)
	}

	// 没有 sound 参数时沿用启动默认值（这里是关闭）。
	played = nil
	req.Params.Arguments = map[string]any{"content": "done"}
	if _, err := dingtalkHandler(context.Background(), req); err != nil {
		t.Fatalf("dingtalkHandler: %v", err)
	}
	if len(played) != 0 {
		t.Fatalf("played = %v, want none", played)
	}
}
