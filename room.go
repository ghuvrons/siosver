package siosver

import "github.com/google/uuid"

type Room struct {
	Name    string
	clients map[uuid.UUID]*SocketIOClient
}

func (room *Room) join(sioClient *SocketIOClient) {
	room.clients[sioClient.id] = sioClient
	sioClient.rooms[room.Name] = room
}

func (room *Room) leave(sioClient *SocketIOClient) {
	delete(room.clients, sioClient.id)
}

func (room *Room) Emit(arg ...interface{}) {
	for _, c := range room.clients {
		c.Emit(arg...)
	}
}
