package siosver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
	client := conn.Request().Context().Value(eioCtxKeyClient).(*engineIOClient)
	client.serveWebsocket(conn)
})

type Server struct {
	events        map[string]SocketIOEvent
	authenticator func(interface{}) bool
	Rooms         map[string]*Room // key: roomName
}

func NewServer(opt ServerOptions) (server *Server) {
	server = &Server{
		events: map[string]SocketIOEvent{},
		Rooms:  map[string]*Room{},
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

	client := newEngineIOClient(sid)
	if !client.isConnected {
		cHandler := &clientHandler{
			server:     svr,
			sioClients: map[string]*SocketIOClient{},
		}
		client.attr = cHandler
		client.onRecvPacket = onEngineIOClientRecvPacket
		client.onClosed = onEngineIOClientClosed

		if transport == "websocket" {
			client.transport = __TRANSPORT_WEBSOCKET
		}

		client.connect()
	}

	switch transport {
	case "polling":
		client.servePolling(w, req)

	case "websocket":
		ctxWithClient := context.WithValue(req.Context(), eioCtxKeyClient, client)
		wsHandler.ServeHTTP(w, req.WithContext(ctxWithClient))
		fmt.Println("websocket closed")
	}
}

func (svr *Server) Authenticator(f func(interface{}) bool) {
	svr.authenticator = f
}

func (svr *Server) On(event string, f SocketIOEvent) {
	if event == "message" {
		event = ""
	}
	if svr.events == nil {
		svr.events = map[string]SocketIOEvent{}
	}
	svr.events[event] = f
}

// Room methods
func (svr *Server) CreateRoom(roomName string) (room *Room) {
	room = &Room{
		Name:    roomName,
		clients: map[uuid.UUID]*SocketIOClient{},
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
