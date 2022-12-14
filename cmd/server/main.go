package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	PipeChunkSize = 2048 // в рунах
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

func (w *Workers) Run(data string) ([]byte, error) {
	php := w.getWorker()
	err := php.WriteString(data + "\n")
	if err != nil {
		return []byte{}, err
	}
	resp, err := php.ReadMsg()
	if err != nil {
		return []byte{}, err
	}

	return resp, nil
}

func (w *Workers) Stop() {
	for _, php := range w.pool {
		php.Stop()
	}
	w.pool = []*PHP{}
	w.lastWrk = 0
}

func (w *Workers) getWorker() *PHP {
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

func (php *PHP) Start() {
	cmd := exec.Command("php", "index.php")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	php.read = bufio.NewReader(stdout)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	php.write = bufio.NewWriter(stdin)

	cmd.Start()
	php.cmd = cmd
}

func (php *PHP) Stop() {
	php.WriteString("exit\n")
	php.cmd.Wait()
}

func (php *PHP) ReadMsg() ([]byte, error) {
	php.mu.Lock()
	defer php.mu.Unlock()
	var res []byte
	l, err := php.read.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	ln, err := strconv.Atoi(strings.TrimSuffix(l, "\n"))
	if err != nil {
		log.Fatal(err)
	}
	for ln > 0 {
		b, err := php.read.ReadSlice('\n')
		if err != nil {
			log.Fatal(err)
		}
		res = append(res, b...)
		ln -= len(b)
	}

	return res, nil
}

func (php *PHP) WriteString(data string) error {
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
		_, err = php.write.WriteString(data[ptr:min(ptr+PipeChunkSize, dlen)])
		if err != nil {
			return err
		}
		ptr += PipeChunkSize
	}
	return php.write.Flush()
}

func main() {
	wrks := Workers{}
	wrks.Start(16)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rq, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Fatal(err)
		}
		if r.URL.Path != "/" {
			return
		}
		res, err := wrks.Run(string(rq))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprint(w, string(res))
	})
	http.ListenAndServe(":8080", nil)
	wrks.Stop()
}
