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
		return nil, err
	}
	res, err := php.ReadMsg()
	if err != nil {
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

type PHP struct {
	cmd   *exec.Cmd
	read  *bufio.Reader
	write *bufio.Writer
	mu    sync.Mutex
}

func (php *PHP) Start() error {
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
				log.Println("Error while logging: ", err)
				break
			}
			log.Print(l)
		}
	}()

	cmd.Start()
	php.cmd = cmd

	return nil
}

func (php *PHP) Stop() {
	php.WriteMsg([]byte("exit\n"))
	php.cmd.Wait()
}

func (php *PHP) ReadMsg() ([]byte, error) {
	php.mu.Lock()
	defer php.mu.Unlock()
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

func main() {
	wrks := Workers{}
	wrks.Start(1)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			return
		}

		m, err := formRequest(r)
		if err != nil {
			log.Fatal(err)
		}

		buf, err := proto.Marshal(m)
		if err != nil {
			log.Fatal(err)
		}

		d, err := wrks.Run(buf)
		if err != nil {
			log.Fatal(err)
		}

		var res message.Response
		proto.Unmarshal(d, &res)

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

	res := make(map[string]*message.List)
	for k, _ := range r.Header {
		res[k] = &message.List{Value: r.Header.Values(k)}
	}
	m.Headers = res

	res = make(map[string]*message.List)
	for k, v := range r.URL.Query() {
		res[k] = &message.List{Value: v}
	}

	d, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	m.Body = string(d)

	return &m, nil
}
