package jobs

import (
	"log"
	"time"

	runner "github.com/ruvents/corerunner"
	"github.com/ruvents/corerunner/message"
	"google.golang.org/protobuf/proto"
)

// Простые эфемерные очереди.
type Pool struct {
	queue chan *message.JobRequest
	wrks  *runner.Pool
}

func New(wrks *runner.Pool) *Pool {
	return &Pool{
		wrks:  wrks,
		queue: make(chan *message.JobRequest, 512),
	}
}

// Queue добавляет в очередь на выполнение задачу.
func (j *Pool) Queue(r *message.JobRequest) {
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
			buf, err := proto.Marshal(req)
			if err != nil {
				log.Print("job:protobuf serialization error : ", err)
			}
			wrkCh := j.wrks.Send(
				[]byte(buf),
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
