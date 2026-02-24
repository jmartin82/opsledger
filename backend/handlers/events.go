package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// SSEEvent is the envelope published to the hub.
type SSEEvent struct {
	Type string      // "change.created" | "change.updated" | "change.deleted"
	Data interface{} // models.Change or DeletedPayload
}

// DeletedPayload carries a string ID to match the frontend Change.id type.
type DeletedPayload struct {
	ID string `json:"id"`
}

// EventHub is an in-memory pub/sub hub for SSE clients.
type EventHub struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

// NewEventHub creates a ready-to-use EventHub.
func NewEventHub() *EventHub {
	return &EventHub{
		clients: make(map[chan SSEEvent]struct{}),
	}
}

// Subscribe registers a new client channel and returns it.
func (h *EventHub) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel from the hub.
func (h *EventHub) Unsubscribe(ch chan SSEEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

// Publish sends an event to all connected clients.
// Uses a non-blocking send so a slow client never blocks the publisher.
func (h *EventHub) Publish(event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			// Buffer full — drop event for this slow client
		}
	}
}

// EventHandler handles the GET /api/events SSE endpoint.
type EventHandler struct {
	Hub *EventHub
}

// Stream opens an SSE stream for authenticated clients.
func (h *EventHandler) Stream(c echo.Context) error {
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch := h.Hub.Subscribe()
	defer h.Hub.Unsubscribe(ch)

	flusher, ok := w.Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := c.Request().Context()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event := <-ch:
			data, err := json.Marshal(event.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()

		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
