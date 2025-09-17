package pkg

import (
	"encoding/json"
	"time"

	"live-chatter/pkg/model"

	Log "live-chatter/pkg/logger"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// Client represents a single WebSocket connection with user information
type Client struct {
	User   *model.User     // User information
	Socket *websocket.Conn // WebSocket connection
	Send   chan []byte     // Buffered channel for outgoing messages
	Rooms  map[string]bool // Set of rooms this client has joined
}

// Read continuously listens for incoming messages from the client
func (c *Client) Read(clientsManager *ClientManager) {
	defer func() {
		c.Close(clientsManager)
	}()

	// Set read deadline and message size limit
	c.Socket.SetReadLimit(maxMessageSize)
	err := c.Socket.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return
	}
	c.Socket.SetPongHandler(func(string) error {
		err := c.Socket.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return nil
	})

	for {
		// Read the next message from the WebSocket
		_, messageData, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				Log.Error("WebSocket error for user %s: %v", c.User.Username, err)
			}
			break
		}

		// Process the received message
		c.HandleMessage(messageData, clientsManager)
	}
}

// HandleMessage processes an incoming message based on its type
func (c *Client) HandleMessage(messageData []byte, clientsManager *ClientManager) {
	var incomingMsg IncomingMessage
	if err := json.Unmarshal(messageData, &incomingMsg); err != nil {
		Log.Error("Error unmarshaling message from user %s: %v", c.User.Username, err)
		c.SendError("Invalid message format")
		return
	}

	Log.Info("Received message from %s: type=%s", c.User.Username, incomingMsg.Type)

	switch incomingMsg.Type {
	case "chat_message":
		c.handleChatMessage(incomingMsg, clientsManager)
	case "join_room":
		c.handleJoinRoom(incomingMsg, clientsManager)
	case "leave_room":
		c.handleLeaveRoom(incomingMsg, clientsManager)
	case "private_message":
		c.handlePrivateMessage(incomingMsg, clientsManager)
	case "typing":
		c.handleTyping(incomingMsg, clientsManager)
	case "ping":
		c.handlePing()
	default:
		Log.Warn("Unknown message type '%s' from user %s", incomingMsg.Type, c.User.Username)
		c.SendError("Unknown message type")
	}
}

// handleChatMessage processes chat messages
func (c *Client) handleChatMessage(msg IncomingMessage, clientsManager *ClientManager) {
	if msg.Content == "" {
		c.SendError("Message content cannot be empty")
		return
	}

	// Create chat message
	chatMsg := &Message{
		ID:        generateMessageID(),
		Type:      "chat_message",
		Content:   msg.Content,
		UserID:    c.User.ID,
		Username:  c.User.Username,
		RoomID:    msg.RoomID,
		Timestamp: time.Now(),
	}

	// Broadcast to room or general chat
	broadcastMsg := BroadcastMessage{
		Message:     chatMsg,
		RoomID:      msg.RoomID,
		ExcludeUser: "", // Include sender in chat messages
		MessageType: "broadcast_room",
	}

	clientsManager.Broadcast <- broadcastMsg
}

// handleJoinRoom processes room join requests
func (c *Client) handleJoinRoom(msg IncomingMessage, clientsManager *ClientManager) {
	if msg.RoomID == "" {
		c.SendError("Room ID cannot be empty")
		return
	}

	// Add client to room
	if clientsManager.Rooms[msg.RoomID] == nil {
		clientsManager.Rooms[msg.RoomID] = make(map[*Client]bool)
	}
	clientsManager.Rooms[msg.RoomID][c] = true
	c.Rooms[msg.RoomID] = true

	// Send confirmation to user
	confirmMsg := &Message{
		ID:        generateMessageID(),
		Type:      "room_joined",
		Content:   "Successfully joined room",
		RoomID:    msg.RoomID,
		Username:  "System",
		Timestamp: time.Now(),
	}

	c.SendMessage(confirmMsg)

	// Notify other room members
	notifyMsg := &Message{
		ID:        generateMessageID(),
		Type:      "user_joined",
		Content:   c.User.Username + " joined the room",
		UserID:    c.User.ID,
		Username:  c.User.Username,
		RoomID:    msg.RoomID,
		Timestamp: time.Now(),
	}

	broadcastMsg := BroadcastMessage{
		Message:     notifyMsg,
		RoomID:      msg.RoomID,
		ExcludeUser: c.User.Username,
		MessageType: "broadcast_room",
	}

	clientsManager.Broadcast <- broadcastMsg

	Log.Info("User %s joined room %s", c.User.Username, msg.RoomID)
}

// handleLeaveRoom processes room leave requests
func (c *Client) handleLeaveRoom(msg IncomingMessage, clientsManager *ClientManager) {
	if msg.RoomID == "" {
		c.SendError("Room ID cannot be empty")
		return
	}

	// Remove client from room
	if clientsManager.Rooms[msg.RoomID] != nil {
		delete(clientsManager.Rooms[msg.RoomID], c)
		if len(clientsManager.Rooms[msg.RoomID]) == 0 {
			delete(clientsManager.Rooms, msg.RoomID)
		}
	}
	delete(c.Rooms, msg.RoomID)

	// Send confirmation to user
	confirmMsg := &Message{
		ID:        generateMessageID(),
		Type:      "room_left",
		Content:   "Successfully left room",
		RoomID:    msg.RoomID,
		Username:  "System",
		Timestamp: time.Now(),
	}

	c.SendMessage(confirmMsg)

	// Notify other room members
	if clientsManager.Rooms[msg.RoomID] != nil && len(clientsManager.Rooms[msg.RoomID]) > 0 {
		notifyMsg := &Message{
			ID:        generateMessageID(),
			Type:      "user_left",
			Content:   c.User.Username + " left the room",
			UserID:    c.User.ID,
			Username:  c.User.Username,
			RoomID:    msg.RoomID,
			Timestamp: time.Now(),
		}

		broadcastMsg := BroadcastMessage{
			Message:     notifyMsg,
			RoomID:      msg.RoomID,
			ExcludeUser: c.User.Username,
			MessageType: "broadcast_room",
		}

		clientsManager.Broadcast <- broadcastMsg
	}

	Log.Info("User %s left room %s", c.User.Username, msg.RoomID)
}

// handlePrivateMessage processes private messages between users
func (c *Client) handlePrivateMessage(msg IncomingMessage, clientsManager *ClientManager) {
	if msg.RecipientUsername == "" {
		c.SendError("Recipient username cannot be empty")
		return
	}

	if msg.Content == "" {
		c.SendError("Message content cannot be empty")
		return
	}

	// Create private message
	privateMsg := &Message{
		ID:                generateMessageID(),
		Type:              "private_message",
		Content:           msg.Content,
		UserID:            c.User.ID,
		Username:          c.User.Username,
		RecipientUsername: msg.RecipientUsername,
		Timestamp:         time.Now(),
	}

	// Send to specific user
	broadcastMsg := BroadcastMessage{
		Message:        privateMsg,
		TargetUsername: msg.RecipientUsername,
		MessageType:    "private_message",
	}

	clientsManager.Broadcast <- broadcastMsg

	// Send copy to sender
	c.SendMessage(privateMsg)
}

// handleTyping processes typing indicators
func (c *Client) handleTyping(msg IncomingMessage, clientsManager *ClientManager) {
	typingMsg := &Message{
		ID:        generateMessageID(),
		Type:      "typing",
		UserID:    c.User.ID,
		Username:  c.User.Username,
		RoomID:    msg.RoomID,
		Content:   msg.Content, // "start" or "stop"
		Timestamp: time.Now(),
	}

	broadcastMsg := BroadcastMessage{
		Message:     typingMsg,
		RoomID:      msg.RoomID,
		ExcludeUser: c.User.Username,
		MessageType: "broadcast_room",
	}

	clientsManager.Broadcast <- broadcastMsg
}

// handlePing responds to ping messages
func (c *Client) handlePing() {
	pongMsg := &Message{
		ID:        generateMessageID(),
		Type:      "pong",
		Username:  "System",
		Timestamp: time.Now(),
	}
	c.SendMessage(pongMsg)
}

// SendMessage sends a message to this client
func (c *Client) SendMessage(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		Log.Error("Error marshaling message for user %s: %v", c.User.Username, err)
		return
	}

	select {
	case c.Send <- data:
	default:
		Log.Warn("Send channel full for user %s, closing connection", c.User.Username)
		close(c.Send)
	}
}

// SendError sends an error message to this client
func (c *Client) SendError(errorMsg string) {
	msg := &Message{
		ID:        generateMessageID(),
		Type:      "error",
		Content:   errorMsg,
		Username:  "System",
		Timestamp: time.Now(),
	}
	c.SendMessage(msg)
}

// Close unregisters the client and closes its WebSocket connection
func (c *Client) Close(clientsManager *ClientManager) {
	Log.Info("Closing connection for user: %s", c.User.Username)
	clientsManager.Unregister <- c
}

// Write listens for outgoing messages and sends them to the WebSocket
func (c *Client) Write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		err := c.Socket.Close()
		if err != nil {
			return
		}
	}()

	for {
		select {
		case message, ok := <-c.Send:
			err := c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				return
			}
			if !ok {
				err := c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					return
				}
				return
			}

			if err := c.Socket.WriteMessage(websocket.TextMessage, message); err != nil {
				Log.Error("Write error for user %s: %v", c.User.Username, err)
				return
			}

		case <-ticker.C:
			err := c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				return
			}
			if err := c.Socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				Log.Error("Ping error for user %s: %v", c.User.Username, err)
				return
			}
		}
	}
}

// Helper function to generate message IDs
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000")
}
