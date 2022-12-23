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
