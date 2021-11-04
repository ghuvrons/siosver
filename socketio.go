package siosver

import (
	"bytes"
	"reflect"
)

type SocketIOEvent func(*SocketIOClient, []interface{})

type socketIOHandler struct {
	sioClients    map[string]*SocketIOClient
	events        map[string]SocketIOEvent
	authenticator func(interface{}) bool
}

func onEngineIOClientConnected(eClient *engineIOClient) {
	eClient.onRecvPacket = onEngineIOClientRecvPacket
}

func onEngineIOClientRecvPacket(eClient *engineIOClient, eioPacket *engineIOPacket) {
	if eioPacket.packetType == __EIO_PAYLOAD {
		// 	if numBuf := len(client.buffers); client.readBuffersIdx < numBuf {
		// 		client.buffers[client.readBuffersIdx].b = b
		// 		client.readBuffersIdx++
		// 		if client.readBuffersIdx == numBuf {
		// 			client.buffers = nil
		// 			client.isReadingPayload = false
		// 			client.readListener <- 1
		// 		}
		// 	}
		return
	}

	buf := bytes.NewBuffer(eioPacket.data)
	packet := decodeAsSocketIOPacket(buf)

	switch packet.packetType {
	case __SIO_PACKET_CONNECT:
		sClient := newSocketIOClient(packet.namespace)
		sClient.eioClient = eClient
		eClient.attr.(socketIOHandler).sioClients[packet.namespace] = sClient
		sClient.connect(packet)
		return

	case __SIO_PACKET_EVENT, __SIO_PACKET_BINARY_EVENT:
		sioClient, isFound := eClient.attr.(socketIOHandler).sioClients[packet.namespace]

		if isFound && sioClient != nil {
			if packet.packetType == __SIO_PACKET_BINARY_EVENT {
				eClient.isReadingPayload = true
			} else {
				sioClient.onMessage(packet)
			}
			return
		}
	}
}

// search {"_placeholder":true,"num":n} and replace to buffer
func sioPacketReplaceBuffer(v interface{}) []reflect.Value {
	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if !rv.IsValid() || (rv.Kind() != reflect.Map && !rv.CanSet()) {
		return nil
	}

	return nil
}
