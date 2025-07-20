package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin in development
	},
}

// MessageType represents different types of WebSocket messages
type MessageType string

const (
	MessageTypeTranslationProgress MessageType = "translation_progress"
	MessageTypeTranslationComplete MessageType = "translation_complete"
	MessageTypeTranslationError    MessageType = "translation_error"
	MessageTypeLog                 MessageType = "log"
	MessageTypePageTranslation     MessageType = "page_translation"
	MessageTypeChapterList         MessageType = "chapter_list"
	MessageTypeLLMRequest          MessageType = "llm_request"
	MessageTypeLLMResponse         MessageType = "llm_response"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      MessageType `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// LogMessage represents a log entry for real-time streaming
type LogMessage struct {
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
	Module  string    `json:"module,omitempty"`
}

// TranslationProgressMessage represents translation progress updates
type TranslationProgressMessage struct {
	EPUBID            string  `json:"epub_id"`
	TotalChapters     int     `json:"total_chapters"`
	CompletedChapters int     `json:"completed_chapters"`
	CurrentChapter    string  `json:"current_chapter"`
	ProgressPercent   float64 `json:"progress_percent"`
	Status            string  `json:"status"`
}

// PageTranslationMessage represents single page translation results
type PageTranslationMessage struct {
	EPUBID         string `json:"epub_id"`
	ChapterID      string `json:"chapter_id"`
	OriginalText   string `json:"original_text"`
	TranslatedText string `json:"translated_text"`
	SourceLanguage string `json:"source_language"`
	TargetLanguage string `json:"target_language"`
}

// LLMRequestMessage represents an LLM request for debugging
type LLMRequestMessage struct {
	RequestID   string                 `json:"request_id"`
	Model       string                 `json:"model"`
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float32                `json:"temperature"`
	Timestamp   time.Time              `json:"timestamp"`
	RequestType string                 `json:"request_type"` // "translation", "detection", etc.
	Context     map[string]interface{} `json:"context,omitempty"`
}

// LLMResponseMessage represents an LLM response for debugging
type LLMResponseMessage struct {
	RequestID    string                 `json:"request_id"`
	Response     string                 `json:"response"`
	TokensUsed   int                    `json:"tokens_used,omitempty"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	conn   *websocket.Conn
	send   chan WebSocketMessage
	hub    *Hub
	logger *logrus.Logger
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan WebSocketMessage
	register   chan *Client
	unregister chan *Client
	logger     *logrus.Logger
	mutex      sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub(logger *logrus.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan WebSocketMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

// Run starts the WebSocket hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			h.logger.Debugf("WebSocket client connected. Total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.mutex.Unlock()
				h.logger.Debugf("WebSocket client disconnected. Total clients: %d", len(h.clients))
			} else {
				h.mutex.Unlock()
			}

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// BroadcastMessage sends a message to all connected clients
func (h *Hub) BroadcastMessage(msgType interface{}, data interface{}) {
	// Convert msgType to MessageType
	var messageType MessageType
	if mt, ok := msgType.(string); ok {
		messageType = MessageType(mt)
	} else if mt, ok := msgType.(MessageType); ok {
		messageType = mt
	} else {
		h.logger.Warnf("Invalid message type: %v", msgType)
		return
	}

	message := WebSocketMessage{
		Type:      messageType,
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case h.broadcast <- message:
	default:
		h.logger.Warn("WebSocket broadcast channel is full, dropping message")
	}
}

// BroadcastLog sends a log message to all connected clients
func (h *Hub) BroadcastLog(level, message, module string) {
	logMsg := LogMessage{
		Level:   level,
		Message: message,
		Time:    time.Now(),
		Module:  module,
	}
	h.BroadcastMessage(MessageTypeLog, logMsg)
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Debugf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			jsonData, err := json.Marshal(message)
			if err != nil {
				c.logger.Errorf("Failed to marshal WebSocket message: %v", err)
				continue
			}

			_, _ = w.Write(jsonData)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				nextMessage := <-c.send
				nextJsonData, err := json.Marshal(nextMessage)
				if err != nil {
					c.logger.Errorf("Failed to marshal queued WebSocket message: %v", err)
					continue
				}
				_, _ = w.Write(nextJsonData)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (s *Server) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Errorf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	client := &Client{
		conn:   conn,
		send:   make(chan WebSocketMessage, 256),
		hub:    s.wsHub,
		logger: s.logger,
	}

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}
