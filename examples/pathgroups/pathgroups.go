package main

import (
	"fmt"
	"indigo"
	"indigo/router"
	"indigo/types"
	"log"
)

var addr = "localhost:9090"

func MyAPIHandler(_ *types.Request) types.Response {
	return types.WithResponse
}

func main() {
	r := router.NewDefaultRouter()

	api := r.Group("/api")

	v1 := api.Group("/v1")
	v1.Get("/endpoint", MyAPIHandler)

	fmt.Println("listening on", addr)
	app := indigo.NewApp(addr)
	log.Fatal(app.Serve(r))
}
