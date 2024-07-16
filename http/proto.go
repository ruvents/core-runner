package http

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	runner "github.com/ruvents/corerunner"
	"github.com/ruvents/corerunner/message"
	"google.golang.org/protobuf/proto"
)

type ProtoHandler struct {
	wrks          *runner.Pool
	cors          bool
	timeout       time.Duration
	timeoutsCount uint
	maxTimeouts   uint
}

// NewProtoHandler инициализирует новый обработчик HTTP-запросов, способный
// отдавать результат выполнения wrks.Send(). Общение с процессами воркеров
// происходит посредством сообщений в формате protobuf. Если len(wrks) == 0, то
// на все запросы отдается 404. При cors == true всем ответам будут добавляться
// отключающие CORS заголовки. Если timeout превышен при обработке запроса 
// воркером, воркер перезапускается. Если timeout превышен maxTimeouts раз
// подряд, то убивается весь процесс corerunner.
func NewProtoHandler(
	wrks *runner.Pool,
	cors bool,
	timeout time.Duration,
	maxTimeouts uint,
) *ProtoHandler {
	return &ProtoHandler{
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

func (h *ProtoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if h.cors {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "PUT,GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("access-control-allow-credentials", "true")
		w.Header().Set("access-control-allow-headers", "*")
	}
	m, err := h.formRequest(r)
	if err != nil {
		log.Print("protobuf request error: ", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	buf, err := proto.Marshal(m)
	if err != nil {
		log.Print("protobuf serialization error:", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	wrkCh := h.wrks.Send(buf, h.timeout)
	wrkRes := <-wrkCh
	d := wrkRes.Res
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
	var res message.Response
	proto.Unmarshal(d, &res)
	for k, h := range res.Headers {
		w.Header().Set(k, h)
	}
	w.WriteHeader(int(res.StatusCode))
	fmt.Fprint(w, string(res.Body))
	log.Printf("%d %s %s (%s)\n", res.StatusCode, r.Method, r.URL.Path, time.Since(start))
}

func (h *ProtoHandler) formRequest(r *http.Request) (*message.Request, error) {
	m := message.Request{}
	m.Url = r.URL.String()
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

func (h *ProtoHandler) parseFiles(r *http.Request) (map[string]*message.File, error) {
	// Сохраняем файлы из запроса во временные файлы для передачи их путей
	// в воркер. Временные файлы будут удалены по завершению обработки
	// запроса.
	err := r.ParseMultipartForm(0)
	if err != nil {
		return nil, err
	}
	fs := make(map[string]*message.File)
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
		fs[k] = &message.File{
			Filename: fh.Filename,
			TmpPath:  f.Name(),
			Size:     fh.Size,
		}
	}
	return fs, nil
}
