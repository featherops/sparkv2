//go:build windows

package autostart

import (
    "golang.org/x/sys/windows/registry"
    "os"
    "path/filepath"
    "os/exec"
)

// Enable registers the executable in HKCU\...\Run to auto-start at user login.
func Enable() error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }
    exe, _ = filepath.Abs(exe)
    k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\\Microsoft\\Windows\\CurrentVersion\\Run`, registry.SET_VALUE)
    if err != nil {
        return err
    }
    defer k.Close()
    return k.SetStringValue("SparkClient", `"`+exe+`" --background`)
}

// EnableScheduledTask creates a user-level scheduled task as a fallback
// so the program starts at logon even if Run entries are disabled.
// Requires elevation.
func EnableScheduledTask() error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }
    exe, _ = filepath.Abs(exe)
    // Create or update a task named "SparkClient" that runs at logon.
    // Use /RL HIGHEST to retain elevated privileges.
    // Ignore errors if schtasks is unavailable.
    cmd := exec.Command("schtasks", "/Create", "/TN", "SparkClient", "/TR", exe+" --background", "/SC", "ONLOGON", "/RL", "HIGHEST", "/F")
    cmd.SysProcAttr = nil
    return cmd.Run()
}

