package siosver

import (
	"github.com/google/uuid"
)

type Client struct {
	id      uuid.UUID
	handler *Handler
	// eClient   *eioClient
	namespace string
	onConnect func()
}

func newClient(namespace string) *Client {
	var c = &Client{
		id:        uuid.New(),
		namespace: namespace,
	}
	return c
}
