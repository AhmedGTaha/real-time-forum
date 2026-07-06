package handlers

type Hub struct {
	register   chan *Client
	unregister chan *Client
	sendToUser chan hubUserMessage
	isOnline   chan hubOnlineRequest
}

type hubUserMessage struct {
	UserID  int
	Message wsOutgoingMessage
}

type hubOnlineRequest struct {
	UserID   int
	Response chan bool
}

func NewHub() *Hub {
	hub := &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		sendToUser: make(chan hubUserMessage),
		isOnline:   make(chan hubOnlineRequest),
	}

	go hub.run()

	return hub
}

func (hub *Hub) run() {
	clients := make(map[int]map[*Client]bool)

	for {
		select {
		case client := <-hub.register:
			userID := client.user.ID
			wasOffline := len(clients[userID]) == 0

			if clients[userID] == nil {
				clients[userID] = make(map[*Client]bool)
			}

			clients[userID][client] = true

			if wasOffline {
				broadcastToAll(clients, wsOutgoingMessage{
					Type:   "presence",
					UserID: userID,
					Online: true,
				})
			}

		case client := <-hub.unregister:
			userID := client.user.ID

			if clients[userID] == nil {
				continue
			}

			delete(clients[userID], client)

			if len(clients[userID]) == 0 {
				delete(clients, userID)

				broadcastToAll(clients, wsOutgoingMessage{
					Type:   "presence",
					UserID: userID,
					Online: false,
				})
			}

		case outgoing := <-hub.sendToUser:
			for client := range clients[outgoing.UserID] {
				select {
				case client.send <- outgoing.Message:
				default:
					// If the browser is too slow to receive messages, skip this send.
					// The connection cleanup will happen when the socket closes.
				}
			}

		case request := <-hub.isOnline:
			request.Response <- len(clients[request.UserID]) > 0
		}
	}
}

func (hub *Hub) AddClient(client *Client) {
	hub.register <- client
}

func (hub *Hub) RemoveClient(client *Client) {
	hub.unregister <- client
}

func (hub *Hub) IsOnline(userID int) bool {
	response := make(chan bool)

	hub.isOnline <- hubOnlineRequest{
		UserID:   userID,
		Response: response,
	}

	return <-response
}

func (hub *Hub) SendToUser(userID int, message wsOutgoingMessage) {
	hub.sendToUser <- hubUserMessage{
		UserID:  userID,
		Message: message,
	}
}

func broadcastToAll(clients map[int]map[*Client]bool, message wsOutgoingMessage) {
	for _, userClients := range clients {
		for client := range userClients {
			select {
			case client.send <- message:
			default:
				// Skip slow clients.
			}
		}
	}
}
