package runner

import (
	"log"
	"runner/message"
	"sync"

	"google.golang.org/protobuf/proto"
)

// Простые эфемерные очереди.
type Jobs struct {
	queue chan *message.JobRequest
	wrks  *Pool
	mu    sync.Mutex
}

func NewJobs(wrks *Pool) *Jobs {
	return &Jobs{
		wrks:  wrks,
		queue: make(chan *message.JobRequest, 128),
	}
}

func (j *Jobs) Queue(r *message.JobRequest) {
	j.queue <- r
}

func (j *Jobs) Stop() {
	close(j.queue)
}

func (j *Jobs) Start() {
	for {
		select {
		case req, ok := <-j.queue:
			if !ok {
				return
			}
			buf, err := proto.Marshal(req)
			if err != nil {
				log.Print("error serializing protobuf request: ", err)
			}
			res, err := j.wrks.Send([]byte(buf))
			if err != nil {
				log.Print("error sending request: ", err)
			}
			if string(res) != "ok" {
				log.Print(`jobs worker did not respond with "ok"`)
			}
		}
	}
}
