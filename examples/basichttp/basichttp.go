package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Kirep-06/lucima"
)

func main() {
	go http.ListenAndServe(lucima.Cli.ApiAddr, lucima.NewApiServer())
	http.DefaultTransport.(*http.Transport).Dial = lucima.NewDialFunc("matter.metal.gold", 5*time.Second)

	eagleListener, err := lucima.Listen("tcp", "localhost:8585", "animal.air.eagle")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello\n"))
	})
	go http.Serve(eagleListener, nil)

	time.Sleep(time.Millisecond)
	before := time.Now()
	resp, err := http.Get("http://localhost:8585/")
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, resp.Body)
	resp.Body.Close()
	fmt.Println("latency:", time.Since(before))
}
