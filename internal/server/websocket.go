package server

import (
	"live-chatter/pkg"
	"live-chatter/pkg/model"
	"net/http"

	Log "live-chatter/pkg/logger"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: implement proper origin checking
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocket upgrades an HTTP request to a WebSocket connection
// and manages the client lifecycle with the given ClientManager.
func WebSocket(res http.ResponseWriter, req *http.Request, clientsManager *pkg.ClientManager) {
	// Get user information from context (set by WebSocketAuthMiddleware)
	userID, ok := req.Context().Value("user_id").(uint)
	if !ok {
		Log.Error("User ID not found in WebSocket request context")
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username, exists := req.Context().Value("username").(string)
	if !exists {
		Log.Error("Username not found in WebSocket request context")
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	email, exists := req.Context().Value("email").(string)
	if !exists {
		Log.Error("Email not found in WebSocket request context")
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade the incoming HTTP request to a WebSocket connection
	conn, err := upgrader.Upgrade(res, req, nil)
	if err != nil {
		Log.Error("Failed to upgrade WebSocket connection: %v", err)
		http.Error(res, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}

	// Create user info
	user := &model.User{
		ID:       uint(userID),
		Username: username,
		Email:    email,
	}

	// Create a new client with user information
	client := &pkg.Client{
		User:   user,
		Socket: conn,
		Send:   make(chan []byte, 256),
		Rooms:  make(map[string]bool),
	}

	Log.Info("WebSocket connection established for user: %s (ID: %d)", username, userID)

	// Register the client with the client manager to start tracking it
	clientsManager.Register <- client

	// Start goroutines to handle incoming and outgoing messages
	go client.Read(clientsManager)
	go client.Write()
}
