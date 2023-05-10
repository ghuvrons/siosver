package engineio

import "golang.org/x/net/websocket"

type websocketMessage struct {
	payloadType byte
	message     []byte
}

var TransportWebsocket = struct {
	codec websocket.Codec
}{
	codec: websocket.Codec{Marshal: wsMarshal, Unmarshal: wsUnmarshal},
}

var TransportWebsocketHandler = websocket.Handler(ServeWebsocket)

func wsMarshal(v interface{}) (msg []byte, payloadType byte, err error) {
	switch data := v.(type) {
	case string:
		return []byte(data), websocket.TextFrame, nil
	case []byte:
		return data, websocket.BinaryFrame, nil
	}
	return nil, websocket.UnknownFrame, websocket.ErrNotSupported
}

func wsUnmarshal(msg []byte, payloadType byte, v interface{}) (err error) {
	data, isOK := v.(*websocketMessage)

	if !isOK {
		return websocket.ErrNotSupported
	}

	data.payloadType = payloadType
	data.message = msg

	return nil
}

func ServeWebsocket(conn *websocket.Conn) {
	client := conn.Request().Context().Value(CtxKeyClient).(*Client)
	message := websocketMessage{}
	var packet *Packet

	defer func() {
		client.close()
		conn.Close()
	}()

	// handshacking for change transport
	for isHandshackingFinished := false; !isHandshackingFinished; {
		err := TransportWebsocket.codec.Receive(conn, &message)
		if err != nil {
			return
		}

		switch string(message.message) {
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

		default:
			return
		}
	}

	// listener: packet sender
	go func() {
		defer func() {
			conn.Close()
		}()

		for client.IsConnected {
			packet := <-client.outbox
			if _, err := conn.Write(packet.encode(true)); err != nil {
				return
			}
			if packet.callback != nil {
				packet.callback <- true
			}
		}
	}()

	// listener: packet reciever
	for client.IsConnected {
		packet = nil
		if err := TransportWebsocket.codec.Receive(conn, &message); err != nil {
			return
		}

		if len(message.message) == 0 {
			continue
		}

		// handle incomming packet
		if message.payloadType == 0x01 { // string message
			packet = &Packet{
				Type: eioPacketType(message.message[0]),
				Data: message.message[1:],
			}
		} else if message.payloadType == 0x02 { // binary message
			packet = &Packet{
				Type: PACKET_PAYLOAD,
				Data: message.message,
			}
		}

		client.inbox <- packet
	}
}
