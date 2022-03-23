package main

import (
	"fmt"
	"net/http"

	"github.com/ghuvrons/siosver"
)

type socketIOHandler struct {
	sioHandler *siosver.Handler
}

type testruct struct {
	Hoho interface{}
	Hihi map[string]interface{}
}

func main() {
	server := &http.Server{
		Addr:    ":8000",
		Handler: socketIOInit(),
	}

	server.ListenAndServe()
}

func socketIOInit() http.Handler {
	io := new(siosver.Handler)
	// sioHandler.Authenticator(func(data interface{}) bool {
	// 	fmt.Println("auth data", data)
	// 	return true
	// })

	io.On("connection", func(client *siosver.SocketIOClient, _ ...interface{}) {
		fmt.Println("new client", client)

		// cbdata := map[string]interface{}{
		// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
		// }
		// client.Emit("test_bin", cbdata)
	})

	io.On("message", func(client *siosver.SocketIOClient, data ...interface{}) {
		fmt.Println("new message", data)

		// cbdata := map[string]interface{}{
		// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
		// }
		// client.Emit("test_bin", cbdata)
	})

	h := socketIOHandler{}
	h.sioHandler = io

	return h
}

func (h socketIOHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	h.sioHandler.ServeHTTP(w, req)
}
