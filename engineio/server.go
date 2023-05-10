package engineio

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"github.com/google/uuid"
)

type Server struct {
	options EngineIOOptions

	sockets    map[uuid.UUID]*Socket
	socketsMtx *sync.Mutex

	handlers struct {
		connection func(*Socket)
	}
}

func NewServer(opt EngineIOOptions) (server *Server) {
	server = &Server{
		options: EngineIOOptions{
			PingInterval: opt.PingInterval,
			PingTimeout:  opt.PingTimeout,
		},
		sockets:    map[uuid.UUID]*Socket{},
		socketsMtx: &sync.Mutex{},
	}

	return server
}

func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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

	socket := newSocket(server, sid)
	ctxWithSocket := context.WithValue(req.Context(), ctxKeySocket, socket)

	if transport == "websocket" {
		socket.Transport = TRANSPORT_WEBSOCKET
	}

	switch transport {
	case "polling":
		ServePolling(w, req.WithContext(ctxWithSocket))

	case "websocket":
		TransportWebsocketHandler.ServeHTTP(w, req.WithContext(ctxWithSocket))
	}
}

func (server *Server) OnConnection(f func(*Socket)) {
	server.handlers.connection = f
}
