package siosver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

type engineIOClient struct {
	id      uuid.UUID
	handler *Handler

	isConnected bool

	transport    eioTypeTransport
	outbox       chan *engineIOPacket
	subClients   map[string]interface{}
	onRecvPacket func(*engineIOClient, *engineIOPacket)

	isReadingPayload bool
}

func newEngineIOClient(id string) *engineIOClient {
	var uid uuid.UUID

	if id == "" {
		uid = uuid.New()
	} else {
		uid = uuid.MustParse(id)
	}

	// search client in memory
	client := eioClients[uid]
	if client == nil {
		client = &engineIOClient{
			id:          uid,
			isConnected: false,
			outbox:      make(chan *engineIOPacket),
			subClients:  map[string]interface{}{},
		}
		client.transport = __TRANSPORT_POLLING
		eioClients[uid] = client
		client.connect()
	}

	return client
}

// Handle request connect by client
func (client *engineIOClient) connect() {
	data := map[string]interface{}{
		"sid":          client.id.String(),
		"upgrades":     []string{"websocket"},
		"pingInterval": 25000,
		"pingTimeout":  5000,
	}
	jsonData, _ := json.Marshal(data)
	client.send(newEngineIOPacket(__EIO_PACKET_OPEN, jsonData))
}

// send to client
func (client *engineIOClient) send(packet *engineIOPacket) {
	go func() {
		select {
		case client.outbox <- packet:
			if !client.isConnected {
				client.isConnected = true
			}
			return

		case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
			return
		}
	}()
}

// handleRequest
func (client *engineIOClient) handleRequest(b []byte) {
	buf := bytes.NewBuffer(b)
	for {
		if buf.Len() == 0 {
			break
		}

		var packet *engineIOPacket = nil

		if client.transport == __TRANSPORT_WEBSOCKET && client.isReadingPayload {
			packet, _ = decodeAsEngineIOPacket(buf, true)
		} else {
			packet, _ = decodeAsEngineIOPacket(buf)
		}

		if packet.packetType == __EIO_PACKET_MESSAGE || packet.packetType == __EIO_PAYLOAD {
			client.onRecvPacket(client, packet)
		}
	}
	return
}

// Handle transport polling
func (client *engineIOClient) servePolling(w http.ResponseWriter, req *http.Request) {
	switch req.Method {

	// listener: packet sender
	case "GET":
		select {
		case packet := <-client.outbox:
			w.Write(packet.encode())
		case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
			packet := newEngineIOPacket(__EIO_PACKET_PING, []byte{})
			w.Write(packet.encode())
		}
		break

	// listener: packet reciever
	case "POST":
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return
		}
		client.handleRequest(b)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("ok"))
		break
	}
}

// Handle transport websocket
func (client *engineIOClient) serveWebsocket(conn *websocket.Conn) {
	var message []byte

	// handshacking for change transport
	for {
		if client.transport == __TRANSPORT_WEBSOCKET {
			break
		}

		message = []byte{}
		err := websocket.Message.Receive(conn, &message)
		if err != nil {
			return
		}

		switch string(message) {
		case "2probe":
			conn.Write([]byte("3probe"))
			client.outbox <- newEngineIOPacket(__EIO_PACKET_NOOP, []byte{})

		case "5":
			client.transport = __TRANSPORT_WEBSOCKET

		default:
			return
		}
	}

	// listener: packet sender
	go func() {
		for {
			select {
			case packet := <-client.outbox:
				conn.Write(packet.encode())

			case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
				packet := newEngineIOPacket(__EIO_PACKET_PING, []byte{})
				conn.Write(packet.encode())
			}
		}
	}()

	// listener: packet reciever
	for {
		message = []byte{}
		if err := websocket.Message.Receive(conn, &message); err != nil {
			break
		}
		client.handleRequest(message)
	}
}
