package siosver

import (
	"github.com/google/uuid"
)

type SocketIOClient struct {
	id      uuid.UUID
	handler *Handler
	// eClient   *eioClient
	namespace string
	onConnect func()
}

func newSocketIOClient(namespace string) *SocketIOClient {
	var c = &SocketIOClient{
		id:        uuid.New(),
		namespace: namespace,
	}
	return c
}
