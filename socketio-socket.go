package siosver

import (
	"github.com/ghuvrons/siosver/engineio"
	"github.com/google/uuid"
)

type Socket struct {
	id        uuid.UUID
	server    *Server
	eioClient *engineio.Client
	namespace string
	tmpPacket *socketIOPacket
	rooms     map[string]*Room // key: roomName
}

type Sockets map[uuid.UUID]*Socket

func newSocket(namespace string) *Socket {
	var c = &Socket{
		id:        uuid.New(),
		namespace: namespace,
		rooms:     map[string]*Room{},
	}
	return c
}

// Handle client's connect request
func (socket *Socket) connect(conpacket *socketIOPacket) {
	// do authenticating ...
	var data interface{}

	if conpacket.data != nil {
		data = conpacket.data
	}
	manager, isOk := socket.eioClient.Attr.(*Manager)
	if isOk && manager.server.authenticator != nil {
		if !manager.server.authenticator(data) {
			errConnData := map[string]interface{}{
				"message": "Not authorized",
				"data": map[string]interface{}{
					"code":  "E001",
					"label": "Invalid credentials",
				},
			}
			socket.send(newSocketIOPacket(__SIO_PACKET_CONNECT_ERROR, errConnData))
			return
		}
	}

	// if success
	socket.send(newSocketIOPacket(__SIO_PACKET_CONNECT, map[string]interface{}{"sid": socket.id.String()}))

	eventFunc, isEventFound := manager.server.events["connection"]
	if isEventFound && eventFunc != nil {
		eventFunc(socket)
	}

	socket.server.socketsMtx.Lock()
	socket.server.Sockets[socket.id] = socket
	socket.server.socketsMtx.Unlock()
}

func (socket *Socket) send(packet *socketIOPacket) {
	packet.namespace = socket.namespace
	encodedPacket, buffers := packet.encode()
	eioPacket := engineio.NewPacket(engineio.PACKET_MESSAGE, encodedPacket)

	if len(buffers) == 0 {
		socket.eioClient.Send(eioPacket)

	} else {
		// binary message
		if err := socket.eioClient.Send(eioPacket); err != nil {
			return
		}

		for _, buf := range buffers {
			eioPacket = engineio.NewPacket(engineio.PACKET_PAYLOAD, buf.Bytes())
			if err := socket.eioClient.Send(eioPacket); err != nil {
				return
			}
		}
	}
}

func (socket *Socket) Emit(arg ...interface{}) {
	socket.send(newSocketIOPacket(__SIO_PACKET_EVENT, arg...))
}

func (socket *Socket) onMessage(packet *socketIOPacket) {
	eventFunc, isEventFound := socket.server.events[""]
	args, isOk := packet.data.([]interface{})

	if isOk && len(args) > 0 {
		switch args[0].(type) {
		case string:
			event := args[0].(string)
			tmpEventFunc, isFound := socket.server.events[event]
			if isFound {
				eventFunc = tmpEventFunc
				isEventFound = isFound
				args = args[1:]
			}
		}
	}

	if isEventFound && eventFunc != nil {
		resp := eventFunc(socket, args...)
		if packet.ackId >= 0 {
			socket.send(newSocketIOPacket(__SIO_PACKET_ACK, resp...).withAck(packet.ackId))
		}
	}
}

func (socket *Socket) onClose() {
	socket.server.socketsMtx.Lock()
	delete(socket.server.Sockets, socket.id)
	socket.server.socketsMtx.Unlock()

	for _, room := range socket.rooms {
		room.leave(socket)
	}
}

func (socket *Socket) SocketJoin(roomName string) {
	room, isFound := socket.server.Rooms[roomName]
	if !isFound {
		room = socket.server.CreateRoom(roomName)
	}
	room.join(socket)
}

func (socket *Socket) SocketLeave(roomName string) {
	room, isFound := socket.server.Rooms[roomName]
	if !isFound {
		return
	}
	room.leave(socket)
}

// Broadcasting to Sockets

func (sockets Sockets) Emit(arg ...interface{}) {
	packet := newSocketIOPacket(__SIO_PACKET_EVENT, arg...)
	for _, socket := range sockets {
		socket.send(packet)
	}
}

func (sockets Sockets) SocketJoin(roomName string) {
	var server *Server = nil

	if len(sockets) == 0 {
		return
	}

	for _, socket := range sockets {
		server = socket.server
		break
	}

	room, isFound := server.Rooms[roomName]
	if !isFound {
		room = server.CreateRoom(roomName)
	}

	for _, socket := range sockets {
		room.join(socket)
	}
}

func (sockets Sockets) SocketLeave(roomName string) {
	var server *Server = nil

	if len(sockets) == 0 {
		return
	}

	for _, socket := range sockets {
		server = socket.server
		break
	}
	room, isFound := server.Rooms[roomName]
	if !isFound {
		return
	}

	for _, socket := range sockets {
		room.leave(socket)
	}

	if len(room.sockets) == 0 {
		server.DeleteRoom(roomName)
	}
}
