package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"runner/message"
	"runner/worker"

	"google.golang.org/protobuf/proto"
)

const (
	ErrWeb500 = "something went wrong on server side"
	ErrWeb404 = "not found"
)

func main() {
	n := flag.Int("n", 1, "Number of workers to start")
	addr := flag.String("l", "127.0.0.1:3000", "Address HTTP-server will listen to")
	flag.Parse()

	wrks := worker.Pool{}
	wrks.Start(flag.Args(), *n)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			return
		}
		m, err := formRequest(r)
		if err != nil {
			log.Print("error forming protobuf request:", err)
			http.Error(w, ErrWeb500, 500)
			return
		}
		buf, err := proto.Marshal(m)
		if err != nil {
			log.Print("error serializing protobuf request:", err)
			http.Error(w, ErrWeb500, 500)
			return
		}
		d, err := wrks.Send(buf)
		if err != nil {
			http.Error(w, ErrWeb500, 500)
			return
		}
		var res message.Response
		proto.Unmarshal(d, &res)
		for k, h := range res.Headers {
			w.Header().Set(k, h)
		}
		w.WriteHeader(int(res.StatusCode))
		fmt.Fprint(w, res.Body)
	})
	log.Print("Listening on " + *addr)
	http.ListenAndServe(*addr, nil)
	wrks.Stop()
}

func formRequest(r *http.Request) (*message.Request, error) {
	m := message.Request{}
	m.HttpVersion = r.Proto
	m.Path = r.URL.Path
	m.Method = r.Method

	hs := make(map[string]string)
	for k, _ := range r.Header {
		hs[k] = r.Header.Get(k)
	}
	m.Headers = hs

	qs := make(map[string]*message.List)
	for k, v := range r.URL.Query() {
		qs[k] = &message.List{Values: v}
	}
	m.Query = qs

	d, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	m.Body = string(d)

	return &m, nil
}
