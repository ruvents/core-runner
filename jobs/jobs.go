package jobs

import (
	"bytes"
	"log"
	"time"

	runner "github.com/ruvents/corerunner"
)

// Простые эфемерные очереди.
type Pool struct {
	queue chan *runner.JobRequest
	wrks  *runner.Pool
}

func New(wrks *runner.Pool) *Pool {
	return &Pool{
		wrks:  wrks,
		queue: make(chan *runner.JobRequest, 512),
	}
}

// Queue добавляет в очередь на выполнение задачу.
func (j *Pool) Queue(r *runner.JobRequest) {
	j.queue <- r
}

// Stop останавливает выполнение очередей.
func (j *Pool) Stop() {
	close(j.queue)
}

// Run запускает обработку эфемерных очередей, блокируя выполнение.
func (j *Pool) Run() {
	for {
		select {
		case req, ok := <-j.queue:
			if !ok {
				return
			}
			start := time.Now()
			log.Printf("job: %s started\n", req.Name)
			var buf bytes.Buffer
			buf.Grow(4096)
			err := req.Write(&buf)
			if err != nil {
				log.Print("job: serialization error: ", err)
			}
			wrkCh := j.wrks.Send(
				buf.Bytes(),
				time.Duration(req.Timeout)*time.Millisecond,
			)
			wrkRes := <-wrkCh
			res := wrkRes.Res
			err = wrkRes.Err
			if err != nil {
				log.Print("job: request error: ", err)
			}
			if string(res) != "ok" {
				log.Print(`job: worker did not respond with "ok"`)
			} else {
				log.Printf("job: %s finished (%s)\n", req.Name, time.Since(start))
			}
		}
	}
}
