package websocket

import (
	"log"
	"net/http"

	// XXX: переделать на https://github.com/nhooyr/websocket
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Переиспользуем буферы http-запроса. По умолчанию, 4КБ. Позволяет
	// сэкономить на памяти при большом количестве соединений.
	ReadBufferSize:  0,
	WriteBufferSize: 0,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type MessageHandler func(msg []byte, conn *Connection) []byte

type WSHandler struct {
	conns      Pool
	msgHandler MessageHandler
}

func NewWSHandler(msgHandler MessageHandler) *WSHandler {
	return &WSHandler{
		conns:      NewPool(),
		msgHandler: msgHandler,
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("connection error: ", err)
		return
	}
	connection := NewConnection(conn, &h.conns, h.msgHandler)
	// Запускаем чтение и запись в горутинах, чтобы GC мог начать чистить
	// неиспользуемую память.
	go connection.read()
	go connection.write()
}
