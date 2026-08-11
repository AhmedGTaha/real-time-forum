package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisEventsChannel    = "realtime-forum:events"
	presenceKeyPrefix     = "realtime-forum:presence:"
	presenceRefreshPeriod = 15 * time.Second
	presenceStaleAfter    = 45 * time.Second
	presenceKeyTTL        = 90 * time.Second
	redisOperationTimeout = 5 * time.Second
)

var checkPresenceScript = redis.NewScript(`
local removed = redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
local count = redis.call("ZCARD", KEYS[1])
if count == 0 then
  redis.call("DEL", KEYS[1])
  if removed > 0 then
    redis.call("PUBLISH", ARGV[2], ARGV[3])
  end
end
return count
`)

var removePresenceScript = redis.NewScript(`
redis.call("ZREM", KEYS[1], ARGV[1])
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[2])
local count = redis.call("ZCARD", KEYS[1])
if count == 0 then
  redis.call("DEL", KEYS[1])
  redis.call("PUBLISH", ARGV[3], ARGV[4])
end
return count
`)

type Hub struct {
	redis       *redis.Client
	pubsub      *redis.PubSub
	register    chan localClientRequest
	unregister  chan localClientRequest
	redisEvents chan wsOutgoingMessage
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
	wg          sync.WaitGroup
}

type localClientRequest struct {
	client *Client
	done   chan struct{}
}

func NewHub() (*Hub, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, errors.New("REDIS_URL environment variable is required")
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}

	redisClient := redis.NewClient(options)
	startupContext, startupCancel := context.WithTimeout(context.Background(), redisOperationTimeout)
	defer startupCancel()

	if err := redisClient.Ping(startupContext).Err(); err != nil {
		redisClient.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	pubsub := redisClient.Subscribe(startupContext, redisEventsChannel)
	if _, err := pubsub.Receive(startupContext); err != nil {
		pubsub.Close()
		redisClient.Close()
		return nil, fmt.Errorf("subscribe to Redis events: %w", err)
	}

	hubContext, hubCancel := context.WithCancel(context.Background())
	hub := &Hub{
		redis:       redisClient,
		pubsub:      pubsub,
		register:    make(chan localClientRequest),
		unregister:  make(chan localClientRequest),
		redisEvents: make(chan wsOutgoingMessage, 64),
		ctx:         hubContext,
		cancel:      hubCancel,
	}

	hub.wg.Add(2)
	go hub.run()
	go hub.receiveRedisEvents()

	return hub, nil
}

func (hub *Hub) run() {
	defer hub.wg.Done()

	clients := make(map[int]map[*Client]bool)

	for {
		select {
		case request := <-hub.register:
			client := request.client
			userID := client.user.ID

			if clients[userID] == nil {
				clients[userID] = make(map[*Client]bool)
			}

			clients[userID][client] = true
			close(request.done)

		case request := <-hub.unregister:
			client := request.client
			userID := client.user.ID

			if clients[userID] != nil {
				delete(clients[userID], client)
				if len(clients[userID]) == 0 {
					delete(clients, userID)
				}
			}

			close(request.done)

		case event := <-hub.redisEvents:
			hub.forwardRedisEvent(clients, event)

		case <-hub.ctx.Done():
			for _, userClients := range clients {
				for client := range userClients {
					client.conn.Close()
				}
			}
			return
		}
	}
}

func (hub *Hub) receiveRedisEvents() {
	defer hub.wg.Done()

	for {
		message, err := hub.pubsub.ReceiveMessage(hub.ctx)
		if err != nil {
			if hub.ctx.Err() != nil {
				return
			}

			log.Printf("receive Redis event: %v", err)
			select {
			case <-time.After(time.Second):
			case <-hub.ctx.Done():
				return
			}
			continue
		}

		var event wsOutgoingMessage
		if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
			log.Printf("decode Redis event: %v", err)
			continue
		}

		if !validRedisEvent(event) {
			continue
		}

		select {
		case hub.redisEvents <- event:
		case <-hub.ctx.Done():
			return
		}
	}
}

func validRedisEvent(event wsOutgoingMessage) bool {
	switch event.Type {
	case "presence":
		return event.UserID > 0
	case "private_message":
		return event.Message != nil
	default:
		return false
	}
}

func (hub *Hub) forwardRedisEvent(clients map[int]map[*Client]bool, event wsOutgoingMessage) {
	switch event.Type {
	case "presence":
		broadcastToAll(clients, event)
	case "private_message":
		message := event.Message
		forwardToLocalUser(clients, message.SenderID, event)
		if message.ReceiverID != message.SenderID {
			forwardToLocalUser(clients, message.ReceiverID, event)
		}
	}
}

func (hub *Hub) AddClient(client *Client) error {
	if err := hub.refreshPresence(hub.ctx, client.user.ID, client.connectionID); err != nil {
		return fmt.Errorf("register Redis presence: %w", err)
	}

	request := localClientRequest{client: client, done: make(chan struct{})}
	if !hub.sendLocalRequest(hub.register, request) {
		hub.removePresence(client.user.ID, client.connectionID)
		return errors.New("websocket hub is closed")
	}

	onlineEvent := wsOutgoingMessage{
		Type:   "presence",
		UserID: client.user.ID,
		Online: true,
	}
	if err := hub.Publish(onlineEvent); err != nil {
		hub.sendLocalRequest(hub.unregister, localClientRequest{client: client, done: make(chan struct{})})
		hub.removePresence(client.user.ID, client.connectionID)
		return fmt.Errorf("publish online presence: %w", err)
	}

	client.presenceCancel, client.presenceDone = hub.startPresenceHeartbeat(client)
	return nil
}

func (hub *Hub) RemoveClient(client *Client) {
	if client.presenceCancel != nil {
		client.presenceCancel()
		<-client.presenceDone
	}

	hub.sendLocalRequest(hub.unregister, localClientRequest{client: client, done: make(chan struct{})})

	if err := hub.removePresence(client.user.ID, client.connectionID); err != nil && hub.ctx.Err() == nil {
		log.Printf("remove Redis presence for user %d: %v", client.user.ID, err)
	}
}

func (hub *Hub) sendLocalRequest(channel chan localClientRequest, request localClientRequest) bool {
	select {
	case channel <- request:
	case <-hub.ctx.Done():
		return false
	}

	select {
	case <-request.done:
		return true
	case <-hub.ctx.Done():
		return false
	}
}

func (hub *Hub) startPresenceHeartbeat(client *Client) (context.CancelFunc, <-chan struct{}) {
	heartbeatContext, heartbeatCancel := context.WithCancel(hub.ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(presenceRefreshPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := hub.refreshPresence(heartbeatContext, client.user.ID, client.connectionID); err != nil && heartbeatContext.Err() == nil {
					log.Printf("refresh Redis presence for user %d: %v", client.user.ID, err)
				}
			case <-heartbeatContext.Done():
				return
			}
		}
	}()

	return heartbeatCancel, done
}

func (hub *Hub) refreshPresence(parent context.Context, userID int, connectionID string) error {
	ctx, cancel := context.WithTimeout(parent, redisOperationTimeout)
	defer cancel()

	key := presenceKey(userID)
	pipeline := hub.redis.TxPipeline()
	pipeline.ZAdd(ctx, key, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: connectionID,
	})
	pipeline.Expire(ctx, key, presenceKeyTTL)
	_, err := pipeline.Exec(ctx)
	return err
}

func (hub *Hub) removePresence(userID int, connectionID string) error {
	ctx, cancel := context.WithTimeout(hub.ctx, redisOperationTimeout)
	defer cancel()

	offlineEvent, err := json.Marshal(wsOutgoingMessage{
		Type:   "presence",
		UserID: userID,
		Online: false,
	})
	if err != nil {
		return err
	}

	return removePresenceScript.Run(
		ctx,
		hub.redis,
		[]string{presenceKey(userID)},
		connectionID,
		stalePresenceCutoff(),
		redisEventsChannel,
		string(offlineEvent),
	).Err()
}

func (hub *Hub) IsOnline(userID int) bool {
	ctx, cancel := context.WithTimeout(hub.ctx, redisOperationTimeout)
	defer cancel()

	offlineEvent, err := json.Marshal(wsOutgoingMessage{
		Type:   "presence",
		UserID: userID,
		Online: false,
	})
	if err != nil {
		return false
	}

	count, err := checkPresenceScript.Run(
		ctx,
		hub.redis,
		[]string{presenceKey(userID)},
		stalePresenceCutoff(),
		redisEventsChannel,
		string(offlineEvent),
	).Int64()
	if err != nil {
		if hub.ctx.Err() == nil {
			log.Printf("check Redis presence for user %d: %v", userID, err)
		}
		return false
	}

	return count > 0
}

func (hub *Hub) Publish(event wsOutgoingMessage) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode Redis event: %w", err)
	}

	ctx, cancel := context.WithTimeout(hub.ctx, redisOperationTimeout)
	defer cancel()

	if err := hub.redis.Publish(ctx, redisEventsChannel, payload).Err(); err != nil {
		return fmt.Errorf("publish Redis event: %w", err)
	}

	return nil
}

func (hub *Hub) Close() error {
	var closeErr error

	hub.closeOnce.Do(func() {
		hub.cancel()
		if err := hub.pubsub.Close(); err != nil {
			closeErr = err
		}
		hub.wg.Wait()
		if err := hub.redis.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})

	return closeErr
}

func presenceKey(userID int) string {
	return presenceKeyPrefix + strconv.Itoa(userID)
}

func stalePresenceCutoff() int64 {
	return time.Now().Add(-presenceStaleAfter).Unix()
}

func forwardToLocalUser(clients map[int]map[*Client]bool, userID int, message wsOutgoingMessage) {
	for client := range clients[userID] {
		select {
		case client.send <- message:
		default:
			// Skip slow clients. Message history remains available from Turso.
		}
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
