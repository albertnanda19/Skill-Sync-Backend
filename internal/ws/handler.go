package ws

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gorilla/websocket"
)

type Handler struct {
	hub    *Hub
	logger *log.Logger
}

func NewHandler(hub *Hub, logger *log.Logger) *Handler {
	return &Handler{hub: hub, logger: logger}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) HandleJobsWS(c fiber.Ctx) error {
	if h == nil || h.hub == nil {
		return fiber.ErrServiceUnavailable
	}

	fiberHandler := adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if h.logger != nil {
				h.logger.Printf("WS upgrade error | error=%v", err)
			}
			return
		}

		client := NewClient(h.hub, conn)
		h.hub.Register(client)
		go client.WritePump()
		go client.ReadPump()
	})

	return fiberHandler(c)
}
