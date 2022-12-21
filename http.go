package runner

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runner/message"
	"strings"

	"google.golang.org/protobuf/proto"
)

type HTTPHandler struct {
	StaticDir string
	Workers   *Pool
}

const (
	ErrWeb500 = "something went wrong on server side"
	ErrWeb404 = "not found"
)

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.StaticDir != "" {
		// Не позволяем доступ к скрытым файлам и перемещение вверх
		// по директориям.
		if strings.Contains(r.URL.Path, "/.") || strings.Contains(r.URL.Path, "..") {
			http.Error(w, ErrWeb404, 404)
			return
		}
		file := strings.TrimSuffix(h.StaticDir, "/") + r.URL.Path
		stat, err := os.Stat(file)
		if !errors.Is(err, os.ErrNotExist) {
			mode := stat.Mode()
			// Проверяем, что это обычный файл (не папка, не
			// unix-сокет и т.д.) и он не исполняемый.
			if mode.IsRegular() && mode.Perm()&0111 == 0 {
				http.ServeFile(w, r, file)
				return
			}
		}
	}

	m, err := h.formRequest(r)
	if err != nil {
		log.Print("error forming protobuf request: ", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	buf, err := proto.Marshal(m)
	if err != nil {
		log.Print("error serializing protobuf request:", err)
		http.Error(w, ErrWeb500, 500)
		return
	}
	d, err := h.Workers.Send(buf)
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

func (h *HTTPHandler) formRequest(r *http.Request) (*message.Request, error) {
	m := message.Request{}
	m.HttpVersion = r.Proto
	m.Path = r.URL.Path
	m.Method = r.Method

	hs := make(map[string]string)
	for k, _ := range r.Header {
		hs[k] = r.Header.Get(k)
	}
	m.Headers = hs

	qs := make(map[string]*message.List)
	for k, v := range r.URL.Query() {
		qs[k] = &message.List{Values: v}
	}
	m.Query = qs

	// Читаем тело только для запросов POST, PATCH, PUT, несмотря на то,
	// что GET поддерживает передачу тела:
	// https://stackoverflow.com/questions/978061/http-get-with-request-body
	if (r.Method != http.MethodPost &&
		r.Method != http.MethodPatch &&
		r.Method != http.MethodPut) || r.ContentLength == 0 {
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

func (h *HTTPHandler) parseFiles(r *http.Request) (map[string]*message.File, error) {
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
