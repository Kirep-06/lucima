package main

import (
	"flag"
	"net/http"

	"github.com/Kirep-06/lucima"
)

var port = flag.String("port", "", "the port to listen on")

func main() {
	flag.Parse()
	if *port != "" {
		lucima.Cli.ApiAddr = "localhost:" + *port
	}
	http.ListenAndServe(lucima.Cli.ApiAddr, lucima.NewApiServer())
}
