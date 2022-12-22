package runner

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runner/message"
	"strings"

	"google.golang.org/protobuf/proto"
)

type HTTPHandler struct {
	staticDir string
	workers   *Pool
	maxAge    int
	cors      bool
}

func NewHTTPHandler(wrks *Pool, staticDir string, maxAge int, cors bool) *HTTPHandler {
	return &HTTPHandler{
		workers:   wrks,
		staticDir: staticDir,
		maxAge:    maxAge,
		cors:      cors,
	}
}

const (
	ErrWeb500 = "something went wrong on server side"
	ErrWeb404 = "not found"
)

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.cors {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "PUT,GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("access-control-allow-credentials", "true")
		w.Header().Set("access-control-allow-headers", "*")
	}
	if h.staticDir != "" && h.serveStatic(w, r) {
		return
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
	d, err := h.workers.Send(buf)
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
	m.Url = r.URL.String()
	m.Method = r.Method

	m.Headers = make(map[string]string)
	for k := range r.Header {
		m.Headers[k] = r.Header.Get(k)
	}

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

func (h *HTTPHandler) serveStatic(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "GET" {
		return false
	}
	// Не позволяем доступ к скрытым файлам и перемещение вверх
	// по директориям.
	if strings.Contains(r.URL.Path, "/.") || strings.Contains(r.URL.Path, "..") {
		http.Error(w, ErrWeb404, 404)
		return false
	}
	file := strings.TrimSuffix(h.staticDir, "/") + r.URL.Path
	if strings.HasSuffix(file, "/") {
		file += "index.html"
	}
	served, err := h.serveFile(w, r, file)
	if err != nil {
		log.Printf("error serving static file %v: ", err)
		return false
	}
	if served {
		return true
	}
	// Если file не существует и расширение не указано: проверяем, есть ли
	// такой файл с суффиксом ".html". Нужно для ЧПУ:
	// GET http://localhost/test отдаст http://localhost/test.html.
	if filepath.Ext(file) == "" {
		served, err = h.serveFile(w, r, file+".html")
		if err != nil {
			log.Printf("error serving static file %v: ", err)
			return false
		}
		return served
	}
	return false
}

func (h *HTTPHandler) serveFile(w http.ResponseWriter, r *http.Request, file string) (bool, error) {
	stat, err := os.Stat(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Не нужно останавливать обработку запроса с ошибкой,
			// если файла не существует и нет никаких более
			// существенных ошибок.
			return false, nil
		}
		return false, err
	}
	mode := stat.Mode()
	// Проверяем, что это обычный файл (не папка, не unix-сокет и т.д.) и
	// он не исполняемый.
	if !mode.IsRegular() || mode.Perm()&0111 != 0 {
		return false, nil
	}
	if h.maxAge != 0 {
		w.Header().Set("cache-control", fmt.Sprintf("max-age=%d", h.maxAge))
	}
	http.ServeFile(w, r, file)
	return true, nil
}
