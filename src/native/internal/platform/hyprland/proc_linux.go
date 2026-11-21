//go:build linux

package hyprland

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// CompositorArgs reads the command line of this user's running Hyprland from /proc, to find a --config it was started
// with.
func CompositorArgs() ([]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	uid := uint32(os.Getuid())
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		dir := filepath.Join("/proc", e.Name())
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "Hyprland" {
			continue
		}
		if info, err := os.Stat(dir); err != nil || info.Sys().(*syscall.Stat_t).Uid != uid {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "cmdline"))
		if err != nil {
			return nil, err
		}
		return strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00"), nil
	}
	return nil, errors.New("hyprland: no Hyprland process of this user")
}
