package ws

import (
	"log"
	"sync"
)

type ScopedMessage struct {
	Keyword string
	Payload []byte
}

type Hub struct {
	clientsByKeyword map[string]map[*Client]bool
	broadcast        chan ScopedMessage
	register         chan *Client
	unregister       chan *Client
	mutex            sync.RWMutex
	logger           *log.Logger
}

func NewHub(logger *log.Logger) *Hub {
	return &Hub{
		clientsByKeyword: make(map[string]map[*Client]bool),
		broadcast:        make(chan ScopedMessage, 1024),
		register:         make(chan *Client, 128),
		unregister:       make(chan *Client, 128),
		logger:           logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			if client == nil {
				continue
			}
			keyword := client.keyword
			h.mutex.Lock()
			if _, ok := h.clientsByKeyword[keyword]; !ok {
				h.clientsByKeyword[keyword] = make(map[*Client]bool)
			}
			h.clientsByKeyword[keyword][client] = true
			totalForKeyword := len(h.clientsByKeyword[keyword])
			h.mutex.Unlock()
			if h.logger != nil {
				h.logger.Printf("WS connected | keyword=%s total_for_keyword=%d", keyword, totalForKeyword)
			}

		case client := <-h.unregister:
			if client == nil {
				continue
			}
			keyword := client.keyword
			remaining := 0
			h.mutex.Lock()
			if group, ok := h.clientsByKeyword[keyword]; ok {
				if _, ok := group[client]; ok {
					delete(group, client)
					close(client.send)
				}
				remaining = len(group)
				if remaining == 0 {
					delete(h.clientsByKeyword, keyword)
				}
			}
			h.mutex.Unlock()
			if h.logger != nil {
				h.logger.Printf("WS disconnected | keyword=%s remaining=%d", keyword, remaining)
			}

		case msg := <-h.broadcast:
			if msg.Keyword == "" {
				continue
			}
			h.mutex.RLock()
			group := h.clientsByKeyword[msg.Keyword]
			clientsSnapshot := make([]*Client, 0, len(group))
			for c := range group {
				clientsSnapshot = append(clientsSnapshot, c)
			}
			total := len(clientsSnapshot)
			h.mutex.RUnlock()

			for _, client := range clientsSnapshot {
				select {
				case client.send <- msg.Payload:
				default:
					h.unregister <- client
				}
			}

			if h.logger != nil {
				h.logger.Printf("WS broadcast | keyword=%s clients=%d", msg.Keyword, total)
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	if h == nil {
		return
	}
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	if h == nil {
		return
	}
	h.unregister <- client
}

func (h *Hub) Broadcast(keyword string, message []byte) {
	if h == nil {
		return
	}
	if keyword == "" {
		return
	}
	select {
	case h.broadcast <- ScopedMessage{Keyword: keyword, Payload: message}:
	default:
		if h.logger != nil {
			h.logger.Printf("WS broadcast dropped | keyword=%s reason=buffer_full", keyword)
		}
	}
}

func (h *Hub) ClientCount(keyword string) int {
	if h == nil {
		return 0
	}
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clientsByKeyword[keyword])
}
