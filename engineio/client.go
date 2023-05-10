package engineio

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	server           *Server
	id               uuid.UUID
	IsConnected      bool
	Transport        TransportType
	inbox            chan *packet
	outbox           chan *packet
	isPollingWaiting bool
	IsReadingPayload bool

	handlers struct {
		message func(*Client, interface{})
		closed  func(*Client)
	}

	// can use for optional attibute
	Attr interface{}
}

func newClient(server *Server, id string) *Client {
	var uid uuid.UUID

	if id == "" {
		uid = uuid.New()
	} else {
		uid = uuid.MustParse(id)
	}

	// search client in memory
	eioClientsMutex.Lock()
	client, isFound := eioClients[uid]
	if !isFound || client == nil {
		client = &Client{
			server:      server,
			id:          uid,
			IsConnected: false,
			inbox:       make(chan *packet),
			outbox:      make(chan *packet),
			Transport:   TRANSPORT_POLLING,
		}
		eioClients[uid] = client
	}
	eioClientsMutex.Unlock()

	go client.handle()

	return client
}

// handle client message, ping, etc
func (client *Client) handle() error {
	var newPacket *packet
	var pingTimeoutTimer *time.Timer
	var err error = nil

	pingIntervalTimer := time.NewTimer(time.Duration(client.server.options.PingInterval) * time.Millisecond)

	if !client.IsConnected {
		client.connect()
	}

	for {
		newPacket = nil
		if pingTimeoutTimer == nil { // if not pinging
			select {
			case newPacket = <-client.inbox:
				goto handleNewPacket

			case <-pingIntervalTimer.C:
				pingIntervalTimer.Reset(time.Duration(client.server.options.PingInterval) * time.Millisecond)
				pingTimeoutTimer = time.NewTimer(time.Duration(client.server.options.PingTimeout) * time.Millisecond)
				if err = client.sendPacket(NewPacket(PACKET_PING, []byte{})); err != nil {
					goto close
				}
			}

		} else { // if pinging
			select {
			case newPacket = <-client.inbox:
				goto handleNewPacket

			case <-pingTimeoutTimer.C:
				err = errors.New("timeout")
				goto close
			}
		}

		continue

	handleNewPacket:
		if newPacket.packetType == PACKET_MESSAGE {
			if client.handlers.message != nil {
				client.handlers.message(client, string(newPacket.data))
			}
		} else if newPacket.packetType == PACKET_PAYLOAD {
			if client.handlers.message != nil {
				client.handlers.message(client, newPacket.data)
			}
		} else if newPacket.packetType == PACKET_PONG {
			pingTimeoutTimer = nil
		}
	}

	// closing client
close:
	return err
}

// Handle request connect by client
func (client *Client) connect() {
	data := map[string]interface{}{
		"sid":          client.id.String(),
		"upgrades":     []string{"websocket"},
		"pingInterval": client.server.options.PingInterval,
		"pingTimeout":  client.server.options.PingTimeout,
	}
	client.IsConnected = true
	jsonData, _ := json.Marshal(data)
	client.sendPacket(NewPacket(PACKET_OPEN, jsonData))
	if client.server.handlers.connection != nil {
		client.server.handlers.connection(client)
	}
}

// send to client
func (client *Client) Send(message interface{}, timeout ...time.Duration) error {
	var p *packet = &packet{}

	switch data := message.(type) {
	case string:
		p.packetType = PACKET_MESSAGE
		p.data = []byte(data)

	case []byte:
		p.packetType = PACKET_PAYLOAD
		p.data = data

	default:
		return errors.New("not support message")
	}

	return client.sendPacket(p, timeout...)
}

func (client *Client) sendPacket(p *packet, timeout ...time.Duration) error {
	var timeoutTimer *time.Timer

	if len(timeout) > 0 {
		timeoutTimer = time.NewTimer(timeout[0])
	} else {
		timeoutTimer = time.NewTimer(time.Duration(client.server.options.PingInterval) * time.Millisecond)
	}

	select {
	case client.outbox <- p:
		return nil

	case <-timeoutTimer.C:
		return errors.New("timeout")
	}
}

// OnMessage add handler on incoming new message. Second argument can be string or bytes
func (client *Client) OnMessage(f func(*Client, interface{})) {
	client.handlers.message = f
}

// OnMessage add handler on incoming new message. Second argument can be string or bytes
func (client *Client) OnClosed(f func(*Client)) {
	client.handlers.closed = f
}

func (client *Client) close() {
	eioClientsMutex.Lock()
	if !client.IsConnected {
		return
	}
	client.IsConnected = false
	delete(eioClients, client.id)
	if client.handlers.closed != nil {
		client.handlers.closed(client)
	}
	eioClientsMutex.Unlock()
}
