package main

import (
	"fmt"
	"strings"
)

var highRiskCommands = map[string]bool{
	"rm": true, "rmdir": true, "unlink": true,
	"dd": true, "mkfs": true, "mke2fs": true, "mkfs.ext4": true, "mkfs.xfs": true, "mkfs.btrfs": true,
	"shred": true, "truncate": true,
	"shutdown": true, "reboot": true, "halt": true, "poweroff": true, "init": true,
	"kill": true, "killall": true, "pkill": true, "pkillall": true,
	"sudo": true, "su": true, "doas": true,
	"passwd": true, "chpasswd": true, "usermod": true, "useradd": true, "userdel": true,
	"groupadd": true, "groupdel": true, "groupmod": true,
	"iptables": true, "nftables": true, "ufw": true, "firewall-cmd": true,
	"nc": true, "ncat": true, "socat": true,
	"chmod": true, "chown": true, "chattr": true, "setfacl": true,
	"mount": true, "umount": true, "swapoff": true, "swapon": true,
	"crontab": true, "at": true,
	"systemctl": true, "service": true,
	"visudo": true,
	"wget":   true, "curl": true, "ftp": true, "scp": true, "rsync": true,
	"eval": true, "exec": true, "source": true,
	"nohup": true, "screen": true, "tmux": true,
}

var dangerousShellPatterns = []string{
	"`",
	"$(",
	"${",
}

func isHighRisk(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)

	for _, pattern := range dangerousShellPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	firstWord := commandFirstWord(lower)
	return highRiskCommands[firstWord]
}

func highRiskReason(command string) string {
	trimmed := strings.TrimSpace(command)
	lower := strings.ToLower(trimmed)

	for _, pattern := range dangerousShellPatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Sprintf("command contains dangerous shell %q", pattern)
		}
	}

	firstWord := commandFirstWord(lower)
	if highRiskCommands[firstWord] {
		return fmt.Sprintf("command %q is blocked for security reasons", firstWord)
	}

	return "command blocked"
}

func commandFirstWord(lower string) string {
	spaceIdx := strings.IndexByte(lower, ' ')
	firstWord := lower
	if spaceIdx != -1 {
		firstWord = lower[:spaceIdx]
	}

	slashIdx := strings.LastIndexByte(firstWord, '/')
	if slashIdx != -1 {
		firstWord = firstWord[slashIdx+1:]
	}
	return firstWord
}
