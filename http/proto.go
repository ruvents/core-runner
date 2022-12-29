package http

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runner"
	"runner/message"
	"strings"

	"google.golang.org/protobuf/proto"
)

type ProtoHandler struct {
	wrks *runner.Pool
	cors bool
}

// NewProtoHandler инициализирует новый обработчик HTTP-запросов, способный
// отдавать результат выполнения wrks.Send(). Общение с процессами воркеров
// происходит посредством сообщений в формате protobuf. Если len(wrks) == 0, то
// на все запросы отдается 404. При cors == true всем ответам будут добавляться
// отключающие CORS заголовки.
func NewProtoHandler(wrks *runner.Pool, cors bool) *ProtoHandler {
	return &ProtoHandler{
		wrks: wrks,
		cors: cors,
	}
}

const (
	ErrWeb500 = "something went wrong on server side"
	ErrWeb404 = "not found"
)

func (h *ProtoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	d, err := h.wrks.Send(buf)
	if err != nil {
		http.Error(w, ErrWeb500, 500)
		return
	}
	var res message.Response
	proto.Unmarshal(d, &res)
	for k, h := range res.Headers {
		w.Header().Set(k, h)
	}
	w.WriteHeader(int(res.StatusCode))
	fmt.Fprint(w, res.Body)
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
	} else {
		d, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		m.Body = string(d)
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
