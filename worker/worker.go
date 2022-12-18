package worker

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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

type Pool struct {
	pool    []*Worker
	lastWrk int
	mu      sync.Mutex
}

func (p *Pool) Start(argv []string, n int) error {
	if len(p.pool) != 0 {
		return errors.New("Already started")
	}
	for i := 0; i < n; i++ {
		wrk := Worker{}
		wrk.Start(argv)
		p.pool = append(p.pool, &wrk)
		log.Printf("PID %d: Worker started", wrk.cmd.Process.Pid)
	}
	return nil
}

func (p *Pool) Send(data []byte) ([]byte, error) {
	wrk := p.getWorker()
	res, err := wrk.Send(data)
	if err != nil && err != io.EOF {
		wrk.Restart(err != io.EOF)
	}
	return res, err
}

func (p *Pool) Stop() {
	for _, wrk := range p.pool {
		wrk.Stop()
	}
	p.pool = []*Worker{}
	p.lastWrk = 0
}

func (p *Pool) getWorker() *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	res := p.pool[p.lastWrk]
	p.lastWrk = (p.lastWrk + 1) % len(p.pool)
	return res
}

type Worker struct {
	cmd   *exec.Cmd
	read  *bufio.Reader
	argv  []string
	write *bufio.Writer
	mu    sync.Mutex
}

func (wrk *Worker) Start(argv []string) error {
	if wrk.cmd != nil {
		return errors.New("Already started")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	wrk.read = bufio.NewReader(stdout)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	wrk.write = bufio.NewWriter(stdin)

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
	wrk.cmd = cmd
	wrk.argv = argv

	return nil
}

func (wrk *Worker) Stop() error {
	wrk.mu.Lock()
	defer wrk.mu.Unlock()

	if wrk.cmd == nil {
		return errors.New("Worker is not running")
	}
	var err error
	if _, err := wrk.write.Write([]byte("exit\n")); err != nil {
		return err
	}
	if err = wrk.write.Flush(); err != nil {
		return err
	}
	if err = wrk.cmd.Wait(); err != nil {
		return err
	}
	wrk.reset()
	return nil
}

func (wrk *Worker) Wait() error {
	wrk.mu.Lock()
	defer wrk.mu.Unlock()

	if wrk.cmd == nil {
		return errors.New("Worker is not running")
	}
	wrk.mu.Lock()
	defer wrk.mu.Unlock()
	if err := wrk.cmd.Wait(); err != nil {
		return err
	}
	wrk.reset()
	return nil
}

func (wrk *Worker) Kill() error {
	wrk.mu.Lock()
	defer wrk.mu.Unlock()

	if wrk.cmd == nil {
		return errors.New("Worker is not running")
	}
	if err := wrk.cmd.Process.Kill(); err != nil {
		return err
	}
	if _, err := wrk.cmd.Process.Wait(); err != nil {
		return err
	}
	wrk.reset()
	return nil
}

func (wrk *Worker) Send(data []byte) ([]byte, error) {
	wrk.mu.Lock()
	defer wrk.mu.Unlock()

	err := wrk.writeMsg(data)
	if err != nil {
		log.Println("error writing: ", err)
		return nil, err
	}
	res, err := wrk.readMsg()
	if err != nil {
		log.Println("error reading: ", err)
		return nil, err
	}

	return res, nil
}

func (wrk *Worker) Restart(kill bool) error {
	wrk.mu.Lock()
	defer wrk.mu.Unlock()

	var err error
	if kill {
		if err = wrk.Kill(); err != nil {
			return err
		}
	} else {
		if err = wrk.Wait(); err != nil {
			return err
		}
	}
	if err = wrk.Start(wrk.argv); err != nil {
		return err
	}
	return nil
}

func (wrk *Worker) readMsg() ([]byte, error) {
	if wrk.read == nil {
		return nil, errors.New("Read pipe is not started")
	}
	// Считываем длину сообщения.
	l, err := wrk.read.ReadString('\n')
	if err != nil {
		return nil, err
	}
	ln, err := strconv.Atoi(strings.TrimSuffix(l, "\n"))
	if err != nil {
		return nil, err
	}
	// Читаем ln байт сообщения частями размером PipeChunkSize.
	buf := &bytes.Buffer{}
	for ln > 0 {
		n, err := io.CopyN(buf, wrk.read, int64(min(ln, PipeChunkSize)))
		if err != nil {
			return nil, err
		}
		ln -= int(n)
	}
	return buf.Bytes(), nil
}

func (wrk *Worker) writeMsg(data []byte) error {
	if wrk.write == nil {
		return errors.New("Write pipe is not started")
	}
	// Записываем длину сообщения.
	dlen := len(data)
	_, err := wrk.write.WriteString(strconv.Itoa(dlen) + "\n")
	if err != nil {
		return err
	}
	// Записываем сообщение по частям размером PipeChunkSize.
	ptr := 0
	for ptr < dlen {
		_, err = wrk.write.Write(data[ptr:min(ptr+PipeChunkSize, dlen)])
		if err != nil {
			return err
		}
		ptr += PipeChunkSize
	}
	return wrk.write.Flush()
}

func (wrk *Worker) reset() {
	wrk.cmd = nil
	wrk.write = nil
	wrk.read = nil
}
