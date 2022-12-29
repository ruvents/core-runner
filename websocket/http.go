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

// Тип функции для обработки получаемых сообщений по соединению.
type MessageHandler func(msg []byte, conn *Connection) []byte

// Тип функции для обработки закрытых соединений.
type CloseHandler func(conn *Connection)

type WSHandler struct {
	msgHandler   MessageHandler
	closeHandler CloseHandler
}

// NewWSHandler инициализирует HTTP-обработчик для websocket-подключений.
func NewWSHandler(msgHandler MessageHandler, closeHandler CloseHandler) *WSHandler {
	return &WSHandler{
		msgHandler:   msgHandler,
		closeHandler: closeHandler,
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("connection error: ", err)
		return
	}
	connection := NewConnection(conn, h.msgHandler, h.closeHandler)
	// Запускаем чтение и запись в горутинах, чтобы GC мог начать чистить
	// неиспользуемую память.
	go connection.read()
	go connection.write()
}
