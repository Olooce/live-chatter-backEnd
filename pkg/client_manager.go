package pkg

import "encoding/json"

// ClientManager keeps track of all connected WebSocket clients
// and handles broadcasting messages as well as client registration/unregistration.
type ClientManager struct {
	Clients    map[*Client]bool // Map of active clients
	Broadcast  chan []byte      // Channel for broadcasting messages to all clients
	Register   chan *Client     // Channel for adding new clients
	Unregister chan *Client     // Channel for removing disconnected clients
}

// Start runs the client manager in a separate goroutine.
// It continuously listens on the Register, Unregister, and Broadcast channels
// and handles adding/removing clients and broadcasting messages.
func (manager *ClientManager) Start() {
	for {
		select {
		case client := <-manager.Register:
			// Add new client to the active clients map
			manager.Clients[client] = true

			// Notify all other clients about the new connection
			jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
			manager.Send(jsonMessage, client)

		case client := <-manager.Unregister:
			if _, ok := manager.Clients[client]; ok {
				// Close the client's send channel
				close(client.Send)

				// Remove the client from the active clients map
				delete(manager.Clients, client)

				// Notify remaining clients about the disconnection
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				manager.Send(jsonMessage, client)
			}

		case message := <-manager.Broadcast:
			// Broadcast incoming message to all active clients
			for client := range manager.Clients {
				if client != nil {
					select {
					case client.Send <- message:
						// Message sent successfully
					default:
						// Client not receiving, assume disconnected and clean up
						close(client.Send)
						delete(manager.Clients, client)
					}
				}
			}
		}
	}
}

// Send broadcasts a message to all connected clients except the one specified in "ignore".
// Useful for notifications where the sender should not receive the message.
func (manager *ClientManager) Send(message []byte, ignore *Client) {
	for client := range manager.Clients {
		if client != ignore {
			client.Send <- message
		}
	}
}
