package engineio

import (
	"sync"

	"github.com/google/uuid"
)

type CtxKey byte

const CtxKeyClient CtxKey = 0x12

type TransportType byte

const (
	TRANSPORT_POLLING   TransportType = 0
	TRANSPORT_WEBSOCKET TransportType = 1
)

type EngineIOOptions struct {
	PingInterval int
	PingTimeout  int
}

var eioClients map[uuid.UUID]*Client = map[uuid.UUID]*Client{}
var eioClientsMutex *sync.Mutex = &sync.Mutex{}
