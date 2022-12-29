package runner

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

const (
	// Размер пакета данных (в байтах) для общения между Go и запускаемым
	// процессом. Чем больше значение, тем меньше будет итераций при
	// передачи большого количества данных, но есть возможость упереться в
	// ограничения транспорта.
	// https://unix.stackexchange.com/questions/11946/how-big-is-the-pipe-buffer
	PipeChunkSize = 2048
)

type Pool struct {
	pool    []*Worker
	lastWrk int
	mu      sync.Mutex
}

// Start запускает n процессов, указанных в argv. Повторный запуск возможен
// только после выполнения Stop.
func (p *Pool) Start(argv []string, n int) error {
	if len(p.pool) != 0 {
		return errors.New("already started")
	}
	for i := 0; i < n; i++ {
		wrk := Worker{}
		start := time.Now()
		err := wrk.Start(argv)
		if err != nil {
			return err
		}
		p.pool = append(p.pool, &wrk)
		log.Printf(
			"PID %d: Worker started in %s",
			wrk.cmd.Process.Pid,
			time.Now().Sub(start),
		)
	}
	return nil
}

// Send отправляет данные следующему в очереди процессу для обработки и
// возвращает ответ из него.
func (p *Pool) Send(data []byte) ([]byte, error) {
	wrk := p.getWorker()
	res, err := wrk.Send(data)
	if err != nil && err != io.EOF {
		wrk.Restart(err != io.EOF)
	}
	return res, err
}

// Stop останавливает все запущенные процессы и очищает пул.
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

// Start запускает процесс с указанными аргументами argv. Этот метод не
// дожидается завершения процесса.
func (wrk *Worker) Start(argv []string) error {
	if wrk.cmd != nil {
		return errors.New("already started")
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
					log.Println("logging error: ", err)
				}
				break
			}
			log.Print(l)
		}
	}()

	if err = cmd.Start(); err != nil {
		return err
	}
	ok, err := wrk.read.ReadString('\n')
	if err != nil {
		return err
	}
	if string(ok) != "ok\n" {
		msg, _ := io.ReadAll(wrk.read)
		return errors.New(string(msg))
	}
	wrk.cmd = cmd
	wrk.argv = argv

	return nil
}

// Stop останавливает процесс и закрывает все соответствующие буферы
// чтения/записи.
func (wrk *Worker) Stop() error {
	wrk.mu.Lock()
	defer wrk.mu.Unlock()

	if wrk.cmd == nil {
		return errors.New("Worker is not running")
	}
	var err error
	if _, err := wrk.write.Write([]byte("\n")); err != nil {
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

// Wait ждет завершения процесса и закрывает все соответствующие буферы
// чтения/записи.
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

// Kill посылает SIGKILL процессу и закрывает все соответствующие буферы
// чтения/записи.
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

// Send отправляет пакет данных процессу для обработки, дожидается ответ и
// возвращает его.
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
		return nil, errors.New("read pipe is not started")
	}
	// Считываем длину сообщения.
	var ln uint64
	if err := binary.Read(wrk.read, binary.LittleEndian, &ln); err != nil {
		return nil, err
	}
	// Пропускаем символ новой строки.
	if _, err := wrk.read.ReadByte(); err != nil {
		return nil, err
	}
	// Читаем ln байт сообщения частями размером PipeChunkSize.
	buf := &bytes.Buffer{}
	for ln > 0 {
		n, err := io.CopyN(buf, wrk.read, int64(umin(ln, PipeChunkSize)))
		if err != nil {
			return nil, err
		}
		ln -= uint64(n)
	}
	return buf.Bytes(), nil
}

func (wrk *Worker) writeMsg(data []byte) error {
	if wrk.write == nil {
		return errors.New("write pipe is not started")
	}
	// Записываем длину сообщения.
	dlen := len(data)
	if err := binary.Write(wrk.write, binary.LittleEndian, uint64(dlen)); err != nil {
		return err
	}
	if err := wrk.write.WriteByte('\n'); err != nil {
		return err
	}
	// Записываем сообщение по частям размером PipeChunkSize.
	ptr := 0
	for ptr < dlen {
		_, err := wrk.write.Write(data[ptr:min(ptr+PipeChunkSize, dlen)])
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
