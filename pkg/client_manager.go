package pkg

import (
	"encoding/json"
	"time"

	Log "live-chatter/pkg/logger"
)

// ClientManager keeps track of all connected WebSocket clients
// and handles broadcasting messages as well as client registration/unregistration.
type ClientManager struct {
	Clients     map[*Client]bool            // Map of active clients
	Broadcast   chan BroadcastMessage       // Channel for broadcasting messages
	Register    chan *Client                // Channel for adding new clients
	Unregister  chan *Client                // Channel for removing disconnected clients
	Rooms       map[string]map[*Client]bool // Map of rooms to clients
	UserClients map[string]*Client          // Map of usernames to clients (for private messages)
}

// BroadcastMessage represents different types of broadcast operations
type BroadcastMessage struct {
	Message        *Message `json:"message"`
	RoomID         string   `json:"room_id,omitempty"`
	TargetUsername string   `json:"target_username,omitempty"`
	ExcludeUser    string   `json:"exclude_user,omitempty"`
	MessageType    string   `json:"message_type"` // "broadcast_all", "broadcast_room", "private_message"
}

// Start runs the client manager in a separate goroutine.
// It continuously listens on the Register, Unregister, and Broadcast channels.
func (manager *ClientManager) Start() {
	Log.Info("Client manager started")

	for {
		select {
		case client := <-manager.Register:
			manager.registerClient(client)

		case client := <-manager.Unregister:
			manager.unregisterClient(client)

		case broadcastMsg := <-manager.Broadcast:
			manager.handleBroadcast(broadcastMsg)
		}
	}
}

// registerClient adds a new client to the manager
func (manager *ClientManager) registerClient(client *Client) {
	// Check if user is already connected and disconnect old connection
	if existingClient, exists := manager.UserClients[client.User.Username]; exists {
		Log.Info("User %s reconnecting, closing old connection", client.User.Username)
		manager.forceDisconnectClient(existingClient)
	}

	// Add client to active clients
	manager.Clients[client] = true
	manager.UserClients[client.User.Username] = client

	Log.Info("User %s connected (Total connections: %d)",
		client.User.Username, len(manager.Clients))

	// Send welcome message to the new client
	welcomeMsg := &Message{
		ID:        generateMessageID(),
		Type:      "system",
		Content:   "Welcome to Chatter! You are now connected.",
		Username:  "System",
		Timestamp: time.Now(),
	}
	client.SendMessage(welcomeMsg)

	// Notify other users about the new connection
	notificationMsg := &Message{
		ID:        generateMessageID(),
		Type:      "user_connected",
		Content:   client.User.Username + " joined the chat",
		UserID:    client.User.ID,
		Username:  client.User.Username,
		Timestamp: time.Now(),
	}

	// Broadcast to all other clients
	manager.broadcastToAll(notificationMsg, client.User.Username)

	// Send current online users list to the new client
	manager.sendOnlineUsersList(client)
}

// unregisterClient removes a client from the manager
func (manager *ClientManager) unregisterClient(client *Client) {
	if _, ok := manager.Clients[client]; ok {
		// Close the client's send channel
		close(client.Send)

		// Remove from all data structures
		delete(manager.Clients, client)
		delete(manager.UserClients, client.User.Username)

		// Remove from all rooms
		for roomID := range client.Rooms {
			if manager.Rooms[roomID] != nil {
				delete(manager.Rooms[roomID], client)
				if len(manager.Rooms[roomID]) == 0 {
					delete(manager.Rooms, roomID)
				}
			}
		}

		Log.Info("User %s disconnected (Total connections: %d)",
			client.User.Username, len(manager.Clients))

		// Notify other users about the disconnection
		notificationMsg := &Message{
			ID:        generateMessageID(),
			Type:      "user_disconnected",
			Content:   client.User.Username + " left the chat",
			UserID:    client.User.ID,
			Username:  client.User.Username,
			Timestamp: time.Now(),
		}

		// Broadcast to all other clients
		manager.broadcastToAll(notificationMsg, client.User.Username)
	}
}

// forceDisconnectClient forcefully disconnects a client
func (manager *ClientManager) forceDisconnectClient(client *Client) {
	if client.Socket != nil {
		client.Socket.Close()
	}
	manager.unregisterClient(client)
}

// handleBroadcast processes different types of broadcast messages
func (manager *ClientManager) handleBroadcast(broadcastMsg BroadcastMessage) {
	switch broadcastMsg.MessageType {
	case "broadcast_all":
		manager.broadcastToAll(broadcastMsg.Message, broadcastMsg.ExcludeUser)

	case "broadcast_room":
		manager.broadcastToRoom(broadcastMsg.Message, broadcastMsg.RoomID, broadcastMsg.ExcludeUser)

	case "private_message":
		manager.sendPrivateMessage(broadcastMsg.Message, broadcastMsg.TargetUsername)

	default:
		Log.Warn("Unknown broadcast message type: %s", broadcastMsg.MessageType)
	}
}

// broadcastToAll sends a message to all connected clients
func (manager *ClientManager) broadcastToAll(message *Message, excludeUser string) {
	data, err := json.Marshal(message)
	if err != nil {
		Log.Error("Error marshaling broadcast message: %v", err)
		return
	}

	count := 0
	for client := range manager.Clients {
		if client.User.Username != excludeUser {
			select {
			case client.Send <- data:
				count++
			default:
				Log.Warn("Client %s not receiving, cleaning up", client.User.Username)
				manager.cleanupClient(client)
			}
		}
	}

	Log.Info("Broadcasted message to %d clients (type: %s)", count, message.Type)
}

// broadcastToRoom sends a message to all clients in a specific room
func (manager *ClientManager) broadcastToRoom(message *Message, roomID string, excludeUser string) {
	if roomID == "" {
		manager.broadcastToAll(message, excludeUser)
		return
	}

	roomClients, exists := manager.Rooms[roomID]
	if !exists {
		Log.Warn("Attempted to broadcast to non-existent room: %s", roomID)
		return
	}

	data, err := json.Marshal(message)
	if err != nil {
		Log.Error("Error marshaling room message: %v", err)
		return
	}

	count := 0
	for client := range roomClients {
		if client.User.Username != excludeUser {
			select {
			case client.Send <- data:
				count++
			default:
				Log.Warn("Client %s in room %s not receiving, cleaning up", client.User.Username, roomID)
				manager.cleanupClient(client)
			}
		}
	}

	Log.Info("Broadcasted message to %d clients in room %s (type: %s)", count, roomID, message.Type)
}

// sendPrivateMessage sends a message to a specific user
func (manager *ClientManager) sendPrivateMessage(message *Message, targetUsername string) {
	targetClient, exists := manager.UserClients[targetUsername]
	if !exists {
		Log.Warn("Attempted to send private message to offline user: %s", targetUsername)

		// Send error message back to sender
		if senderClient, senderExists := manager.UserClients[message.Username]; senderExists {
			errorMsg := &Message{
				ID:        generateMessageID(),
				Type:      "error",
				Content:   "User " + targetUsername + " is not online",
				Username:  "System",
				Timestamp: time.Now(),
			}
			senderClient.SendMessage(errorMsg)
		}
		return
	}

	data, err := json.Marshal(message)
	if err != nil {
		Log.Error("Error marshaling private message: %v", err)
		return
	}

	select {
	case targetClient.Send <- data:
		Log.Info("Private message sent from %s to %s", message.Username, targetUsername)
	default:
		Log.Warn("Target client %s not receiving private message, cleaning up", targetUsername)
		manager.cleanupClient(targetClient)
	}
}

// cleanupClient removes a non-responsive client
func (manager *ClientManager) cleanupClient(client *Client) {
	close(client.Send)
	delete(manager.Clients, client)
	delete(manager.UserClients, client.User.Username)

	// Remove from all rooms
	for roomID := range client.Rooms {
		if manager.Rooms[roomID] != nil {
			delete(manager.Rooms[roomID], client)
			if len(manager.Rooms[roomID]) == 0 {
				delete(manager.Rooms, roomID)
			}
		}
	}
}

// sendOnlineUsersList sends the current list of online users to a client
func (manager *ClientManager) sendOnlineUsersList(client *Client) {
	var onlineUsers []string
	for username := range manager.UserClients {
		if username != client.User.Username {
			onlineUsers = append(onlineUsers, username)
		}
	}

	usersListMsg := &Message{
		ID:        generateMessageID(),
		Type:      "online_users",
		Content:   "",
		Username:  "System",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"users": onlineUsers,
			"count": len(onlineUsers),
		},
	}

	client.SendMessage(usersListMsg)
}

// GetOnlineUsers returns a list of currently online users
func (manager *ClientManager) GetOnlineUsers() []string {
	var users []string
	for username := range manager.UserClients {
		users = append(users, username)
	}
	return users
}

// GetRoomUsers returns a list of users in a specific room
func (manager *ClientManager) GetRoomUsers(roomID string) []string {
	var users []string
	if roomClients, exists := manager.Rooms[roomID]; exists {
		for client := range roomClients {
			users = append(users, client.User.Username)
		}
	}
	return users
}

// GetClientCount returns the number of connected clients
func (manager *ClientManager) GetClientCount() int {
	return len(manager.Clients)
}

// GetRoomCount returns the number of active rooms
func (manager *ClientManager) GetRoomCount() int {
	return len(manager.Rooms)
}

// IsUserOnline checks if a user is currently online
func (manager *ClientManager) IsUserOnline(username string) bool {
	_, exists := manager.UserClients[username]
	return exists
}
