package websocket

import (
	"sync"
	"time"

	fiberws "github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
)

// Hub menyimpan semua browser yang sedang membuka dashboard dan meneruskan data ke mereka.
type Hub struct {
	mu      sync.Mutex
	clients map[*fiberws.Conn]struct{}
}

// H adalah hub tunggal yang dipakai seluruh aplikasi.
var H = &Hub{clients: map[*fiberws.Conn]struct{}{}}

// message adalah format pesan ke browser: {"event":"sensor","data":{...}}
type message struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// Handler dipasang di route /ws.
func (h *Hub) Handler() fiber.Handler {
	return fiberws.New(func(conn *fiberws.Conn) {
		h.add(conn)
		defer func() {
			h.remove(conn)
			_ = conn.Close()
		}()

		// Browser tidak mengirim apa-apa. Loop ini hanya untuk mendeteksi koneksi yang putus.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
}

// Broadcast mengirim satu event ke semua browser yang terhubung.
func (h *Hub) Broadcast(event string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	msg := message{Event: event, Data: data}
	for conn := range h.clients {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteJSON(msg); err != nil {
			_ = conn.Close()
			delete(h.clients, conn)
		}
	}
}

func (h *Hub) add(conn *fiberws.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(conn *fiberws.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}
