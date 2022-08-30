package siosver

import (
	"bytes"
	"reflect"

	"github.com/ghuvrons/siosver/engineio"
)

type SocketIOEvent func(client *Socket, args ...interface{})

type clientHandler struct {
	server          *Server
	sockets         map[string]*Socket // key: namespace
	events          map[string]SocketIOEvent
	bufferingsocket *Socket
}

var typeOfBuffer = reflect.ValueOf(bytes.Buffer{}).Type()

var socketIOBufferIndex = map[string]interface{}{
	"_placeholder": true,
	"num":          0,
}

func newClientHandler(server *Server) *clientHandler {
	return &clientHandler{
		server:  server,
		sockets: map[string]*Socket{},
	}
}

func onEngineIOClientRecvPacket(eioClient *engineio.Client, eioPacket *engineio.Packet) {
	cHandler, isOk := eioClient.Attr.(*clientHandler)
	if !isOk {
		return
	}

	if eioPacket.Type == engineio.PACKET_PAYLOAD {
		if cHandler.bufferingsocket == nil {
			return
		}

		if packet := cHandler.bufferingsocket.tmpPacket; packet != nil && packet.numOfBuffer > 0 {
			buf := bytes.NewBuffer(eioPacket.Data)
			sioPacketSetBuffer(packet.data, buf)
			packet.numOfBuffer--

			// buffering complete
			if packet.numOfBuffer == 0 {
				eioClient.IsReadingPayload = false
				cHandler.bufferingsocket.onMessage(packet)
				cHandler.bufferingsocket.tmpPacket = nil
				cHandler.bufferingsocket = nil
			}
		}
		return
	}

	buf := bytes.NewBuffer(eioPacket.Data)
	packet := decodeAsSocketIOPacket(buf)

	switch packet.packetType {
	case __SIO_PACKET_CONNECT:
		socket := newSocket(packet.namespace)
		socket.server = cHandler.server
		socket.eioClient = eioClient
		cHandler.sockets[packet.namespace] = socket
		socket.connect(packet)
		return

	case __SIO_PACKET_EVENT, __SIO_PACKET_BINARY_EVENT:
		if eioClient.Attr != nil {
			socket, isFound := cHandler.sockets[packet.namespace]

			if isFound && socket != nil {
				if packet.packetType == __SIO_PACKET_BINARY_EVENT {
					eioClient.IsReadingPayload = true
					socket.tmpPacket = packet
					cHandler.bufferingsocket = socket

				} else {
					socket.onMessage(packet)
				}

				return
			}
		}
	}
}

func onEngineIOClientClosed(eioClient *engineio.Client) {
	if cHandler, isOk := eioClient.Attr.(*clientHandler); isOk {
		eventFunc, isEventFound := cHandler.events["close"]

		for _, socket := range cHandler.sockets {
			if isEventFound && eventFunc != nil {
				eventFunc(socket)
			}
			socket.onClose()
		}
	}
}

// search {"_placeholder":true,"num":n} and replace with buffer
func sioPacketSetBuffer(v interface{}, buf *bytes.Buffer) (isFound bool, isReplaced bool, err error) {
	rv := reflect.ValueOf(v)

	if rk := rv.Kind(); rk == reflect.Ptr || rk == reflect.Interface {
		rv = rv.Elem()
	}

	if !rv.IsValid() || ((rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice) && !rv.CanSet()) {
		return false, false, nil
	}

	switch rk := rv.Kind(); rk {
	case reflect.Ptr:
		if !rv.IsNil() {
			if isFound, isReplaced, err = sioPacketSetBuffer(rv.Interface(), buf); isFound || err != nil {
				return
			}
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
				return true, false, nil
			}
		}

		for _, key := range keys {
			rvv := rv.MapIndex(key)
			isFound, isReplaced, err = sioPacketSetBuffer(rvv.Interface(), buf)
			if isFound && !isReplaced {
				rv.SetMapIndex(key, reflect.ValueOf(buf))
				isReplaced = true
			}
			if isFound || err != nil {
				break
			}
		}

	case reflect.Array, reflect.Slice:
		for j := 0; j < rv.Len(); j++ {
			rvv := rv.Index(j)
			if rkv := rvv.Kind(); rkv == reflect.Interface {
				rvv = rvv.Elem()
			}
			isFound, isReplaced, err = sioPacketSetBuffer(rvv.Interface(), buf)
			if isFound && !isReplaced {
				if rvv = rv.Index(j); !isReplaced && rvv.CanSet() {
					rvv.Set(reflect.ValueOf(buf))
					isReplaced = true
				}
			}
			if isFound || err != nil {
				break
			}
		}
	}
	return
}

// search buffer and replace with {"_placeholder":true,"num":n}
func sioPacketGetBuffer(buffers *([]*bytes.Buffer), v interface{}) bool {
	rv := reflect.ValueOf(v)

	if rk := rv.Kind(); rk == reflect.Ptr {
		rv = rv.Elem()
	}

	if !rv.IsValid() || ((rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice) && !rv.CanSet()) {
		return false
	}

	switch rk := rv.Kind(); rk {
	case reflect.Ptr:
		return sioPacketGetBuffer(buffers, rv.Interface())

	case reflect.Struct:
		if rv.Type() == typeOfBuffer {
			return true
		}
		for i := 0; i < rv.NumField(); i++ {
			rvField := rv.Field(i)
			if rvField.CanSet() && rvField.CanAddr() {
				sioPacketGetBuffer(buffers, rvField.Addr().Interface())
			}
		}

	case reflect.Map:
		keys := rv.MapKeys()

		for _, key := range keys {
			rvv := rv.MapIndex(key)
			if isFound := sioPacketGetBuffer(buffers, rvv.Interface()); isFound {
				bufPtr, isOk := rvv.Interface().(*bytes.Buffer)
				if isOk {
					*buffers = append(*buffers, bufPtr)
					rv.SetMapIndex(key, reflect.ValueOf(socketIOBufferIndex))
				}
			}
		}

	case reflect.Array, reflect.Slice:
		for j := 0; j < rv.Len(); j++ {
			rvv := rv.Index(j)
			if rkv := rvv.Kind(); rkv == reflect.Interface {
				rvv = rvv.Elem()
			}
			if isFound := sioPacketGetBuffer(buffers, rvv.Interface()); isFound {
				bufPtr, isOk := rvv.Interface().(*bytes.Buffer)
				if isOk {
					*buffers = append(*buffers, bufPtr)
					rv.Index(j).Set(reflect.ValueOf(socketIOBufferIndex))
				}
			}
		}

	case reflect.Interface:
		rvv := rv.Elem()
		if isFound := sioPacketGetBuffer(buffers, rvv.Interface()); isFound {
			bufPtr, isOk := rvv.Interface().(*bytes.Buffer)
			if isOk && rv.CanSet() && rv.CanAddr() {
				*buffers = append(*buffers, bufPtr)
				rv.Set(reflect.ValueOf(socketIOBufferIndex))
			}
		}
	}
	return false
}
