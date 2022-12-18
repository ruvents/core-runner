package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runner/message"
	"runner/worker"
	"runtime"
	"strings"

	"google.golang.org/protobuf/proto"
)

const (
	ErrWeb500 = "something went wrong on server side"
	ErrWeb404 = "not found"
)

func main() {
	n := flag.Int("n", runtime.NumCPU(), "Number of workers to start")
	addr := flag.String("l", "127.0.0.1:3000", "Address HTTP-server will listen to")
	static := flag.String("s", "", "Directory to serve statically")
	flag.Parse()

	wrks := worker.Pool{}
	wrks.Start(flag.Args(), *n)
	log.Print("Listening on " + *addr)
	http.ListenAndServe(*addr, &HTTPHandler{
		StaticDir: *static,
		Workers:   &wrks,
	})
	wrks.Stop()
}

type HTTPHandler struct {
	StaticDir string
	Workers   *worker.Pool
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.StaticDir != "" {
		// Не позволяем доступ к скрытым файлам и перемещение вверх
		// по директориям.
		if strings.Contains(r.URL.Path, "/.") || strings.Contains(r.URL.Path, "..") {
			http.Error(w, ErrWeb404, 404)
			return
		}
		file := strings.TrimSuffix(h.StaticDir, "/") + r.URL.Path
		stat, err := os.Stat(file)
		if !errors.Is(err, os.ErrNotExist) {
			mode := stat.Mode()
			// Проверяем, что это обычный файл (не папка, не
			// unix-сокет и т.д.) и он не исполняемый.
			if mode.IsRegular() && mode.Perm()&0111 == 0 {
				http.ServeFile(w, r, file)
				return
			}
		}
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
	d, err := h.Workers.Send(buf)
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
