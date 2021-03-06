package siosver

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	pongIrq          chan bool
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
	client, isFound := eioClients[uid]
	if !isFound || client == nil {
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
		"pingInterval": serverOptions.pingInterval,
		"pingTimeout":  serverOptions.pingTimeout,
	}
	client.isConnected = true
	jsonData, _ := json.Marshal(data)
	client.send(newEngineIOPacket(__EIO_PACKET_OPEN, jsonData))
}

// send to client
func (client *engineIOClient) send(packet *engineIOPacket, isWaitSent ...bool) {
	sendingFlag := false

	if len(isWaitSent) > 0 && isWaitSent[0] {
		packet.callback = make(chan bool)
		sendingFlag = true
	}

	go func() {
		select {
		case client.outbox <- packet:
			return

		case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
			return
		}
	}()

	if sendingFlag {
		if isSent := <-packet.callback; isSent {
			return
		}
	}
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
		} else if packet.packetType == __EIO_PACKET_PONG {
			if client.pongIrq != nil {
				client.pongIrq <- true
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
		if !client.isConnected {
			packet := newEngineIOPacket(__EIO_PACKET_CLOSE, []byte{})
			w.Write(packet.encode())
			return
		}

		select {
		case packet := <-client.outbox:
			if _, err := w.Write(packet.encode(true)); err != nil {
				client.close()
				return
			}
			if packet.callback != nil {
				packet.callback <- true
			}

		case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
			client.pongIrq = make(chan bool, 1)

			packet := newEngineIOPacket(__EIO_PACKET_PING, []byte{})
			if _, err := w.Write(packet.encode()); err != nil {
				client.close()
				return
			}

			go func() {
				select {
				case <-client.pongIrq:
					close(client.pongIrq)
					return
				case <-time.After(time.Duration(serverOptions.pingTimeout) * time.Millisecond):
					client.close()
					return
				}
			}()
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

	defer func() {
		if client.isConnected {
			client.close()
			conn.Close()
		}
	}()

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
			if _, err := conn.Write([]byte("3probe")); err != nil {
				return
			}
			client.outbox <- newEngineIOPacket(__EIO_PACKET_NOOP, []byte{})

		case string(__EIO_PACKET_UPGRADE):
			client.transport = __TRANSPORT_WEBSOCKET

		default:
			client.close()
		}
	}

	// listener: packet sender
	go func() {
		defer func() {
			if client.isConnected {
				client.close()
				conn.Close()
			}
		}()

		for client.isConnected {
			select {
			case packet := <-client.outbox:
				if _, err := conn.Write(packet.encode(true)); err != nil {
					return
				}
				if packet.callback != nil {
					packet.callback <- true
				}

			case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
				if !client.isConnected {
					return
				}
				client.pongIrq = make(chan bool, 1)

				packet := newEngineIOPacket(__EIO_PACKET_PING, []byte{})
				if _, err := conn.Write(packet.encode()); err != nil {
					return
				}

				select {
				case <-client.pongIrq:
					close(client.pongIrq)
					break
				case <-time.After(time.Duration(serverOptions.pingTimeout) * time.Millisecond):
					return
				}
			}
		}

	}()

	// listener: packet reciever
	for client.isConnected {
		message = []byte{}
		if err := websocket.Message.Receive(conn, &message); err != nil {
			return
		}

		buf := bytes.NewBuffer(message)
		client.handleRequest(buf)
	}
}

func (client *engineIOClient) close() {
	if !client.isConnected {
		return
	}
	client.isConnected = false
	delete(eioClients, client.id)
	fmt.Println("Closed client")
}
