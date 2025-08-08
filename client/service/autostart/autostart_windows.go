//go:build windows

package autostart

import (
    "golang.org/x/sys/windows/registry"
    "os"
    "path/filepath"
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

