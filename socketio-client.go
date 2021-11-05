package siosver

import (
	"fmt"

	"github.com/google/uuid"
)

type SocketIOClient struct {
	id        uuid.UUID
	handler   *Handler
	eioClient *engineIOClient
	namespace string
	tmpPacket *socketIOPacket
}

func newSocketIOClient(namespace string) *SocketIOClient {
	var c = &SocketIOClient{
		id:        uuid.New(),
		namespace: namespace,
	}
	return c
}

// Handle client's connect request
func (client *SocketIOClient) connect(conpacket *socketIOPacket) {
	// do authenticating ...
	var data interface{}
	var isOk bool
	if conpacket.data != nil {
		data = conpacket.data
	}
	if client.eioClient.attr.(*socketIOHandler).authenticator != nil {
		if !isOk || !client.eioClient.attr.(*socketIOHandler).authenticator(data) {
			errConnData := map[string]interface{}{
				"message": "Not authorized",
				"data": map[string]interface{}{
					"code":  "E001",
					"label": "Invalid credentials",
				},
			}
			packet := newSocketIOPacket(__SIO_PACKET_CONNECT_ERROR, errConnData)
			client.send(packet)
			return
		}
	}

	// if success
	packet := newSocketIOPacket(__SIO_PACKET_CONNECT, map[string]interface{}{"sid": client.id.String()})
	client.send(packet)
}

func (client *SocketIOClient) send(packet *socketIOPacket) {
	packet.namespace = client.namespace
	encodedPacket, buffers := packet.encode()
	eioPacket := newEngineIOPacket(__EIO_PACKET_MESSAGE, encodedPacket)
	fmt.Println("send message", packet.data)
	if len(buffers) == 0 {
		client.eioClient.send(eioPacket)

	} else {
		// binary message
		client.eioClient.send(eioPacket, true)

		for _, buf := range buffers {
			fmt.Println("send binary")
			eioPacket = newEngineIOPacket(__EIO_PAYLOAD, buf.Bytes())
			client.eioClient.send(eioPacket, true)
		}
	}
}

func (client *SocketIOClient) Emit(arg ...interface{}) {
	packet := newSocketIOPacket(__SIO_PACKET_EVENT, arg...)
	client.send(packet)
}

func (client *SocketIOClient) onMessage(packet *socketIOPacket) {
	sioHandler, isOk := client.eioClient.attr.(*socketIOHandler)
	if !isOk {
		return
	}

	eventFunc := sioHandler.events[""]
	args, isOk := packet.data.([]interface{})

	if isOk && len(args) > 0 {
		switch args[0].(type) {
		case string:
			event := args[0].(string)
			tmpEventFunc, isFound := sioHandler.events[event]
			if isFound {
				eventFunc = tmpEventFunc
				args = args[1:]
			}
		}
	}
	if eventFunc != nil {
		eventFunc(client, args)
	}
}
