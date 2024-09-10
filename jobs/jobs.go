package jobs

import (
	"bytes"
	"fmt"
	"time"

	runner "github.com/ruvents/corerunner"
)

// Обертка надо очередью воркеров для удобного вызова кода внутри процессов.
// Можно использовать для очередей, RPC вызовов и т.д.
type Pool struct {
	wrks *runner.Pool
}

func NewPool(wrks *runner.Pool) *Pool {
	return &Pool{wrks: wrks}
}

// Call вызывает метод name в свободном воркере, передавая ему payload. Если
// метод не успевает выполниться в течение timeout, то возвращается ошибка
// runner.ErrWorkerTimedOut.
func (j *Pool) Call(
	name string, payload []byte, timeout time.Duration,
) ([]byte, error) {
	req := runner.JobRequest{
		Name: name,
		Payload: payload,
		Timeout: uint64(timeout.Milliseconds()),
	}
	buf := bytes.NewBuffer([]byte{})
	buf.Grow(4096)
	err := req.Write(buf)
	if err != nil {
		return nil, fmt.Errorf("job: serialization error: %s", err)
	}

	wrkCh := j.wrks.Send(buf.Bytes(), timeout)
	wrkRes := <-wrkCh
	d := wrkRes.Res
	err = wrkRes.Err
	if err != nil {
		return nil, fmt.Errorf("job: request error: %s", err)
	}

	var res runner.JobResponse
	buf.Reset()
	buf.Write(d)
	err = res.Parse(buf)
	if err != nil {
		return nil, fmt.Errorf(
			"job: response deserialization error: %s",
			err,
		)
	}
	return res.Payload, nil
}
