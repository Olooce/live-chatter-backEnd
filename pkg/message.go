package pkg

import "time"

// Message represents a chat message with enhanced fields
type Message struct {
	ID                string                 `json:"id"`
	Type              string                 `json:"type"` // chat_message, system_message, user_joined, user_left, etc.
	Content           string                 `json:"content"`
	UserID            uint                   `json:"user_id,omitempty"`
	Username          string                 `json:"username"`
	RoomID            string                 `json:"room_id,omitempty"`
	RecipientUsername string                 `json:"recipient_username,omitempty"`
	Timestamp         time.Time              `json:"timestamp"`
	Data              map[string]interface{} `json:"data,omitempty"` // For additional metadata
}

// IncomingMessage represents messages received from clients
type IncomingMessage struct {
	Type              string `json:"type"`
	Content           string `json:"content"`
	RoomID            string `json:"room_id,omitempty"`
	RecipientUsername string `json:"recipient_username,omitempty"`
}

// MessageType constants for different message types
const (
	// Chat related messages
	MessageTypeChatMessage    = "chat_message"
	MessageTypePrivateMessage = "private_message"

	// System messages
	MessageTypeSystemMessage = "system_message"
	MessageTypeError         = "error"
	MessageTypeSuccess       = "success"

	// User status messages
	MessageTypeUserConnected    = "user_connected"
	MessageTypeUserDisconnected = "user_disconnected"
	MessageTypeUserJoined       = "user_joined"
	MessageTypeUserLeft         = "user_left"

	// Room management messages
	MessageTypeRoomJoined  = "room_joined"
	MessageTypeRoomLeft    = "room_left"
	MessageTypeRoomCreated = "room_created"

	// Real-time indicators
	MessageTypeTyping      = "typing"
	MessageTypeOnlineUsers = "online_users"

	// Connection management
	MessageTypePing = "ping"
	MessageTypePong = "pong"
)

// TypingStatus represents typing indicator states
type TypingStatus struct {
	Username  string    `json:"username"`
	RoomID    string    `json:"room_id,omitempty"`
	Status    string    `json:"status"` // "start" or "stop"
	Timestamp time.Time `json:"timestamp"`
}

// OnlineUsersResponse represents the response for online users request
type OnlineUsersResponse struct {
	Users []UserInfo `json:"users"`
	Count int        `json:"count"`
}

// UserInfo represents basic user information
type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Status   string `json:"status"` // online, away, busy
}

// RoomInfo represents room information
type RoomInfo struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	UserCount   int        `json:"user_count"`
	Users       []UserInfo `json:"users,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ChatHistoryRequest represents a request for chat history
type ChatHistoryRequest struct {
	RoomID string    `json:"room_id,omitempty"`
	Before time.Time `json:"before,omitempty"`
	Limit  int       `json:"limit,omitempty"`
}

// ChatHistoryResponse represents chat history response
type ChatHistoryResponse struct {
	Messages []Message `json:"messages"`
	HasMore  bool      `json:"has_more"`
}
