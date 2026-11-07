package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// soundTimeout 是单次播放的兜底超时，避免异常音频文件卡住进程。
const soundTimeout = 15 * time.Second

var (
	soundGOOS = runtime.GOOS

	// 以下变量在测试中替换，避免依赖真实平台与外部命令。
	soundLookup = exec.LookPath
	soundExists = func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	soundRun = func(ctx context.Context, name string, args ...string) error {
		return exec.CommandContext(ctx, name, args...).Run()
	}

	// playNotificationSound 由 handler 调用，测试中可替换为同步记录器。
	playNotificationSound = func(enabled bool, file string) {
		if !enabled {
			return
		}
		go func() {
			if err := playSound(enabled, file); err != nil {
				log.Printf("play notification sound failed: %v", err)
			}
		}()
	}
)

// defaultSoundFile 返回当前平台自带的提示音，没有可用文件时返回空串。
func defaultSoundFile(goos string) string {
	switch goos {
	case "darwin":
		return "/System/Library/Sounds/Glass.aiff"
	case "linux":
		for _, path := range []string{
			"/usr/share/sounds/freedesktop/stereo/complete.oga",
			"/usr/share/sounds/freedesktop/stereo/bell.oga",
		} {
			if soundExists(path) {
				return path
			}
		}
	}
	return ""
}

// soundCommand 返回播放 file 所需的命令；平台不支持时返回空命令名。
func soundCommand(goos, file string) (string, []string) {
	switch goos {
	case "darwin":
		return "afplay", []string{file}
	case "linux":
		if path, err := soundLookup("ffplay"); err == nil {
			return path, []string{"-nodisp", "-autoexit", "-loglevel", "quiet", file}
		}
		if path, err := soundLookup("paplay"); err == nil {
			return path, []string{file}
		}
	}
	return "", nil
}

// requestSound 解析本次调用是否播放提示音、播放哪个文件。
// 工具参数优先于启动参数，两者都没配置时由平台默认提示音兜底。
func requestSound(req mcp.CallToolRequest) (bool, string) {
	enabled, file := appConfig.sound, appConfig.soundFile

	args := req.GetArguments()
	if v, ok := args["sound"]; ok {
		if b, ok := parseBoolArg(v); ok {
			enabled = b
		}
	}
	if v, ok := args["sound_file"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			file = strings.TrimSpace(s)
		}
	}
	return enabled, file
}

func parseBoolArg(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	case float64:
		return t != 0, true
	}
	return false, false
}

// playSound 在本地播放提示音。播放失败只记录日志，不影响消息发送结果。
func playSound(enabled bool, file string) error {
	if !enabled {
		return nil
	}
	if strings.TrimSpace(file) == "" {
		file = defaultSoundFile(soundGOOS)
	}
	if file == "" {
		return fmt.Errorf("no default sound file found for %s; pass sound_file", soundGOOS)
	}
	if !soundExists(file) {
		return fmt.Errorf("sound file %q does not exist or is not a file", file)
	}

	name, args := soundCommand(soundGOOS, file)
	if name == "" {
		return fmt.Errorf("playing sound is not supported on %s", soundGOOS)
	}

	ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
	defer cancel()
	if err := soundRun(ctx, name, args...); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}
