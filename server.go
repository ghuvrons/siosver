package siosver

import (
	"bytes"
	"net/http"
	"reflect"
	"sync"

	"github.com/ghuvrons/siosver/engineio"
	"github.com/google/uuid"
)

type ServerOptions struct {
	PingTimeout  int
	PingInterval int
}

type Server struct {
	engineio   *engineio.Server
	Sockets    Sockets // [TODO] make it private
	socketsMtx *sync.Mutex

	handlers struct {
		connection func(*Socket)
	}

	Rooms         map[string]*Room // key: roomName
	authenticator func(interface{}) bool
}

var managerCtxKey engineio.ContextKey = 0x01

func NewServer(opt ServerOptions) (server *Server) {
	eioOptions := engineio.EngineIOOptions{
		PingInterval: opt.PingInterval,
		PingTimeout:  opt.PingTimeout,
	}

	server = &Server{
		engineio:   engineio.NewServer(eioOptions),
		Sockets:    Sockets{},
		socketsMtx: &sync.Mutex{},
		Rooms:      map[string]*Room{},
	}

	server.engineio.OnConnection(func(c *engineio.Socket) {
		c.SetCtxValue(managerCtxKey, newManager(server))
		c.OnMessage(onEngineIOSocketRecvPacket)
		c.OnClosed(onEngineIOSocketClosed)
	})

	return
}

func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.engineio.ServeHTTP(w, req)
}

func (server *Server) Authenticator(f func(interface{}) bool) {
	server.authenticator = f
}

func (server *Server) OnConnection(f func(*Socket)) {
	server.handlers.connection = f
}

// Room methods
func (server *Server) CreateRoom(roomName string) (room *Room) {
	room = &Room{
		Name:    roomName,
		sockets: map[uuid.UUID]*Socket{},
	}

	if server.Rooms == nil {
		server.Rooms = map[string]*Room{}
	}

	server.Rooms[roomName] = room
	return
}

func (server *Server) DeleteRoom(roomName string) {
	delete(server.Rooms, roomName)
}

func onEngineIOSocketRecvPacket(eioSocket *engineio.Socket, message interface{}) {
	manager, isOk := eioSocket.GetCtxValue(managerCtxKey).(*Manager)

	if !isOk {
		return
	}

	isBinary := false

	var msgBytes []byte
	var msgString string

	switch data := message.(type) {
	case string:
		msgString = data
	case []byte:
		isBinary = true
		msgBytes = data
	}

	if isBinary {
		if manager.bufferingsocket == nil {
			return
		}

		if packet := manager.bufferingsocket.tmpPacket; packet != nil && packet.numOfBuffer > 0 {
			buf := bytes.NewBuffer(msgBytes)
			sioPacketSetBuffer(packet.data, buf)
			packet.numOfBuffer--

			// buffering complete
			if packet.numOfBuffer == 0 {
				eioSocket.IsReadingPayload = false
				manager.bufferingsocket.onMessage(packet)
				manager.bufferingsocket.tmpPacket = nil
				manager.bufferingsocket = nil
			}
		}
		return
	}

	buf := bytes.NewBuffer([]byte(msgString))
	packet := decodeToPacket(buf)
	if packet == nil {
		return
	}

	if packet.packetType == __SIO_PACKET_CONNECT {
		socket := newSocket(manager.server, packet.namespace)
		socket.eioSocket = eioSocket
		manager.sockets[packet.namespace] = socket
		socket.connect(packet)
		return
	}

	socket, isFound := manager.sockets[packet.namespace]
	if !isFound || socket == nil {
		return
	}

	switch packet.packetType {
	case __SIO_PACKET_EVENT, __SIO_PACKET_BINARY_EVENT:
		if packet.packetType == __SIO_PACKET_BINARY_EVENT {
			eioSocket.IsReadingPayload = true
			socket.tmpPacket = packet
			manager.bufferingsocket = socket

		} else {
			socket.onMessage(packet)
		}
		return

	case __SIO_PACKET_DISCONNECT:
		socket.onClosing()
		socket.onClose()
	}
}

func onEngineIOSocketClosed(eioSocket *engineio.Socket) {
	if manager, isOk := eioSocket.GetCtxValue(managerCtxKey).(*Manager); isOk {
		for _, socket := range manager.sockets {
			socket.onClosing()
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
				if rvv.IsNil() {
					continue
				}
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
		if rv.IsNil() {
			return false
		}
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
