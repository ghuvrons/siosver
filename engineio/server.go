package engineio

import (
	"context"
	"net/http"
	"strconv"
)

// import (
// 	"context"
// 	"net/http"
// 	"strconv"

// 	"golang.org/x/net/websocket"
// )

type Server struct {
	options EngineIOOptions

	handlers struct {
		connection func(*Client)
	}
}

func NewServer(opt EngineIOOptions) (server *Server) {
	server = &Server{
		options: EngineIOOptions{
			PingInterval: opt.PingInterval,
			PingTimeout:  opt.PingTimeout,
		},
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

	client := newClient(server, sid)
	ctxWithClient := context.WithValue(req.Context(), CtxKeyClient, client)

	if transport == "websocket" {
		client.Transport = TRANSPORT_WEBSOCKET
	}

	switch transport {
	case "polling":
		ServePolling(w, req.WithContext(ctxWithClient))

	case "websocket":
		TransportWebsocketHandler.ServeHTTP(w, req.WithContext(ctxWithClient))
	}
}

func (server *Server) OnConnection(f func(*Client)) {
	server.handlers.connection = f
}
