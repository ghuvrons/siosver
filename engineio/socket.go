package engineio

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Socket struct {
	server           *Server
	mtx              *sync.Mutex
	id               uuid.UUID
	IsConnected      bool
	Transport        TransportType
	inbox            chan *packet
	outbox           chan *packet
	isPollingWaiting bool
	IsReadingPayload bool

	handlers struct {
		message func(*Socket, interface{})
		closed  func(*Socket)
	}

	ctx           context.Context
	ctxCancelFunc context.CancelFunc
}

func newSocket(server *Server, id string) *Socket {
	var uid uuid.UUID

	if id == "" {
		uid = uuid.New()
	} else {
		uid = uuid.MustParse(id)
	}

	// search socket in memory
	server.socketsMtx.Lock()
	socket, isFound := server.sockets[uid]
	if !isFound || socket == nil {
		ctx, cancelFunc := context.WithCancel(context.Background())
		socket = &Socket{
			server:        server,
			mtx:           &sync.Mutex{},
			id:            uid,
			IsConnected:   false,
			inbox:         make(chan *packet),
			outbox:        make(chan *packet, 4),
			Transport:     TRANSPORT_POLLING,
			ctx:           ctx,
			ctxCancelFunc: cancelFunc,
		}

		server.sockets[uid] = socket

		go socket.handle()
	}
	server.socketsMtx.Unlock()

	return socket
}

// handle socket message, ping, etc
func (socket *Socket) handle() error {
	var newPacket *packet
	var pingTimeoutTimer *time.Timer
	var err error = nil

	pingIntervalTimer := time.NewTimer(time.Duration(socket.server.options.PingInterval) * time.Millisecond)

	if !socket.IsConnected {
		socket.connect()
	}

	isChanOk := false
	for {
		newPacket = nil
		if pingTimeoutTimer == nil { // if not pinging
			select {
			case <-socket.ctx.Done():
				goto close

			case newPacket, isChanOk = <-socket.inbox:
				if isChanOk {
					goto handleNewPacket
				}

			case <-pingIntervalTimer.C:
				pingIntervalTimer.Reset(time.Duration(socket.server.options.PingInterval) * time.Millisecond)
				pingTimeoutTimer = time.NewTimer(time.Duration(socket.server.options.PingTimeout) * time.Millisecond)
				if err = socket.sendPacket(NewPacket(PACKET_PING, []byte{})); err != nil {
					goto close
				}
			}

		} else { // if pinging
			select {
			case <-socket.ctx.Done():
				goto close

			case newPacket, isChanOk = <-socket.inbox:
				if isChanOk {
					goto handleNewPacket
				}

			case <-pingTimeoutTimer.C:
				err = ErrPingTimeout
				goto close
			}
		}

		continue

	handleNewPacket:
		if newPacket.packetType == PACKET_MESSAGE {
			if socket.handlers.message != nil {
				socket.handlers.message(socket, string(newPacket.data))
			}
		} else if newPacket.packetType == PACKET_PAYLOAD {
			if socket.handlers.message != nil {
				socket.handlers.message(socket, newPacket.data)
			}
		} else if newPacket.packetType == PACKET_PONG {
			pingTimeoutTimer = nil
		}
	}

	// closing socket
close:
	socket.server.socketsMtx.Lock()

	socket.IsConnected = false
	delete(socket.server.sockets, socket.id)
	if socket.handlers.closed != nil {
		socket.handlers.closed(socket)
	}

	socket.server.socketsMtx.Unlock()
	return err
}

// Handle request connect by socket
func (socket *Socket) connect() {
	data := map[string]interface{}{
		"sid":          socket.id.String(),
		"upgrades":     []string{"websocket"},
		"pingInterval": socket.server.options.PingInterval,
		"pingTimeout":  socket.server.options.PingTimeout,
	}
	socket.IsConnected = true
	jsonData, _ := json.Marshal(data)
	socket.sendPacket(NewPacket(PACKET_OPEN, jsonData))
	if socket.server.handlers.connection != nil {
		socket.server.handlers.connection(socket)
	}
}

// Send to socket client
func (socket *Socket) Send(message interface{}, timeout ...time.Duration) error {
	var p *packet = &packet{}

	switch data := message.(type) {
	case string:
		p.packetType = PACKET_MESSAGE
		p.data = []byte(data)

	case []byte:
		p.packetType = PACKET_PAYLOAD
		p.data = data

	default:
		return ErrMessageNotSupported
	}

	return socket.sendPacket(p, timeout...)
}

func (socket *Socket) sendPacket(p *packet, timeout ...time.Duration) error {
	socket.mtx.Lock()
	defer socket.mtx.Unlock()

	if socket.outbox == nil {
		return ErrSocketClosed
	}

	select {
	case socket.outbox <- p:
		return nil

	case <-socket.ctx.Done():
		return ErrSocketClosed
	}
}

func (socket *Socket) SetCtxValue(key ContextKey, value interface{}) {
	socket.ctx = context.WithValue(socket.ctx, key, value)
}

func (socket *Socket) GetCtxValue(key ContextKey) (value interface{}) {
	value = socket.ctx.Value(key)
	return
}

// OnMessage add handler on incoming new message. Second argument can be string or bytes
func (socket *Socket) OnMessage(f func(*Socket, interface{})) {
	socket.handlers.message = f
}

// OnMessage add handler on incoming new message. Second argument can be string or bytes
func (socket *Socket) OnClosed(f func(*Socket)) {
	socket.handlers.closed = f
}

func (socket *Socket) close() {
	socket.ctxCancelFunc()

	socket.mtx.Lock()
	close(socket.outbox)
	socket.outbox = nil
	socket.mtx.Unlock()
}
