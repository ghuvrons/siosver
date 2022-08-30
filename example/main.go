package main

import (
	"fmt"
	"net/http"

	"github.com/ghuvrons/siosver"
)

type socketIOServer struct {
	sioServer *siosver.Server
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
	io := new(siosver.Server)
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

	h := socketIOServer{}
	h.sioServer = io

	return h
}

func (svr socketIOServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	svr.sioServer.ServeHTTP(w, req)
}
