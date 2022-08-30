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

type Sockets []*Socket

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
	cHandler, isOk := socket.eioClient.Attr.(*clientHandler)
	if isOk && cHandler.server.authenticator != nil {
		if !cHandler.server.authenticator(data) {
			errConnData := map[string]interface{}{
				"message": "Not authorized",
				"data": map[string]interface{}{
					"code":  "E001",
					"label": "Invalid credentials",
				},
			}
			packet := newSocketIOPacket(__SIO_PACKET_CONNECT_ERROR, errConnData)
			socket.send(packet)
			return
		}
	}

	// if success
	packet := newSocketIOPacket(__SIO_PACKET_CONNECT, map[string]interface{}{"sid": socket.id.String()})
	socket.send(packet)

	eventFunc, isEventFound := cHandler.server.events["connection"]
	if isEventFound && eventFunc != nil {
		eventFunc(socket)
	}
}

func (socket *Socket) send(packet *socketIOPacket) {
	packet.namespace = socket.namespace
	encodedPacket, buffers := packet.encode()
	eioPacket := engineio.NewPacket(engineio.PACKET_MESSAGE, encodedPacket)

	if len(buffers) == 0 {
		socket.eioClient.Send(eioPacket)

	} else {
		// binary message
		socket.eioClient.Send(eioPacket, true)

		for _, buf := range buffers {
			eioPacket = engineio.NewPacket(engineio.PACKET_PAYLOAD, buf.Bytes())
			socket.eioClient.Send(eioPacket, true)
		}
	}
}

func (socket *Socket) Emit(arg ...interface{}) {
	packet := newSocketIOPacket(__SIO_PACKET_EVENT, arg...)
	socket.send(packet)
}

func (socket *Socket) onMessage(packet *socketIOPacket) {
	cHandler, isOk := socket.eioClient.Attr.(*clientHandler)
	if !isOk {
		return
	}

	eventFunc, isEventFound := cHandler.events[""]
	args, isOk := packet.data.([]interface{})

	if isOk && len(args) > 0 {
		switch args[0].(type) {
		case string:
			event := args[0].(string)
			tmpEventFunc, isFound := cHandler.events[event]
			if isFound {
				eventFunc = tmpEventFunc
				args = args[1:]
			}
		}
	}

	if isEventFound && eventFunc != nil {
		eventFunc(socket, args...)
	}
}

func (socket *Socket) onClose() {
	for _, room := range socket.rooms {
		room.leave(socket)
	}
}

func (socket *Socket) SocketJoin(roomName string) {
	(Sockets{socket}).SocketJoin(roomName)
}

func (sockets Sockets) SocketJoin(roomName string) {
	if len(sockets) == 0 {
		return
	}

	server := sockets[0].server
	room, isFound := server.Rooms[roomName]
	if !isFound {
		room = server.CreateRoom(roomName)
	}

	for _, c := range sockets {
		room.join(c)
	}
}

func (socket *Socket) SocketLeave(roomName string) {
	(Sockets{socket}).SocketLeave(roomName)
}

func (sockets Sockets) SocketLeave(roomName string) {
	if len(sockets) == 0 {
		return
	}

	server := sockets[0].server
	room, isFound := server.Rooms[roomName]
	if !isFound {
		return
	}

	for _, c := range sockets {
		room.leave(c)
	}

	if len(room.sockets) == 0 {
		server.DeleteRoom(roomName)
	}
}
