package ws

import (
	"log"
	"sync"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
	logger     *log.Logger
}

func NewHub(logger *log.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 1024),
		register:   make(chan *Client, 128),
		unregister: make(chan *Client, 128),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			if client == nil {
				continue
			}
			h.mutex.Lock()
			h.clients[client] = true
			total := len(h.clients)
			h.mutex.Unlock()
			if h.logger != nil {
				h.logger.Printf("WS connected | total_clients=%d", total)
			}

		case client := <-h.unregister:
			if client == nil {
				continue
			}
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			total := len(h.clients)
			h.mutex.Unlock()
			if h.logger != nil {
				h.logger.Printf("WS disconnected | total_clients=%d", total)
			}

		case message := <-h.broadcast:
			h.mutex.RLock()
			clientsSnapshot := make([]*Client, 0, len(h.clients))
			for c := range h.clients {
				clientsSnapshot = append(clientsSnapshot, c)
			}
			total := len(clientsSnapshot)
			h.mutex.RUnlock()

			for _, client := range clientsSnapshot {
				select {
				case client.send <- message:
				default:
					h.unregister <- client
				}
			}

			if h.logger != nil {
				h.logger.Printf("WS broadcast | clients=%d", total)
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

func (h *Hub) Broadcast(message []byte) {
	if h == nil {
		return
	}
	select {
	case h.broadcast <- message:
	default:
		if h.logger != nil {
			h.logger.Printf("WS broadcast dropped | reason=buffer_full")
		}
	}
}

func (h *Hub) ClientCount() int {
	if h == nil {
		return 0
	}
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}
