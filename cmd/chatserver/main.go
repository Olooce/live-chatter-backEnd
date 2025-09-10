package main

import (
	"fmt"

	"live-chatter/internal/server"
	"live-chatter/pkg"

	"github.com/gin-gonic/gin"
)

var (
	// clientsManager handles all WebSocket clients, registration, unregistration, and broadcasting messages
	clientsManager = &pkg.ClientManager{
		Broadcast:  make(chan []byte),          // Channel for broadcasting messages to all clients
		Register:   make(chan *pkg.Client),     // Channel for registering new clients
		Unregister: make(chan *pkg.Client),     // Channel for removing disconnected clients
		Clients:    make(map[*pkg.Client]bool), // Map to keep track of active clients
	}
)

func main() {
	fmt.Println("Gin WebSocket server starting...")

	// Start the client manager in a separate goroutine to handle client events concurrently
	go clientsManager.Start()

	// Create a new Gin router instance
	r := gin.Default()

	// Define WebSocket endpoint at root path
	r.GET("/", func(c *gin.Context) {
		// Upgrade the incoming HTTP request to a WebSocket connection
		server.WebSocket(c.Writer, c.Request, clientsManager)
	})

	// Start the Gin server on port 5000
	if err := r.Run(":5000"); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
