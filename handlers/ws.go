package handlers

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

const maxChatMessageLength = 1000

type Client struct {
	app  *App
	user currentUser
	conn *websocket.Conn
	send chan wsOutgoingMessage
}

type wsIncomingMessage struct {
	Type       string `json:"type"`
	ReceiverID int    `json:"receiver_id"`
	Content    string `json:"content"`
}

type wsOutgoingMessage struct {
	Type    string               `json:"type"`
	Message *chatMessageResponse `json:"message,omitempty"`
	UserID  int                  `json:"user_id,omitempty"`
	Online  bool                 `json:"online,omitempty"`
	Error   string               `json:"error,omitempty"`
}

var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (app *App) ChatWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		app:  app,
		user: user,
		conn: conn,
		send: make(chan wsOutgoingMessage, 16),
	}

	app.Hub.AddClient(user.ID, client)

	app.Hub.Broadcast(wsOutgoingMessage{
		Type:   "presence",
		UserID: user.ID,
		Online: true,
	})

	go client.writePump()
	client.readPump()
}

func (client *Client) readPump() {
	defer func() {
		client.app.Hub.RemoveClient(client.user.ID, client)

		client.app.Hub.Broadcast(wsOutgoingMessage{
			Type:   "presence",
			UserID: client.user.ID,
			Online: false,
		})

		close(client.send)
		client.conn.Close()
	}()

	client.conn.SetReadLimit(maxChatMessageLength + 500)

	for {
		var incoming wsIncomingMessage

		err := client.conn.ReadJSON(&incoming)
		if err != nil {
			return
		}

		client.handleIncomingMessage(incoming)
	}
}

func (client *Client) writePump() {
	defer client.conn.Close()

	for message := range client.send {
		err := client.conn.WriteJSON(message)
		if err != nil {
			return
		}
	}
}

func (client *Client) handleIncomingMessage(incoming wsIncomingMessage) {
	switch incoming.Type {
	case "private_message":
		client.handlePrivateMessage(incoming)
	default:
		client.sendError("unknown websocket message type")
	}
}

func (client *Client) handlePrivateMessage(incoming wsIncomingMessage) {
	content := strings.TrimSpace(incoming.Content)

	if incoming.ReceiverID <= 0 {
		client.sendError("receiver_id is required")
		return
	}

	if incoming.ReceiverID == client.user.ID {
		client.sendError("cannot send message to yourself")
		return
	}

	if content == "" {
		client.sendError("message content is required")
		return
	}

	if len(content) > maxChatMessageLength {
		client.sendError("message is too long")
		return
	}

	receiverExists, err := client.app.userExists(incoming.ReceiverID)
	if err != nil {
		client.sendError("failed to check receiver")
		return
	}

	if !receiverExists {
		client.sendError("receiver not found")
		return
	}

	message, err := client.app.createChatMessage(client.user, incoming.ReceiverID, content)
	if err != nil {
		client.sendError("failed to save message")
		return
	}

	outgoing := wsOutgoingMessage{
		Type:    "private_message",
		Message: &message,
	}

	client.app.Hub.SendToUser(client.user.ID, outgoing)
	client.app.Hub.SendToUser(incoming.ReceiverID, outgoing)
}

func (client *Client) sendError(message string) {
	client.send <- wsOutgoingMessage{
		Type:  "error",
		Error: message,
	}
}

func (app *App) createChatMessage(sender currentUser, receiverID int, content string) (chatMessageResponse, error) {
	result, err := app.DB.Exec(`
		INSERT INTO messages (sender_id, receiver_id, content)
		VALUES (?, ?, ?);
	`, sender.ID, receiverID, content)
	if err != nil {
		return chatMessageResponse{}, err
	}

	messageID, err := result.LastInsertId()
	if err != nil {
		return chatMessageResponse{}, err
	}

	var message chatMessageResponse

	err = app.DB.QueryRow(`
		SELECT
			messages.id,
			messages.sender_id,
			users.nickname,
			messages.receiver_id,
			messages.content,
			messages.created_at
		FROM messages
		INNER JOIN users ON users.id = messages.sender_id
		WHERE messages.id = ?;
	`, messageID).Scan(
		&message.ID,
		&message.SenderID,
		&message.SenderNickname,
		&message.ReceiverID,
		&message.Content,
		&message.CreatedAt,
	)
	if err != nil {
		return chatMessageResponse{}, err
	}

	return message, nil
}
