package siosver

import (
	"bytes"
	"reflect"
)

type SocketIOEvent func(*SocketIOClient, []interface{})

type socketIOHandler struct {
	sioClients         map[string]*SocketIOClient
	events             map[string]SocketIOEvent
	authenticator      func(interface{}) bool
	bufferingSioClient *SocketIOClient
}

func onEngineIOClientConnected(eioClient *engineIOClient) {
	eioClient.onRecvPacket = onEngineIOClientRecvPacket

	if sioHandler, isOk := eioClient.attr.(*socketIOHandler); isOk {
		sioHandler.sioClients = map[string]*SocketIOClient{}
	}
}

func onEngineIOClientRecvPacket(eioClient *engineIOClient, eioPacket *engineIOPacket) {
	sioHandler, isOk := eioClient.attr.(*socketIOHandler)
	if !isOk {
		return
	}

	if eioPacket.packetType == __EIO_PAYLOAD {
		if sioHandler.bufferingSioClient == nil {
			return
		}

		if packet := sioHandler.bufferingSioClient.tmpPacket; packet != nil && packet.numOfBuffer > 0 {
			buf := bytes.NewBuffer(eioPacket.data)
			sioPacketSetBuffer(packet.data, buf)
			packet.numOfBuffer--

			// buffering complete
			if packet.numOfBuffer == 0 {
				eioClient.isReadingPayload = false
				sioHandler.bufferingSioClient.onMessage(packet)
				sioHandler.bufferingSioClient.tmpPacket = nil
				sioHandler.bufferingSioClient = nil
			}
		}
		return
	}

	buf := bytes.NewBuffer(eioPacket.data)
	packet := decodeAsSocketIOPacket(buf)

	switch packet.packetType {
	case __SIO_PACKET_CONNECT:
		sioClient := newSocketIOClient(packet.namespace)
		sioClient.eioClient = eioClient
		sioHandler.sioClients[packet.namespace] = sioClient
		sioClient.connect(packet)
		return

	case __SIO_PACKET_EVENT, __SIO_PACKET_BINARY_EVENT:
		if eioClient.attr != nil {
			sioClient, isFound := sioHandler.sioClients[packet.namespace]

			if isFound && sioClient != nil {
				if packet.packetType == __SIO_PACKET_BINARY_EVENT {
					eioClient.isReadingPayload = true
					sioClient.tmpPacket = packet
					sioHandler.bufferingSioClient = sioClient

				} else {
					sioClient.onMessage(packet)
				}

				return
			}
		}
	}
}

// search {"_placeholder":true,"num":n} and replace with buffer
func sioPacketSetBuffer(v interface{}, buf *bytes.Buffer) (isFound bool, err error) {
	rv := reflect.ValueOf(v)

	if rk := rv.Kind(); rk == reflect.Ptr || rk == reflect.Interface {
		rv = rv.Elem()
	}

	if !rv.IsValid() || ((rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice) && !rv.CanSet()) {
		return false, nil
	}

	switch rk := rv.Kind(); rk {
	case reflect.Ptr:
		rv.Set(reflect.New(rv.Type().Elem()))
		if isFound, err = sioPacketSetBuffer(rv.Interface(), buf); isFound || err != nil {
			return
		}

	case reflect.Map:
		flag := 2
		keys := rv.MapKeys()

		if len(keys) == 2 {
			for _, key := range keys {
				strKey := key.String()
				if strKey != "_placeholder" && strKey != "num" {
					break
				}

				rvv := rv.MapIndex(key)
				if rkv := rvv.Kind(); rkv == reflect.Interface {
					rvv = rvv.Elem()
				}

				if rkv := rvv.Kind(); (strKey == "_placeholder" && rkv == reflect.Bool) || (strKey == "num" && rkv == reflect.Float64) {
					flag--
				}
			}

			if flag == 0 { // buffer req found
				return true, nil
			}
		}

		for _, key := range keys {
			rvv := rv.MapIndex(key)
			isFound, err = sioPacketSetBuffer(rvv.Interface(), buf)
			if isFound {
				rv.SetMapIndex(key, reflect.ValueOf(buf))
			}
			if isFound || err != nil {
				return
			}
		}

	case reflect.Array, reflect.Slice:
		for j := 0; j < rv.Len(); j++ {
			rvv := rv.Index(j)
			if rkv := rvv.Kind(); rkv == reflect.Interface {
				rvv = rvv.Elem()
			}
			if isFound, err = sioPacketSetBuffer(rvv.Interface(), buf); isFound || err != nil {
				return
			}
		}
	}
	return false, nil
}
