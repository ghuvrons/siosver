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
	id               uuid.UUID
	isConnected      bool
	transport        eioTypeTransport
	outbox           chan *engineIOPacket
	isReadingPayload bool
	onConnected      func(*engineIOClient)
	onRecvPacket     func(*engineIOClient, *engineIOPacket)

	// can use for optional attibute
	attr interface{}
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
		}
		client.transport = __TRANSPORT_POLLING
		eioClients[uid] = client
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
	client.isConnected = true
	jsonData, _ := json.Marshal(data)
	client.send(newEngineIOPacket(__EIO_PACKET_OPEN, jsonData))
}

// send to client
func (client *engineIOClient) send(packet *engineIOPacket) {
	go func() {
		select {
		case client.outbox <- packet:
			return

		case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
			return
		}
	}()
}

// handleRequest
func (client *engineIOClient) handleRequest(buf *bytes.Buffer) {
	// TODO : return error
	for {
		var packet *engineIOPacket = nil

		if buf.Len() == 0 {
			break
		}

		if client.transport == __TRANSPORT_WEBSOCKET && client.isReadingPayload {
			packet, _ = decodeAsEngineIOPacket(buf, true)
		} else {
			packet, _ = decodeAsEngineIOPacket(buf)
		}

		if packet.packetType == __EIO_PACKET_MESSAGE || packet.packetType == __EIO_PAYLOAD {
			if client.onRecvPacket != nil {
				client.onRecvPacket(client, packet)
			}
		}
	}
	return
}

// Handle transport polling
func (client *engineIOClient) servePolling(w http.ResponseWriter, req *http.Request) {
	// TODO : return error
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
		buf := bytes.NewBuffer(b)
		client.handleRequest(buf)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("ok"))
		break
	}
}

// Handle transport websocket
func (client *engineIOClient) serveWebsocket(conn *websocket.Conn) {
	// TODO : return error
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

		buf := bytes.NewBuffer(message)
		client.handleRequest(buf)
	}
}
