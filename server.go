package siosver

import (
	"context"
	"net/http"
	"strconv"

	"golang.org/x/net/websocket"
)

type ServerOptions struct {
	pingTimeout  int
	pingInterval int
}

var serverOptions = &ServerOptions{
	pingTimeout:  5000,
	pingInterval: 25000,
}

var wsHandler = websocket.Handler(func(conn *websocket.Conn) {
	client := conn.Request().Context().Value(eioCtxKeyClient).(*engineIOClient)
	client.serveWebsocket(conn)
})

type Handler struct {
	events        map[string]SocketIOEvent
	authenticator func(interface{}) bool
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	version := req.URL.Query().Get("EIO")
	v, err := strconv.Atoi(version)
	if err != nil {
		w.Write([]byte("Get version error"))
		return
	}
	if err != nil || v != 4 {
		w.Write([]byte("Protocol version is not support"))
		return
	}

	sid := req.URL.Query().Get("sid")
	transport := req.URL.Query().Get("transport")

	client := newEngineIOClient(sid)
	if !client.isConnected {
		client.attr = &socketIOHandler{
			events:        h.events,
			authenticator: h.authenticator,
		}
		client.onConnected = onEngineIOClientConnected

		if transport == "websocket" {
			client.transport = __TRANSPORT_WEBSOCKET
		}

		client.connect()

		if client.onConnected != nil {
			client.onConnected(client)
		}
	}

	switch transport {
	case "polling":
		client.servePolling(w, req)

	case "websocket":
		ctxWithClient := context.WithValue(req.Context(), eioCtxKeyClient, client)
		wsHandler.ServeHTTP(w, req.WithContext(ctxWithClient))
	}
}

func (h *Handler) Authenticator(f func(interface{}) bool) {
	h.authenticator = f
}

func (h *Handler) On(event string, f SocketIOEvent) {
	if event == "message" {
		event = ""
	}
	if h.events == nil {
		h.events = map[string]SocketIOEvent{}
	}
	h.events[event] = f
}
