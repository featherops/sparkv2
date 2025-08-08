//go:build linux

package main

import (
    "io"
    "os"
    "os/exec"
    "path/filepath"
)

func init() {
    // Install to ~/.local/share/Spark and relaunch once.
    if ensureInstallPathLinux() {
        os.Exit(0)
        return
    }
    // Create autostart .desktop under ~/.config/autostart
    _ = enableAutostartLinux()
}

func ensureInstallPathLinux() bool {
    home, _ := os.UserHomeDir()
    if len(home) == 0 {
        return false
    }
    targetDir := filepath.Join(home, ".local", "share", "Spark")
    _ = os.MkdirAll(targetDir, 0755)
    exe, _ := os.Executable()
    exeAbs, _ := filepath.Abs(exe)
    target := filepath.Join(targetDir, filepath.Base(exeAbs))
    if filepath.Clean(exeAbs) == filepath.Clean(target) {
        return false
    }
    in, err := os.Open(exeAbs)
    if err != nil { return false }
    defer in.Close()
    out, err := os.Create(target)
    if err != nil { return false }
    if _, err = io.Copy(out, in); err != nil { _ = out.Close(); return false }
    _ = out.Close()
    _ = os.Chmod(target, 0755)
    cmd := exec.Command(target, "--installed", "--background")
    _ = cmd.Start()
    return true
}

func enableAutostartLinux() error {
    home, _ := os.UserHomeDir()
    if len(home) == 0 { return nil }
    dir := filepath.Join(home, ".config", "autostart")
    _ = os.MkdirAll(dir, 0755)
    exe, _ := os.Executable()
    exeAbs, _ := filepath.Abs(exe)
    desktop := "[Desktop Entry]\nType=Application\nName=SparkClient\nExec=" + exeAbs + " --background\nX-GNOME-Autostart-enabled=true\n"
    return os.WriteFile(filepath.Join(dir, "sparkclient.desktop"), []byte(desktop), 0644)
}

