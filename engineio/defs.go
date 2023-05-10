package engineio

type CtxKey byte

const ctxKeySocket CtxKey = 0x12

type TransportType byte

const (
	TRANSPORT_POLLING   TransportType = 0
	TRANSPORT_WEBSOCKET TransportType = 1
)

type EngineIOOptions struct {
	PingInterval int
	PingTimeout  int
}
