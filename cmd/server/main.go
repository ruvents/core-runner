package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"runner/message"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
)

const (
	PipeChunkSize = 2048 // в байтах
	ErrWeb500     = "something went wrong on server side"
	ErrWeb404     = "not found"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Workers struct {
	pool    []*PHP
	lastWrk int
	mu      sync.Mutex
}

func (w *Workers) Start(n int) error {
	if len(w.pool) != 0 {
		return errors.New("Already started")
	}

	for i := 0; i < n; i++ {
		php := PHP{}
		php.Start()
		w.pool = append(w.pool, &php)
	}
	return nil
}

func (w *Workers) Run(data []byte) ([]byte, error) {
	php := w.getWorker()
	err := php.WriteMsg(data)
	if err != nil {
		log.Println("error writing: ", err)
		w.restartWorker(php, err != io.EOF)
		return nil, err
	}
	res, err := php.ReadMsg()
	if err != nil {
		log.Println("error reading: ", err)
		w.restartWorker(php, err != io.EOF)
		return nil, err
	}

	return res, nil
}

func (w *Workers) Stop() {
	for _, php := range w.pool {
		php.Stop()
	}
	w.pool = []*PHP{}
	w.lastWrk = 0
}

func (w *Workers) getWorker() *PHP {
	w.mu.Lock()
	defer w.mu.Unlock()
	res := w.pool[w.lastWrk]
	w.lastWrk = (w.lastWrk + 1) % len(w.pool)
	return res
}

func (w *Workers) restartWorker(wrk *PHP, kill bool) error {
	var err error
	if !kill {
		if err = wrk.Wait(); err != nil {
			return err
		}
	} else {
		if err = wrk.Kill(); err != nil {
			return err
		}
	}
	if err = wrk.Start(); err != nil {
		return err
	}
	return nil
}

type PHP struct {
	cmd   *exec.Cmd
	read  *bufio.Reader
	write *bufio.Writer
	mu    sync.Mutex
}

func (php *PHP) Start() error {
	if php.cmd != nil {
		return errors.New("Already started")
	}
	cmd := exec.Command("php", "index.php")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	php.read = bufio.NewReader(stdout)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	php.write = bufio.NewWriter(stdin)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	errReader := bufio.NewReader(stderr)
	go func() {
		for {
			l, err := errReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Println("Error while logging: ", err)
				}
				break
			}
			log.Print(l)
		}
	}()

	cmd.Start()
	php.cmd = cmd

	return nil
}

func (php *PHP) Stop() error {
	php.mu.Lock()
	defer php.mu.Unlock()
	if php.cmd == nil {
		return errors.New("Worker is not running")
	}
	var err error
	if _, err := php.write.Write([]byte("exit\n")); err != nil {
		return err
	}
	if err = php.write.Flush(); err != nil {
		return err
	}
	if err = php.cmd.Wait(); err != nil {
		return err
	}
	php.reset()
	return nil
}

func (php *PHP) Wait() error {
	php.mu.Lock()
	defer php.mu.Unlock()

	if php.cmd == nil {
		return errors.New("Worker is not running")
	}
	php.mu.Lock()
	defer php.mu.Unlock()
	if err := php.cmd.Wait(); err != nil {
		return err
	}
	php.reset()
	return nil
}

func (php *PHP) Kill() error {
	php.mu.Lock()
	defer php.mu.Unlock()

	if php.cmd == nil {
		return errors.New("Worker is not running")
	}
	if err := php.cmd.Process.Kill(); err != nil {
		return err
	}
	if _, err := php.cmd.Process.Wait(); err != nil {
		return err
	}
	php.reset()
	return nil
}

func (php *PHP) ReadMsg() ([]byte, error) {
	php.mu.Lock()
	defer php.mu.Unlock()

	if php.read == nil {
		return nil, errors.New("Read pipe is not started")
	}
	l, err := php.read.ReadString('\n')
	if err != nil {
		return nil, err
	}
	ln, err := strconv.Atoi(strings.TrimSuffix(l, "\n"))
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	var res []byte
	for ln > 0 {
		n, err := io.CopyN(buf, php.read, int64(min(ln, PipeChunkSize)))
		if err != nil {
			return nil, err
		}
		res = append(res, buf.Bytes()...)
		ln -= int(n)
	}

	return res, nil
}

func (php *PHP) WriteMsg(data []byte) error {
	php.mu.Lock()
	defer php.mu.Unlock()

	if php.write == nil {
		return errors.New("Write pipe is not started")
	}
	// Записываем длину сообщения.
	dlen := len(data)
	_, err := php.write.WriteString(strconv.Itoa(dlen) + "\n")
	if err != nil {
		return err
	}
	// Записываем сообщение по частям.
	ptr := 0
	for ptr < dlen {
		_, err = php.write.Write(data[ptr:min(ptr+PipeChunkSize, dlen)])
		if err != nil {
			return err
		}
		ptr += PipeChunkSize
	}
	return php.write.Flush()
}

func (php *PHP) reset() {
	php.cmd = nil
	php.write = nil
	php.read = nil
}

func main() {
	wrks := Workers{}
	wrks.Start(1)
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

		d, err := wrks.Run(buf)
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
	http.ListenAndServe(":8080", nil)
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
