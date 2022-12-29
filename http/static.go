package http

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type StaticHandler struct {
	staticDir string
	maxAge    int
	cors      bool
	next      http.Handler
}

// NewHandler инициализирует новый обработчик HTTP-запросов, способный отдавать
// статические файлы. maxAge указывает максимальную длительность (в
// секундах) хранения статически розданных файлов в браузере. При cors == true
// всем ответам будут добавляться отключающие CORS заголовки.
func NewStaticHandler(staticDir string, maxAge int, cors bool) *StaticHandler {
	return &StaticHandler{
		staticDir: staticDir,
		maxAge:    maxAge,
		cors:      cors,
	}
}

func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.proceed(w, r)
		return
	}
	if h.cors {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "PUT,GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("access-control-allow-credentials", "true")
		w.Header().Set("access-control-allow-headers", "*")
	}
	// Не позволяем доступ к скрытым файлам и перемещение вверх
	// по директориям.
	if strings.Contains(r.URL.Path, "/.") || strings.Contains(r.URL.Path, "..") {
		h.proceed(w, r)
		return
	}
	file := strings.TrimSuffix(h.staticDir, "/") + r.URL.Path
	if strings.HasSuffix(file, "/") {
		file += "index.html"
	}
	served, err := h.serveFile(w, r, file)
	if err != nil {
		log.Printf("error serving static file %v: ", err)
		return
	}
	if served {
		return
	}
	// Если file не существует и расширение не указано: проверяем, есть ли
	// такой файл с суффиксом ".html". Нужно для ЧПУ:
	// GET http://localhost/test отдаст http://localhost/test.html.
	if filepath.Ext(file) == "" {
		served, err = h.serveFile(w, r, file+".html")
		if err != nil {
			log.Printf("error serving static file %v: ", err)
			return
		}
		if served {
			return
		}
	}
	h.proceed(w, r)
}

// Next задает следующий http.Handler, который должен выполниться, если
// статического файла для ответа не существует.
func (h *StaticHandler) Next(handler http.Handler) {
	h.next = handler
}

func (h *StaticHandler) proceed(w http.ResponseWriter, r *http.Request) {
	if h.next != nil {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, ErrWeb404, 404)
}

func (h *StaticHandler) serveFile(w http.ResponseWriter, r *http.Request, file string) (bool, error) {
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
