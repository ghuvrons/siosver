package main

import (
	"fmt"
	"net/http"
	"time"

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
	myChan := make(chan bool)

	go func() {
		time.Sleep(time.Duration(1) * time.Millisecond)
		rname := "task 1"
		for {
			time.Sleep(time.Duration(1) * time.Second)
			fmt.Println(rname, "waiting")
			<-myChan
			fmt.Println(rname, "got chan")
		}
	}()

	go func() {
		time.Sleep(time.Duration(2) * time.Millisecond)
		rname := "task 2"
		for {
			time.Sleep(time.Duration(1) * time.Second)
			fmt.Println(rname, "waiting")
			<-myChan
			fmt.Println(rname, "got chan")
		}
	}()

	go func() {
		time.Sleep(time.Duration(3) * time.Millisecond)
		rname := "task 3"
		for {
			time.Sleep(time.Duration(1) * time.Second)
			fmt.Println(rname, "waiting")
			<-myChan
			fmt.Println(rname, "got chan")
		}
	}()

	for {
		myChan <- true
		fmt.Println("sent to chan")
		// time.Sleep(time.Duration(1) * time.Second)
	}

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

	io.On("connection", func(client *siosver.Socket, _ ...interface{}) (resp siosver.EventResponse) {
		fmt.Println("new client", client)

		// cbdata := map[string]interface{}{
		// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
		// }
		// client.Emit("test_bin", cbdata)
		return
	})

	io.On("message", func(client *siosver.Socket, data ...interface{}) (resp siosver.EventResponse) {
		fmt.Println("new message", data)

		// cbdata := map[string]interface{}{
		// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
		// }
		// client.Emit("test_bin", cbdata)
		return
	})

	h := socketIOServer{}
	h.sioServer = io

	return h
}

func (svr socketIOServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	svr.sioServer.ServeHTTP(w, req)
}
