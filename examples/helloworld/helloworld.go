package main

import (
	"log"

	"github.com/indigo-web/indigo"
	"github.com/indigo-web/indigo/http"
	"github.com/indigo-web/indigo/http/status"
	"github.com/indigo-web/indigo/router/inbuilt"
)

const addr = ":8080"

func MyHandler(request *http.Request) *http.Response {
	return request.Respond().
		Code(status.OK).
		Header("Hello", "world").
		String("<h1>Hello, world!</h1>")
}

func main() {
	myRouter := inbuilt.New()
	myRouter.Get("/", MyHandler)

	app := indigo.New(addr).
		AutoHTTPS(":8443").
		OnListenerStart(func(listener indigo.Listener) {
			log.Printf("running on %s\n", listener.Addr)
		})

	log.Fatal(app.Serve(myRouter))
}
