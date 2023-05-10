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
	inbox            chan *Packet
	outbox           chan *Packet
	isPollingWaiting bool
	IsReadingPayload bool

	// events
	OnRecvPacket func(*Client, *Packet)
	OnClosed     func(*Client)

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
			inbox:       make(chan *Packet),
			outbox:      make(chan *Packet),
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
	var newPacket *Packet
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
				if err = client.Send(NewPacket(PACKET_PING, []byte{})); err != nil {
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
		if newPacket.Type == PACKET_MESSAGE || newPacket.Type == PACKET_PAYLOAD {
			if client.OnRecvPacket != nil {
				client.OnRecvPacket(client, newPacket)
			}
		} else if newPacket.Type == PACKET_PONG {
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
	client.Send(NewPacket(PACKET_OPEN, jsonData))
	if client.server.handlers.connection != nil {
		client.server.handlers.connection(client)
	}
}

// send to client
func (client *Client) Send(packet *Packet, timeout ...time.Duration) error {
	var timeoutTimer *time.Timer

	if len(timeout) > 0 {
		timeoutTimer = time.NewTimer(timeout[0])
	} else {
		timeoutTimer = time.NewTimer(time.Duration(client.server.options.PingInterval) * time.Millisecond)
	}

	select {
	case client.outbox <- packet:
		return nil

	case <-timeoutTimer.C:
		return errors.New("timeout")
	}
}

func (client *Client) close() {
	eioClientsMutex.Lock()
	if !client.IsConnected {
		return
	}
	client.IsConnected = false
	delete(eioClients, client.id)
	if client.OnClosed != nil {
		client.OnClosed(client)
	}
	eioClientsMutex.Unlock()
}
