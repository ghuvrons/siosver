package siosver

import (
	"net/http"
	"strconv"
)

type Handler struct {
	events        map[string]func(*SocketIOClient, []interface{})
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
}

func (h *Handler) Authenticator(f func(interface{}) bool) {
	h.authenticator = f
}

func (h *Handler) On(event string, f func(*SocketIOClient, []interface{})) {
	if h.events == nil {
		h.events = map[string]func(*SocketIOClient, []interface{}){}
	}
	h.events[event] = f
}
