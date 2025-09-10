package pkg

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// Client represents a single WebSocket connection
// and handles sending and receiving messages.
type Client struct {
	ID     string          // Unique identifier for the client
	Socket *websocket.Conn // WebSocket connection
	Send   chan []byte     // Channel for outgoing messages
}

// Read continuously listens for incoming messages from the client.
func (c *Client) Read(clientsManager *ClientManager) {
	defer func() {
		// Ensure client is properly closed and unregistered on exit
		c.Close(clientsManager)
	}()

	for {
		// Read the next message from the WebSocket
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			// Close the connection if there’s a read error (e.g., client disconnected)
			c.Close(clientsManager)
			break
		}

		// Process the received message and broadcast to other clients
		c.HandleMessage(message, clientsManager)
	}
}

// HandleMessage processes an incoming message and broadcasts it to all clients.
func (c *Client) HandleMessage(message []byte, clientsManager *ClientManager) {
	// Wrap the message with sender info and marshal into JSON
	jsonMessage, err := json.Marshal(&Message{Sender: c.ID, Content: string(message)})
	if err != nil {
		log.Println("Error marshaling message:", err)
		return
	}

	// Send the JSON message to the broadcast channel for all connected clients
	clientsManager.Broadcast <- jsonMessage
}

// Close unregisters the client and closes its WebSocket connection.
func (c *Client) Close(clientsManager *ClientManager) {
	// Remove client from the manager
	clientsManager.Unregister <- c

	// Close the WebSocket connection and log any errors
	if err := c.Socket.Close(); err != nil {
		log.Println("Error closing socket:", err)
	}
}

// Write listens for outgoing messages and sends them to the WebSocket.
func (c *Client) Write() {
	defer func() {
		// Ensure socket is closed when exiting the write loop
		_ = c.Socket.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// If the channel is closed, send a close frame and exit
				_ = c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send the message as a text frame to the client
			if err := c.Socket.WriteMessage(websocket.TextMessage, message); err != nil {
				// Exit loop on write error (client likely disconnected)
				return
			}
		}
	}
}
