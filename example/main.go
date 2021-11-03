package main

import (
	"net/http"

	"github.com/ghuvrons/siosver"
)

type socketIOHandler struct {
	sioHandler *siosver.Handler
}

func main() {
	server := &http.Server{
		Addr:    ":8000",
		Handler: socketIOInit(),
	}

	server.ListenAndServe()
}

func socketIOInit() http.Handler {
	sioHandler := new(siosver.Handler)
	// sioHandler.Authenticator(func(data interface{}) bool {
	// 	fmt.Println("auth data", data)
	// 	return true
	// })

	h := socketIOHandler{}
	h.sioHandler = sioHandler

	return h
}

func (h socketIOHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	h.sioHandler.ServeHTTP(w, req)
}
