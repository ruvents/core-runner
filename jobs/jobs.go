package jobs

import (
	"log"
	"runner"
	"runner/message"

	"google.golang.org/protobuf/proto"
)

// Простые эфемерные очереди.
type Jobs struct {
	queue chan *message.JobRequest
	wrks  *runner.Pool
}

func New(wrks *runner.Pool) *Jobs {
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
				log.Print("protobuf serialization error : ", err)
			}
			res, err := j.wrks.Send([]byte(buf))
			if err != nil {
				log.Print("request error: ", err)
			}
			if string(res) != "ok" {
				log.Print(`jobs worker did not respond with "ok"`)
			}
		}
	}
}
