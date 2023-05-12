package main

import (
	"fmt"
	"net/http"

	"github.com/ghuvrons/siosver"
)

type socketIOServer struct {
	sioServer *siosver.Server
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

	io.OnConnection(func(socket *siosver.Socket) {
		fmt.Println("new socket", socket)

		socket.On("message", func(data ...interface{}) {
			fmt.Println("new message", data)

			// cbdata := map[string]interface{}{
			// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
			// }
			// socket.Emit("test_bin", cbdata)
		})
		// cbdata := map[string]interface{}{
		// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
		// }
		// socket.Emit("test_bin", cbdata)
	})

	h := socketIOServer{}
	h.sioServer = io

	return h
}

func (svr socketIOServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	svr.sioServer.ServeHTTP(w, req)
}
