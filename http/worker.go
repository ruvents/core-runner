package http

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	runner "github.com/ruvents/corerunner"
)

type WorkerHandler struct {
	wrks          *runner.Pool
	cors          bool
	timeout       time.Duration
	timeoutsCount uint
	maxTimeouts   uint
}

// NewWorkerHandler инициализирует новый обработчик HTTP-запросов, способный
// отдавать результат выполнения wrks.Send(). Общение с процессами воркеров
// происходит посредством бинарных сообщений в формате. Если len(wrks) == 0, то
// на все запросы отдается 404. При cors == true всем ответам будут добавляться
// отключающие CORS заголовки. Если timeout превышен при обработке запроса
// воркером, воркер перезапускается. Если timeout превышен maxTimeouts раз
// подряд, то убивается весь процесс corerunner.
func NewWorkerHandler(
	wrks *runner.Pool,
	cors bool,
	timeout time.Duration,
	maxTimeouts uint,
) *WorkerHandler {
	return &WorkerHandler{
		wrks:        wrks,
		cors:        cors,
		timeout:     timeout,
		maxTimeouts: maxTimeouts,
	}
}

const (
	ErrWeb500 = "something went wrong on server side"
	ErrWeb404 = "not found"
)

func (h *WorkerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if h.cors {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "PUT,GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("access-control-allow-credentials", "true")
		w.Header().Set("access-control-allow-headers", "*")
	}
	m, err := h.formRequest(r)
	if err != nil {
		log.Print("convertation error: ", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	buf := bytes.NewBuffer([]byte{})
	// Заранее увеличиваем буфер, чтобы не делать это слишком часто при
	// записи в него.
	buf.Grow(len(m.Body) + 4096)
	err = m.Write(buf)
	if err != nil {
		log.Print("serialization error:", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	wrkCh := h.wrks.Send(buf.Bytes(), h.timeout)
	wrkRes := <-wrkCh
	err = wrkRes.Err
	if err != nil {
		if errors.Is(err, runner.ErrWorkerTimedOut) {
			h.timeoutsCount = h.timeoutsCount + 1
			if h.timeoutsCount == h.maxTimeouts {
				log.Fatal("maximum number of subsequent timeouts was reached")
			}
		}
		log.Print("http handling error:", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	h.timeoutsCount = 0
	d := wrkRes.Res
	var res runner.HTTPResponse
	buf.Reset()
	buf.Write(d)
	err = res.Parse(buf)
	if err != nil {
		log.Print("deserialization error:", err)
		http.Error(w, ErrWeb500, 500)
		return
	}

	for k, h := range res.Headers {
		w.Header().Set(k, h)
	}
	w.WriteHeader(int(res.StatusCode))
	fmt.Fprint(w, string(res.Body))
	log.Printf("%d %s %s (%s)\n", res.StatusCode, r.Method, r.URL.Path, time.Since(start))
}

func (h *WorkerHandler) formRequest(r *http.Request) (*runner.HTTPRequest, error) {
	m := runner.HTTPRequest{}
	m.URL = r.URL.String()
	m.Method = r.Method

	m.Headers = make(map[string]string)
	for k := range r.Header {
		m.Headers[k] = r.Header.Get(k)
	}

	// Читаем тело только для запросов POST, PATCH, PUT, несмотря на то,
	// что GET поддерживает передачу тела:
	// https://stackoverflow.com/questions/978061/http-get-with-request-body
	if r.ContentLength == 0 ||
		(r.Method != http.MethodPost &&
			r.Method != http.MethodPatch &&
			r.Method != http.MethodPut) {
		return &m, nil
	}

	if strings.HasPrefix(r.Header.Get("content-type"), "multipart/form-data") {
		// XXX: добавить обработку r.Form.
		fs, err := h.parseFiles(r)
		if err != nil {
			return nil, err
		}
		m.Files = fs
		m.Form = make(map[string]string)
		for k, v := range r.MultipartForm.Value {
			// Пока что берем только первое значение.
			m.Form[k] = v[0]
		}
	} else {
		d, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		m.Body = d
	}

	return &m, nil
}

func (h *WorkerHandler) parseFiles(r *http.Request) (map[string]*runner.File, error) {
	// Сохраняем файлы из запроса во временные файлы для передачи их путей
	// в воркер. Временные файлы будут удалены по завершению обработки
	// запроса.
	err := r.ParseMultipartForm(0)
	if err != nil {
		return nil, err
	}
	fs := make(map[string]*runner.File)
	for k, v := range r.MultipartForm.File {
		// Пока просто берем первый файл под ключом k.
		fh := v[0]
		if fh == nil {
			continue
		}
		// XXX: нужно ли закрывать файлы? По окончании обработки
		// запроса временные файлы удаляются, так что похоже, что они
		// закрываются автоматически.
		mf, _ := fh.Open()
		f := mf.(*os.File)
		fs[k] = &runner.File{
			Filename: fh.Filename,
			TmpPath:  f.Name(),
			// должно быть безопасно, Size не должен быть
			// отрицательным
			Size: uint64(fh.Size),
		}
	}
	return fs, nil
}
