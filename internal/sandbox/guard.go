// Package sandbox provides a secure container-based execution environment.
package sandbox

import (
	"regexp"
	"strings"
)

// blockedCommandPatterns contains regex patterns for dangerous commands.
// These patterns are checked before any command execution.
var blockedCommandPatterns = []*regexp.Regexp{
	// Destructive recursive deletion
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*)?-[a-z]*r[a-z]*\s+(-[a-z]*\s+)*(/|~|\$HOME|\.\.|\*)\s*`),
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*\s+)*--no-preserve-root`),
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*)?-[a-z]*r[a-z]*\s+(-[a-z]*\s+)*/\*`),
	regexp.MustCompile(`(?i)\brm\s+-rf\b`),
	regexp.MustCompile(`(?i)\brm\s+-r\b`),

	// Windows destructive commands
	regexp.MustCompile(`(?i)\bdel\s+/[a-z]*f`),
	regexp.MustCompile(`(?i)\bdel\s+/[a-z]*s`),
	regexp.MustCompile(`(?i)\brd\s+/[a-z]*s`),
	regexp.MustCompile(`(?i)\brmdir\s+/[a-z]*s`),

	// Disk formatting and partition manipulation
	regexp.MustCompile(`(?i)\bformat\b`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`(?i)\bdiskpart\b`),
	regexp.MustCompile(`(?i)\bfdisk\b`),
	regexp.MustCompile(`(?i)\bparted\b`),
	regexp.MustCompile(`(?i)\bgdisk\b`),

	// Direct disk writes
	regexp.MustCompile(`(?i)\bdd\s+.*\bif\s*=`),
	regexp.MustCompile(`(?i)\bdd\s+.*\bof\s*=\s*/dev/(sd[a-z]|hd[a-z]|nvme|vd[a-z]|xvd[a-z]|loop)`),
	regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),
	regexp.MustCompile(`(?i)>\s*/dev/hd[a-z]`),
	regexp.MustCompile(`(?i)>\s*/dev/nvme`),
	regexp.MustCompile(`(?i)>\s*/dev/vd[a-z]`),
	regexp.MustCompile(`(?i)>\s*/dev/xvd[a-z]`),

	// System shutdown/reboot
	regexp.MustCompile(`(?i)\bshutdown\b`),
	regexp.MustCompile(`(?i)\breboot\b`),
	regexp.MustCompile(`(?i)\bpoweroff\b`),
	regexp.MustCompile(`(?i)\bhalt\b`),
	regexp.MustCompile(`(?i)\binit\s+[06]\b`),
	regexp.MustCompile(`(?i)\bsystemctl\s+(halt|poweroff|reboot|shutdown)`),

	// Fork bombs and resource exhaustion
	regexp.MustCompile(`:\s*\(\s*\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;`),  // :(){ :|:& };:
	regexp.MustCompile(`(?i)\bfork\s*\(\s*\)\s*while`),                 // fork() while
	regexp.MustCompile(`(?i)\bwhile\s*\(\s*1\s*\)\s*fork`),             // while(1) fork
	regexp.MustCompile(`(?i)\bwhile\s+true.*fork`),                     // while true fork
	regexp.MustCompile(`(?i)\bfor\s*\(\s*;\s*;\s*\)\s*fork`),           // for(;;) fork
	regexp.MustCompile(`\.\s*\|\s*\.`),                                 // . | . (another fork bomb)

	// Dangerous system file manipulation
	regexp.MustCompile(`(?i)\bshred\b`),
	regexp.MustCompile(`(?i)\bwipefs\b`),
	regexp.MustCompile(`(?i)\bblkdiscard\b`),
	regexp.MustCompile(`(?i)>\s*/proc/`),
	regexp.MustCompile(`(?i)>\s*/sys/`),
	regexp.MustCompile(`(?i)\bchmod\s+(-[a-z]*\s+)*777\s+/`),
	regexp.MustCompile(`(?i)\bchown\s+.*\s+/\s*$`),
	regexp.MustCompile(`(?i)\brm\s+.*(/etc/passwd|/etc/shadow|/boot/)`),

	// Dangerous network commands (curl/wget piped to shell)
	regexp.MustCompile(`(?i)\bcurl\s+.*\|\s*(ba)?sh`),
	regexp.MustCompile(`(?i)\bwget\s+.*\|\s*(ba)?sh`),
}

// blockedPatternDescriptions provides human-readable descriptions for each pattern.
var blockedPatternDescriptions = map[int]string{
	0:  "recursive file deletion with dangerous target",
	1:  "rm with --no-preserve-root flag",
	2:  "recursive deletion of root directory contents",
	3:  "rm -rf command (potentially dangerous)",
	4:  "rm -r command (potentially dangerous)",
	5:  "Windows del with force flag",
	6:  "Windows del with recursive flag",
	7:  "Windows rd with recursive flag",
	8:  "Windows rmdir with recursive flag",
	9:  "format command (disk formatting)",
	10: "mkfs command (filesystem creation)",
	11: "diskpart command (disk partitioning)",
	12: "fdisk command (disk partitioning)",
	13: "parted command (disk partitioning)",
	14: "gdisk command (GPT disk partitioning)",
	15: "dd command with input file (potential disk write)",
	16: "dd command writing to disk device",
	17: "redirect to /dev/sd* device",
	18: "redirect to /dev/hd* device",
	19: "redirect to /dev/nvme* device",
	20: "redirect to /dev/vd* device",
	21: "redirect to /dev/xvd* device",
	22: "shutdown command",
	23: "reboot command",
	24: "poweroff command",
	25: "halt command",
	26: "init 0 or init 6 (system state change)",
	27: "systemctl halt/poweroff/reboot/shutdown",
	28: "fork bomb pattern detected",
	29: "fork() in loop",
	30: "infinite fork loop",
	31: "while true fork pattern",
	32: "infinite for loop fork",
	33: "pipe fork bomb pattern",
	34: "shred command (secure file deletion)",
	35: "wipefs command (filesystem signature removal)",
	36: "blkdiscard command (device sector discard)",
	37: "write to /proc filesystem",
	38: "write to /sys filesystem",
	39: "chmod 777 on root",
	40: "chown on root directory",
	41: "removal of critical system files",
	42: "curl piped to shell",
	43: "wget piped to shell",
}

// GuardCommand checks if a command is safe to execute.
// Returns an error message if the command is blocked, empty string if allowed.
func GuardCommand(command string) string {
	// Trim and normalize whitespace
	command = strings.TrimSpace(command)
	if command == "" {
		return "empty command is not allowed"
	}

	// Check against all blocked patterns
	for i, pattern := range blockedCommandPatterns {
		if pattern.MatchString(command) {
			desc := blockedPatternDescriptions[i]
			if desc == "" {
				desc = "dangerous command pattern"
			}
			return "command blocked: " + desc
		}
	}

	// Check for null byte injection
	if strings.Contains(command, "\x00") {
		return "command blocked: null byte injection detected"
	}

	// Check for command substitution that might bypass guards
	// This is a heuristic check for obfuscated commands
	if containsObfuscation(command) {
		return "command blocked: potential command obfuscation detected"
	}

	return ""
}

// containsObfuscation checks for common command obfuscation techniques.
func containsObfuscation(command string) bool {
	// Check for excessive use of escape sequences
	backslashCount := strings.Count(command, "\\")
	if backslashCount > 10 && float64(backslashCount)/float64(len(command)) > 0.1 {
		return true
	}

	// Check for hex escape sequences that might spell dangerous commands
	hexEscapePattern := regexp.MustCompile(`\\x[0-9a-fA-F]{2}`)
	hexMatches := hexEscapePattern.FindAllString(command, -1)
	if len(hexMatches) > 5 {
		return true
	}

	// Check for octal escape sequences
	octalEscapePattern := regexp.MustCompile(`\\[0-7]{1,3}`)
	octalMatches := octalEscapePattern.FindAllString(command, -1)
	if len(octalMatches) > 5 {
		return true
	}

	// Check for base64 decode piped to shell
	base64Pattern := regexp.MustCompile(`(?i)(base64\s+-d|base64\s+--decode).*\|\s*(ba)?sh`)
	if base64Pattern.MatchString(command) {
		return true
	}

	// Check for eval with encoded content
	evalPattern := regexp.MustCompile(`(?i)\beval\s+.*(\$\(|\x60|base64|decode)`)
	if evalPattern.MatchString(command) {
		return true
	}

	return false
}

// IsCommandSafe is a convenience function that returns true if the command is allowed.
func IsCommandSafe(command string) bool {
	return GuardCommand(command) == ""
}

// GuardResult represents the result of a command guard check.
type GuardResult struct {
	// Allowed indicates if the command is safe to execute.
	Allowed bool
	// Reason contains the reason for blocking, empty if allowed.
	Reason string
	// Command is the original command that was checked.
	Command string
}

// CheckCommand performs a guard check and returns a detailed result.
func CheckCommand(command string) GuardResult {
	reason := GuardCommand(command)
	return GuardResult{
		Allowed: reason == "",
		Reason:  reason,
		Command: command,
	}
}
