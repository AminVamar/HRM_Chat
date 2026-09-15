package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 10 * time.Second
	pingPeriod     = 4 * time.Second
	maxMessageSize = 512 * 1024 
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	Hub            *Hub
	Conn           *websocket.Conn
	send           chan []byte
	UserID         int64
	User           *domain.User
	messageUseCase usecase.MessageUseCase
	chatUseCase    usecase.ChatUseCase
	notifier       *Notifier

	done      chan struct{}
	closeOnce sync.Once
}

func NewClient(
	hub *Hub,
	conn *websocket.Conn,
	user *domain.User,
	messageUseCase usecase.MessageUseCase,
	chatUseCase usecase.ChatUseCase,
	notifier *Notifier,
) *Client {
	return &Client{
		Hub:            hub,
		Conn:           conn,
		send:           make(chan []byte, 256),
		UserID:         user.ID,
		User:           user,
		messageUseCase: messageUseCase,
		chatUseCase:    chatUseCase,
		notifier:       notifier,
		done:           make(chan struct{}),
	}
}

func (c *Client) SendOnlineSnapshot() {
	users := c.Hub.OnlineUsers()
	payload, err := json.Marshal(OnlineUsersPayload{Count: len(users), Users: users})
	if err != nil {
		log.Printf("Error marshaling online snapshot: %v", err)
		return
	}
	c.sendEvent(WSEvent{Type: EventOnlineUsers, Payload: payload})
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.Conn.Close()
	})
}

func (c *Client) trySend(data []byte) bool {
	select {
	case <-c.done:
		return true  
	default:
	}

	select {
	case c.send <- data:
		return true
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageData, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS Error reading message from user %d: %v", c.UserID, err)
			}
			break
		}

		var event WSEvent
		if err := json.Unmarshal(messageData, &event); err != nil {
			c.sendError("Invalid JSON event format", 400)
			continue
		}

		c.handleEvent(event)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Hub.Unregister(c)
	}()

	for {
		select {
		case <-c.done:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case message := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleEvent(event WSEvent) {
	ctx := context.Background()

	switch event.Type {
	case EventPing:
		pongPayload, _ := json.Marshal(map[string]string{"message": "pong"})
		c.sendEvent(WSEvent{Type: EventPong, Payload: pongPayload})

	case EventSendMessage:
		var p SendMessagePayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid send_message payload", 400)
			return
		}

		msg, err := c.messageUseCase.SendMessage(ctx, c.UserID, domain.CreateMessageDTO{
			ChatID:  p.ChatID,
			Content: p.Content,
		})
		if err != nil {
			c.sendError(err.Error(), 400)
			return
		}

		if err := c.notifier.NewMessage(ctx, msg); err != nil {
			c.sendError("Message saved but could not be delivered: "+err.Error(), 500)
		}

	case EventEditMessage:
		var p EditMessagePayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid edit_message payload", 400)
			return
		}

		msg, err := c.messageUseCase.UpdateMessage(ctx, c.UserID, p.MessageID, domain.UpdateMessageDTO{
			Content: p.Content,
		})
		if err != nil {
			c.sendError(err.Error(), 400)
			return
		}

		if err := c.notifier.MessageUpdated(ctx, msg); err != nil {
			c.sendError("Message updated but could not be delivered: "+err.Error(), 500)
		}

	case EventDeleteMessage:
		var p DeleteMessagePayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid delete_message payload", 400)
			return
		}

		chatID, err := c.messageUseCase.DeleteMessage(ctx, c.UserID, p.MessageID)
		if err != nil {
			c.sendError(err.Error(), 400)
			return
		}

		if err := c.notifier.MessageDeleted(ctx, chatID, p.MessageID); err != nil {
			c.sendError("Message deleted but could not be delivered: "+err.Error(), 500)
		}

	case EventAddReaction:
		var p AddReactionPayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid add_reaction payload", 400)
			return
		}

		_, chatID, err := c.messageUseCase.AddReaction(ctx, c.UserID, p.MessageID, p.Reaction)
		if err != nil {
			c.sendError(err.Error(), 400)
			return
		}

		if err := c.notifier.ReactionUpdated(ctx, chatID, p.MessageID, c.UserID, p.Reaction, ReactionAdded); err != nil {
			c.sendError("Reaction saved but could not be delivered: "+err.Error(), 500)
		}

	case EventRemoveReaction:
		var p RemoveReactionPayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid remove_reaction payload", 400)
			return
		}

		chatID, err := c.messageUseCase.RemoveReaction(ctx, c.UserID, p.MessageID, p.Reaction)
		if err != nil {
			c.sendError(err.Error(), 400)
			return
		}

		if err := c.notifier.ReactionUpdated(ctx, chatID, p.MessageID, c.UserID, p.Reaction, ReactionRemoved); err != nil {
			c.sendError("Reaction removed but could not be delivered: "+err.Error(), 500)
		}

	case EventTyping:
		var p TypingPayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid typing payload", 400)
			return
		}

		if err := c.notifier.Typing(ctx, p.ChatID, c.UserID, p.IsTyping); err != nil {
			c.sendError(err.Error(), 400)
		}

	case EventReadReceipt:
		var p ReadReceiptPayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			c.sendError("Invalid read_receipt payload", 400)
			return
		}

		if err := c.chatUseCase.MarkAsRead(ctx, c.UserID, p.ChatID, p.LastReadMessageID); err != nil {
			c.sendError(err.Error(), 400)
			return
		}

		if err := c.notifier.ReadReceipt(ctx, p.ChatID, c.UserID, p.LastReadMessageID); err != nil {
			c.sendError("Read receipt saved but could not be delivered: "+err.Error(), 500)
		}

	default:
		c.sendError("Unknown WS event type: "+event.Type, 400)
	}
}

func (c *Client) sendEvent(event WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling WS event %q for user %d: %v", event.Type, c.UserID, err)
		return
	}
	c.trySend(data)
}

func (c *Client) sendError(msg string, code int) {
	payload, err := json.Marshal(ErrorPayload{
		Message: msg,
		Code:    code,
	})
	if err != nil {
		return
	}
	c.sendEvent(WSEvent{
		Type:    EventError,
		Payload: payload,
	})
}
