package main

import (
	"flag"
	"log"
	"net/http"
	"runner"
	"runtime"
)

func main() {
	n := flag.Int("n", runtime.NumCPU(), "Number of workers to start")
	addr := flag.String("l", "127.0.0.1:3000", "Address HTTP-server will listen to")
	static := flag.String("s", "", "Directory to serve statically")
	flag.Parse()

	wrks := runner.Pool{}
	wrks.Start(append([]string{"php"}, flag.Args()...), *n)
	log.Print("Listening on " + *addr)
	err := http.ListenAndServe(*addr, &runner.HTTPHandler{
		StaticDir: *static,
		Workers:   &wrks,
	})
	wrks.Stop()
	if err != nil {
		log.Fatal(err)
	}
}
