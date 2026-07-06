package handlers

import "sync"

type Hub struct {
	mu      sync.RWMutex
	clients map[int]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int]map[*Client]bool),
	}
}

func (hub *Hub) AddClient(userID int, client *Client) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.clients[userID] == nil {
		hub.clients[userID] = make(map[*Client]bool)
	}

	hub.clients[userID][client] = true
}

func (hub *Hub) RemoveClient(userID int, client *Client) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.clients[userID] == nil {
		return
	}

	delete(hub.clients[userID], client)

	if len(hub.clients[userID]) == 0 {
		delete(hub.clients, userID)
	}
}

func (hub *Hub) IsOnline(userID int) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	return len(hub.clients[userID]) > 0
}

func (hub *Hub) SendToUser(userID int, message wsOutgoingMessage) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	for client := range hub.clients[userID] {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(hub.clients[userID], client)
		}
	}
}

func (hub *Hub) Broadcast(message wsOutgoingMessage) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	for _, clients := range hub.clients {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
			}
		}
	}
}