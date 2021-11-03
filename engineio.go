package siosver

import "github.com/google/uuid"

type eioCtxKey byte

const eioCtxKeyClient eioCtxKey = 0x12

type eioTypeTransport byte

const (
	__TRANSPORT_POLLING   eioTypeTransport = 0
	__TRANSPORT_WEBSOCKET eioTypeTransport = 1
)

var eioClients map[uuid.UUID]*engineIOClient = map[uuid.UUID]*engineIOClient{}
