package engineio

import (
	"bytes"
	"io"
	"net/http"
)

// Handle transport polling
func ServePolling(w http.ResponseWriter, req *http.Request) {
	socket, isSocketFound := req.Context().Value(ctxKeySocket).(*Socket)

	switch req.Method {
	// listener: packet sender
	case "GET":
		if !isSocketFound {
			packet := NewPacket(PACKET_CLOSE, []byte{})
			w.Write([]byte(packet.encode()))
			return
		}

		if socket.Transport != TRANSPORT_POLLING {
			if _, err := w.Write([]byte(NewPacket(PACKET_NOOP, []byte{}).encode())); err != nil {
				socket.close()
				return
			}
			return
		}

		socket.isPollingWaiting = true
		select {
		case <-req.Context().Done():
			socket.close()

		case packet := <-socket.outbox:
			if _, err := w.Write([]byte(packet.encode())); err != nil {
				socket.close()
				return
			}
		}
		socket.isPollingWaiting = false

	// listener: packet reciever
	case "POST":
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return
		}
		buf := bytes.NewBuffer(b)

		for {
			if buf.Len() == 0 {
				break
			}
			packet, _ := decodeAsEngineIOPacket(buf)
			socket.inbox <- packet
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("ok"))
	}
}
