package main

import (
	"fmt"
	"net/http"

	"github.com/ghuvrons/siosver"
)

func main() {
	server := &http.Server{
		Addr:    ":8000",
		Handler: socketIOInit(),
	}

	server.ListenAndServe()
}

func socketIOInit() http.Handler {
	sioHandler := new(siosver.Handler)
	sioHandler.Authenticator(func(data interface{}) bool {
		fmt.Println("auth data", data)
		return true
	})
	return sioHandler
}
