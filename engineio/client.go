package engineio

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

type Client struct {
	id               uuid.UUID
	IsConnected      bool
	Transport        TransportType
	outbox           chan *Packet
	pongIrq          chan bool
	pingTimer        *time.Timer
	isPollingWaiting bool
	IsReadingPayload bool

	// events
	OnRecvPacket func(*Client, *Packet)
	OnClosed     func(*Client)

	// options
	options EngineIOOptions

	// can use for optional attibute
	Attr interface{}
}

func NewClient(id string, opt EngineIOOptions) *Client {
	var uid uuid.UUID

	if id == "" {
		uid = uuid.New()
	} else {
		uid = uuid.MustParse(id)
	}

	// search client in memory
	client, isFound := eioClients[uid]
	if !isFound || client == nil {
		client = &Client{
			id:          uid,
			IsConnected: false,
			outbox:      make(chan *Packet),
			pingTimer:   time.NewTimer(time.Duration(opt.PingInterval) * time.Millisecond),
		}
		client.options = opt
		client.Transport = TRANSPORT_POLLING
		eioClients[uid] = client
	}

	return client
}

// Handle request connect by client
func (client *Client) Connect() {
	data := map[string]interface{}{
		"sid":          client.id.String(),
		"upgrades":     []string{"websocket"},
		"pingInterval": client.options.PingInterval,
		"pingTimeout":  client.options.PingTimeout,
	}
	client.IsConnected = true
	jsonData, _ := json.Marshal(data)
	client.Send(NewPacket(PACKET_OPEN, jsonData))
}

// send to client
func (client *Client) Send(packet *Packet, isWaitSent ...bool) {
	sendingFlag := false

	if len(isWaitSent) > 0 && isWaitSent[0] {
		packet.callback = make(chan bool)
		sendingFlag = true
	}

	go func() {
		select {
		case client.outbox <- packet:
			return

		case <-time.After(time.Duration(client.options.PingInterval) * time.Millisecond):
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
func (client *Client) handleRequest(buf *bytes.Buffer) {
	// TODO : return error
	for {
		var packet *Packet = nil

		if buf.Len() == 0 {
			break
		}

		if client.Transport == TRANSPORT_WEBSOCKET && client.IsReadingPayload {
			packet, _ = decodeAsEngineIOPacket(buf, true)
		} else {
			packet, _ = decodeAsEngineIOPacket(buf)
		}

		if packet.Type == PACKET_MESSAGE || packet.Type == PACKET_PAYLOAD {
			if client.OnRecvPacket != nil {
				client.OnRecvPacket(client, packet)
			}
		} else if packet.Type == PACKET_PONG {
			if client.pongIrq != nil {
				client.pongIrq <- true
			}
		}
	}
	return
}

// Handle transport polling
func (client *Client) ServePolling(w http.ResponseWriter, req *http.Request) {
	// TODO : return error
	switch req.Method {

	// listener: packet sender
	case "GET":
		if !client.IsConnected {
			packet := NewPacket(PACKET_CLOSE, []byte{})
			w.Write(packet.encode())
			return
		}
		if client.Transport != TRANSPORT_POLLING {
			if _, err := w.Write(NewPacket(PACKET_NOOP, []byte{}).encode(true)); err != nil {
				client.close()
				return
			}
			return
		}

		client.isPollingWaiting = true
		select {
		case packet := <-client.outbox:
			if _, err := w.Write(packet.encode(true)); err != nil {
				client.close()
				return
			}
			if packet.callback != nil {
				packet.callback <- true
			}

		case <-client.pingTimer.C:
			client.pongIrq = make(chan bool, 1)

			client.resetPingTimer()
			packet := NewPacket(PACKET_PING, []byte{})
			if _, err := w.Write(packet.encode()); err != nil {
				client.close()
				return
			}

			go func() {
				select {
				case <-client.pongIrq:
					close(client.pongIrq)
					return
				case <-client.pingWasTimeout():
					client.close()
					return
				}
			}()
		}
		client.isPollingWaiting = false
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
func (client *Client) ServeWebsocket(conn *websocket.Conn) {
	// TODO : return error
	var message []byte

	defer func() {
		if client.IsConnected {
			client.close()
			conn.Close()
		}
	}()

	// handshacking for change transport
	for isHandshackingFinished := false; !isHandshackingFinished; {
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
			client.Transport = TRANSPORT_WEBSOCKET
			if client.isPollingWaiting {
				client.outbox <- NewPacket(PACKET_NOOP, []byte{})
			}

		case string(PACKET_UPGRADE):
			isHandshackingFinished = true
			break

		default:
			client.close()
			return
		}
	}

	// listener: packet sender
	go func() {
		defer func() {
			if client.IsConnected {
				client.close()
				conn.Close()
			}
		}()

		for client.IsConnected {
			select {
			case packet := <-client.outbox:
				if _, err := conn.Write(packet.encode(true)); err != nil {
					return
				}
				if packet.callback != nil {
					packet.callback <- true
				}

			case <-client.pingTimer.C:
				if !client.IsConnected {
					return
				}
				client.pongIrq = make(chan bool, 1)

				client.resetPingTimer()
				packet := NewPacket(PACKET_PING, []byte{})
				if _, err := conn.Write(packet.encode()); err != nil {
					return
				}

				select {
				case <-client.pongIrq:
					close(client.pongIrq)
					break
				case <-client.pingWasTimeout():
					return
				}
			}
		}

	}()

	// listener: packet reciever
	for client.IsConnected {
		message = []byte{}
		if err := websocket.Message.Receive(conn, &message); err != nil {
			return
		}

		buf := bytes.NewBuffer(message)
		client.handleRequest(buf)
	}
}

func (client *Client) close() {
	if !client.IsConnected {
		return
	}
	client.IsConnected = false
	delete(eioClients, client.id)
	if client.OnClosed != nil {
		client.OnClosed(client)
	}
}

// Handle transport websocket
func (client *Client) resetPingTimer() {
	client.pingTimer.Reset(time.Duration(client.options.PingInterval) * time.Millisecond)
}

func (client *Client) pingWasTimeout() <-chan time.Time {
	return time.After(time.Duration(client.options.PingTimeout) * time.Millisecond)
}
