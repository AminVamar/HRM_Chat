package ws

import (
	"encoding/json"
	"log"
	"sync"
)

type Hub struct {
	clients     map[*Client]bool
	userClients map[int64]map[*Client]bool
	broadcast   chan BroadcastMessage
	register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
}

// BroadcastMessage — событие и кому его отправить.
// Пустой UserIDs значит "никому", а не "всем". Для всех нужно явно ставить Global.
type BroadcastMessage struct {
	UserIDs []int64
	Global  bool
	Event   WSEvent
}

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		userClients: make(map[int64]map[*Client]bool),
		broadcast:   make(chan BroadcastMessage, 1024),
		register:    make(chan *Client, 256),
		unregister:  make(chan *Client, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if _, ok := h.userClients[client.UserID]; !ok {
				h.userClients[client.UserID] = make(map[*Client]bool)
			}
			firstConnection := len(h.userClients[client.UserID]) == 0
			h.userClients[client.UserID][client] = true
			total := len(h.clients)
			h.mu.Unlock()

			log.Printf("WS Client registered: UserID=%d, Total clients=%d", client.UserID, total)

			if firstConnection {
				h.BroadcastPresence(client.UserID, true)
				h.BroadcastOnlineList()
			}

		case client := <-h.unregister:
			removed, wentOffline := h.removeClient(client)
			if removed && wentOffline {
				log.Printf("WS User offline: UserID=%d", client.UserID)
				h.BroadcastPresence(client.UserID, false)
				h.BroadcastOnlineList()
			}

		case message := <-h.broadcast:
			data, err := json.Marshal(message.Event)
			if err != nil {
				log.Printf("Error marshaling WS event %q: %v", message.Event.Type, err)
				continue
			}

			// Клиентов с переполненным буфером отключаем после снятия блокировки,
			// потому что removeClient сам берёт блокировку.
			var stalled []*Client

			h.mu.RLock()
			if message.Global {
				for client := range h.clients {
					if !client.trySend(data) {
						stalled = append(stalled, client)
					}
				}
			} else {
				for _, targetUserID := range message.UserIDs {
					for client := range h.userClients[targetUserID] {
						if !client.trySend(data) {
							stalled = append(stalled, client)
						}
					}
				}
			}
			h.mu.RUnlock()

			for _, client := range stalled {
				log.Printf("WS Client send buffer full, disconnecting: UserID=%d", client.UserID)
				h.removeClient(client)
			}
		}
	}
}

// removeClient удаляет клиента из хаба и закрывает соединение.
// Возвращает: был ли клиент зарегистрирован и было ли это последнее подключение пользователя.
func (h *Hub) removeClient(client *Client) (removed, wentOffline bool) {
	h.mu.Lock()
	if _, ok := h.clients[client]; !ok {
		h.mu.Unlock()
		return false, false
	}

	delete(h.clients, client)
	if userMap, ok := h.userClients[client.UserID]; ok {
		delete(userMap, client)
		if len(userMap) == 0 {
			delete(h.userClients, client.UserID)
			wentOffline = true
		}
	}
	h.mu.Unlock()

	client.Close()
	return true, wentOffline
}

func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	default:
		// Очередь заполнена — удаляем клиента напрямую, чтобы он не завис.
		go h.removeClient(client)
	}
}

func (h *Hub) IsUserOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	userMap, ok := h.userClients[userID]
	return ok && len(userMap) > 0
}

// BroadcastToUsers отправляет событие только указанным пользователям.
// Если список пустой — событие не отправляется и пишется в лог.
func (h *Hub) BroadcastToUsers(userIDs []int64, event WSEvent) {
	if len(userIDs) == 0 {
		log.Printf("Warning: WS event %q has no recipients, dropped", event.Type)
		return
	}
	h.enqueue(BroadcastMessage{UserIDs: userIDs, Event: event})
}

// BroadcastToAll отправляет событие всем подключённым клиентам.
// Только для общих событий, например статус онлайн.
func (h *Hub) BroadcastToAll(event WSEvent) {
	h.enqueue(BroadcastMessage{Global: true, Event: event})
}

func (h *Hub) enqueue(msg BroadcastMessage) {
	select {
	case h.broadcast <- msg:
	default:
		log.Printf("Warning: WS broadcast buffer full, event dropped: %s", msg.Event.Type)
	}
}

func (h *Hub) BroadcastPresence(userID int64, isOnline bool) {
	payload, err := json.Marshal(UserPresencePayload{
		UserID:   userID,
		IsOnline: isOnline,
	})
	if err != nil {
		log.Printf("Error marshaling presence payload: %v", err)
		return
	}
	h.BroadcastToAll(WSEvent{
		Type:    EventUserPresence,
		Payload: payload,
	})
}

func (h *Hub) OnlineUsers() []OnlineUserDTO {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make([]OnlineUserDTO, 0, len(h.userClients))
	for uid, clients := range h.userClients {
		for c := range clients {
			users = append(users, OnlineUserDTO{UserID: uid, Username: c.User.Username})
			break
		}
	}
	return users
}

func (h *Hub) BroadcastOnlineList() {
	users := h.OnlineUsers()
	payload, err := json.Marshal(OnlineUsersPayload{Count: len(users), Users: users})
	if err != nil {
		log.Printf("Error marshaling online users payload: %v", err)
		return
	}
	h.BroadcastToAll(WSEvent{
		Type:    EventOnlineUsers,
		Payload: payload,
	})
}
