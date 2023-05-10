package siosver

import (
	"net/http"
	"sync"

	"github.com/ghuvrons/siosver/engineio"
	"github.com/google/uuid"
)

type ServerOptions struct {
	PingTimeout  int
	PingInterval int
}

type Server struct {
	engineio      *engineio.Server
	Sockets       Sockets
	socketsMtx    *sync.Mutex
	events        map[string]EventHandler
	Rooms         map[string]*Room // key: roomName
	authenticator func(interface{}) bool
}

func NewServer(opt ServerOptions) (server *Server) {
	eioOptions := engineio.EngineIOOptions{
		PingInterval: opt.PingInterval,
		PingTimeout:  opt.PingTimeout,
	}

	server = &Server{
		engineio:   engineio.NewServer(eioOptions),
		Sockets:    Sockets{},
		socketsMtx: &sync.Mutex{},
		events:     map[string]EventHandler{},
		Rooms:      map[string]*Room{},
	}

	server.engineio.OnConnection(func(c *engineio.Socket) {
		c.Attr = newManager(server)
		c.OnMessage(onEngineIOSocketRecvPacket)
		c.OnClosed(onEngineIOSocketClosed)
	})

	return
}

func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.engineio.ServeHTTP(w, req)
}

func (server *Server) Authenticator(f func(interface{}) bool) {
	server.authenticator = f
}

func (server *Server) On(event string, f EventHandler) {
	if event == "message" {
		event = ""
	}
	if server.events == nil {
		server.events = map[string]EventHandler{}
	}
	server.events[event] = f
}

// Room methods
func (server *Server) CreateRoom(roomName string) (room *Room) {
	room = &Room{
		Name:    roomName,
		sockets: map[uuid.UUID]*Socket{},
	}

	if server.Rooms == nil {
		server.Rooms = map[string]*Room{}
	}

	server.Rooms[roomName] = room
	return
}

func (server *Server) DeleteRoom(roomName string) {
	delete(server.Rooms, roomName)
}
