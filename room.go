package siosver

type Room struct {
	Name    string
	sockets Sockets
}

func (room *Room) join(socket *Socket) {
	room.sockets[socket.id] = socket
	socket.rooms[room.Name] = room
}

func (room *Room) leave(socket *Socket) {
	delete(room.sockets, socket.id)
}

func (room *Room) Emit(arg ...interface{}) {
	packet := newSocketIOPacket(__SIO_PACKET_EVENT, arg...)
	for _, socket := range room.sockets {
		socket.send(packet)
	}
}
