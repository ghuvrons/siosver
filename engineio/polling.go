package engineio

import (
	"bytes"
	"io"
	"net/http"
)

// Handle transport polling
func ServePolling(w http.ResponseWriter, req *http.Request) {
	client, isClientFound := req.Context().Value(CtxKeyClient).(*Client)

	switch req.Method {
	// listener: packet sender
	case "GET":
		if !isClientFound {
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
		packet := <-client.outbox
		if _, err := w.Write(packet.encode(true)); err != nil {
			client.close()
			return
		}
		client.isPollingWaiting = false

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
			client.inbox <- packet
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("ok"))
	}
}
