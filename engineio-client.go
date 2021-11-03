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

	transport eioTypeTransport
	outbox    chan *engineIOPacket
	// onData     func(*engineIOClient, []byte)
	sioClients map[string]*SocketIOClient

	//when get buffer message
	// isReadingBuffer bool
	// buffers         []*eioPacketBuffer
	// readBuffersIdx int
	// readListener   chan int
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
			sioClients:  map[string]*SocketIOClient{},
		}
		client.transport = __TRANSPORT_POLLING
		eioClients[uid] = client
		client.connect()
	}

	return client
}

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

func (eClient *engineIOClient) onData(b []byte) {
	packet := decodeAsSocketIOPacket(b)

	if packet.packetType == __SIO_PACKET_CONNECT {
		// create new and add to map
		sClient := newSocketIOClient(packet.namespace)
		eClient.sioClients[packet.namespace] = sClient
		sClient.eioClient = eClient
		sClient.connect(packet)

	} else if packet.packetType == __SIO_PACKET_EVENT || packet.packetType == __SIO_PACKET_BINARY_EVENT {
		sioClient := eClient.sioClients[packet.namespace]
		if sioClient != nil {
			sioClient.onMessage(packet)
		}
	}
}

// isBase64 default true
func (client *engineIOClient) handleRequest(b []byte, isBase64 ...bool) {
	buf := bytes.NewBuffer(b)
	for {
		if buf.Len() == 0 {
			break
		}

		packet, _ := decodeAsEngineIOPacket(buf)

		if packet.packetType == __EIO_PACKET_MESSAGE {
			client.onData(packet.data)
		}
		// TODO : if packet.packetType == __EIO_PAYLOAD

		// client.handleRequest(bytes, false)
	}
	return
	// if client.isReadingBuffer {
	// 	if numBuf := len(client.buffers); client.readBuffersIdx < numBuf {
	// 		client.buffers[client.readBuffersIdx].b = b
	// 		client.readBuffersIdx++
	// 		if client.readBuffersIdx == numBuf {
	// 			client.buffers = nil
	// 			client.isReadingBuffer = false
	// 			client.readListener <- 1
	// 		}
	// 	}
	// 	return
	// }
}

func (client *engineIOClient) servePolling(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		select {
		case packet := <-client.outbox:
			w.Write(packet.encode())
		case <-time.After(time.Duration(serverOptions.pingInterval) * time.Millisecond):
			packet := newEngineIOPacket(__EIO_PACKET_PING, []byte{})
			w.Write(packet.encode())
		}
	} else if req.Method == "POST" {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return
		}
		client.handleRequest(b)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("ok"))
	}
}

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

	for {
		message = []byte{}
		if err := websocket.Message.Receive(conn, &message); err != nil {
			break
		}
		client.handleRequest(message)
	}
}
