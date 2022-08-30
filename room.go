package siosver

import "github.com/google/uuid"

type Room struct {
	Name    string
	sockets map[uuid.UUID]*Socket
}

func (room *Room) join(socket *Socket) {
	room.sockets[socket.id] = socket
	socket.rooms[room.Name] = room
}

func (room *Room) leave(socket *Socket) {
	delete(room.sockets, socket.id)
}

func (room *Room) Emit(arg ...interface{}) {
	for _, c := range room.sockets {
		c.Emit(arg...)
	}
}
