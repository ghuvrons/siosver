package siosver

import (
	"context"
	"net/http"
	"strconv"

	"github.com/ghuvrons/siosver/engineio"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

type ServerOptions struct {
	PingTimeout  int
	PingInterval int
}

var serverOptions = &ServerOptions{
	PingTimeout:  5000,
	PingInterval: 25000,
}

var wsHandler = websocket.Handler(func(conn *websocket.Conn) {
	client := conn.Request().Context().Value(engineio.CtxKeyClient).(*engineio.Client)
	client.ServeWebsocket(conn)
})

type Server struct {
	Sockets       Sockets
	events        map[string]EventHandler
	Rooms         map[string]*Room // key: roomName
	authenticator func(interface{}) bool
}

func NewServer(opt ServerOptions) (server *Server) {
	server = &Server{
		Sockets: Sockets{},
		events:  map[string]EventHandler{},
		Rooms:   map[string]*Room{},
	}
	server.Setup(opt)
	return
}

func (svr *Server) Setup(opt ServerOptions) {
	if opt.PingInterval != 0 {
		serverOptions.PingInterval = opt.PingInterval
	}
	if opt.PingTimeout != 0 {
		serverOptions.PingTimeout = opt.PingTimeout
	}
}

func (svr *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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

	client := engineio.NewClient(sid, engineio.EngineIOOptions{
		PingInterval: serverOptions.PingInterval,
		PingTimeout:  serverOptions.PingTimeout,
	})
	if !client.IsConnected {
		client.Attr = newManager(svr)
		client.OnRecvPacket = onEngineIOClientRecvPacket
		client.OnClosed = onEngineIOClientClosed

		if transport == "websocket" {
			client.Transport = engineio.TRANSPORT_WEBSOCKET
		}

		client.Connect()
	}

	switch transport {
	case "polling":
		client.ServePolling(w, req)

	case "websocket":
		ctxWithClient := context.WithValue(req.Context(), engineio.CtxKeyClient, client)
		wsHandler.ServeHTTP(w, req.WithContext(ctxWithClient))
	}
}

func (svr *Server) Authenticator(f func(interface{}) bool) {
	svr.authenticator = f
}

func (svr *Server) On(event string, f EventHandler) {
	if event == "message" {
		event = ""
	}
	if svr.events == nil {
		svr.events = map[string]EventHandler{}
	}
	svr.events[event] = f
}

// Room methods
func (svr *Server) CreateRoom(roomName string) (room *Room) {
	room = &Room{
		Name:    roomName,
		sockets: map[uuid.UUID]*Socket{},
	}

	if svr.Rooms == nil {
		svr.Rooms = map[string]*Room{}
	}

	svr.Rooms[roomName] = room
	return
}

func (svr *Server) DeleteRoom(roomName string) {
	delete(svr.Rooms, roomName)
}
