package siosver

import (
	"github.com/google/uuid"
)

type SocketIOClient struct {
	id        uuid.UUID
	handler   *Handler
	eioClient *engineIOClient
	namespace string
	// onConnect func()
}

func newSocketIOClient(namespace string) *SocketIOClient {
	var c = &SocketIOClient{
		id:        uuid.New(),
		namespace: namespace,
	}
	return c
}

func (client *SocketIOClient) connect(conpacket *socketIOPacket) {
	// do authenticating ...
	var data interface{}
	var isOk bool
	if conpacket.data != nil {
		data = conpacket.data
	}
	if client.eioClient.handler.authenticator != nil {
		if !isOk || !client.eioClient.handler.authenticator(data) {
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
	eioPacket := newEngineIOPacket(__EIO_PACKET_MESSAGE, packet.encode())
	client.eioClient.send(eioPacket)
}

func (client *SocketIOClient) Emit(arg ...interface{}) {
	packet := newSocketIOPacket(__SIO_PACKET_EVENT, arg)
	client.send(packet)
}

func (client *SocketIOClient) onMessage(packet *socketIOPacket) {
	var handlerFunc func(*SocketIOClient, []interface{})
	args := packet.data.([]interface{})

	if len(args) > 0 {
		switch args[0].(type) {
		case string:
			event := args[0].(string)
			handlerFunc = client.eioClient.handler.events[event]
			args = args[1:]
		}
	}
	if handlerFunc != nil {
		handlerFunc(client, args)
	}
}
