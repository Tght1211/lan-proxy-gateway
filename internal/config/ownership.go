package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// ReclaimToSudoUser chowns the given path (and walks directories) back to the
// user that invoked sudo, so the user can delete / read the files as their
// normal (non-root) identity later.
//
// Has no effect when SUDO_UID is not set (i.e. we weren't invoked via sudo).
func ReclaimToSudoUser(path string) {
	uid, gid, ok := sudoUIDGID()
	if !ok {
		return
	}
	_ = os.Chown(path, uid, gid)
	// Walk children if path is a directory (best-effort).
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chown(p, uid, gid)
			}
			return nil
		})
	}
}

func sudoUIDGID() (int, int, bool) {
	uidStr := os.Getenv("SUDO_UID")
	if uidStr == "" {
		return 0, 0, false
	}
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return 0, 0, false
	}
	gidStr := os.Getenv("SUDO_GID")
	gid, _ := strconv.Atoi(gidStr)
	return uid, gid, true
}

// SudoUserHome returns the home of the user that invoked sudo, or "" if not under sudo.
// This lets callers write config into the original user's home dir even when
// running as root.
func SudoUserHome() string {
	if os.Getenv("SUDO_UID") == "" {
		return ""
	}
	// macOS: `sudo` preserves HOME unless `-H` is used. Linux defaults vary; trust
	// SUDO_USER's home directory via os/user if HOME has been reset to /root.
	home := os.Getenv("HOME")
	if home == "" || home == "/root" || home == "/var/root" {
		// Fall back to passwd lookup via HOME of SUDO_USER.
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			// Standard install locations; minimal heuristic without calling os/user.
			candidates := []string{
				"/Users/" + sudoUser,
				"/home/" + sudoUser,
			}
			for _, c := range candidates {
				if info, err := os.Stat(c); err == nil && info.IsDir() {
					return c
				}
			}
		}
	}
	return home
}
