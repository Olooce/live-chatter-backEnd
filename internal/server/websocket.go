package server

import (
	"net/http"

	"live-chatter/pkg"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

// WebSocket upgrades an HTTP request to a WebSocket connection
// and manages the client lifecycle with the given ClientManager.
func WebSocket(res http.ResponseWriter, req *http.Request, clientsManager *pkg.ClientManager) {
	// Upgrade the incoming HTTP request to a WebSocket connection.
	// CheckOrigin always returns true to allow connections from any origin.
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}).Upgrade(res, req, nil)
	if err != nil {
		// If upgrade fails, respond with 404 Not Found.
		http.NotFound(res, req)
		return
	}

	// Create a new client with a unique ID and a send channel for outgoing messages.
	client := &pkg.Client{
		ID:     uuid.NewV4().String(),
		Socket: conn,
		Send:   make(chan []byte),
	}

	// Register the client with the client manager to start tracking it.
	clientsManager.Register <- client

	// Start a goroutine to handle incoming messages from this client.
	go client.Read(clientsManager)

	// Start a goroutine to handle outgoing messages to this client.
	go client.Write()
}
