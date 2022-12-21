package websocket

import (
	"log"
	"runner"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Время ожидания pong-ответа от клиента. При неполученном pong-ответе
	// соединение закрывается.
	pongWait = 180 * time.Second
	// Промежуток времени между ping-пакетами. Должен быть меньше, чем pongWait.
	pingPeriod = pongWait / 5
	// Количество собщений в очереди, ожидающие отправки. При слишком маленьком
	// значении будет блокироваться выполнение кода, вызывающего conn.Send().
	// При слишком большом будет использоваться больше памяти.
	messageQueueSize = 2048
	// Максимальный размер (в байтах) принимаемых данных в одном
	// сообщении/фрейме. При превышении соединение закрывается.
	readLimit = 8096
	// Максимальное время отправки сообщения. При превышении соединение
	// закрывается. При выставлении значения 0 время отправки сообщения не
	// ограничивается.
	writeDeadline = 0
)

type Connection struct {
	ID         runner.UUID4
	Pool       *Pool
	send       chan []byte
	connection *websocket.Conn
	closed     bool
	msgHandler MessageHandler
	mu         sync.Mutex
}

func NewConnection(conn *websocket.Conn, pool *Pool, msgHandler MessageHandler) Connection {
	return Connection{
		ID:         runner.NewUUID4(),
		Pool:       pool,
		send:       make(chan []byte, messageQueueSize),
		connection: conn,
		msgHandler: msgHandler,
		closed:     false,
	}
}

func (conn *Connection) IsClosed() bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.closed
}

func (conn *Connection) Send(data []byte) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closed {
		return
	}
	conn.send <- data
}

func (conn *Connection) Close() {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closed {
		return
	}
	close(conn.send)
	conn.closed = true
}

func (conn *Connection) write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Pool.Remove(conn)
		conn.Close()
		log.Println("close on write error")
	}()

	for {
		select {
		case data, ok := <-conn.send:
			if writeDeadline > 0 {
				conn.connection.SetWriteDeadline(time.Now().Add(writeDeadline))
			}
			// WriteMessage можно вызывать только внутри этого
			// блока.
			if !ok {
				// Канал conn.send был закрыт. Отправляем
				// сообщение клиенту о закрытии соединения и
				// закрываем его.
				conn.connection.WriteMessage(websocket.CloseMessage, []byte{})
				conn.connection.Close()
				return
			}

			start := time.Now()
			err := conn.connection.WriteMessage(websocket.TextMessage, data)
			log.Println("WriteMessage time:", time.Since(start))
			if err != nil {
				log.Printf("error writing: %v\n", err)
				return
			}
		case <-ticker.C:
			if writeDeadline > 0 {
				conn.connection.SetWriteDeadline(time.Now().Add(writeDeadline))
			}
			if err := conn.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("error pinging:", err)
				return
			}
		}
	}
}

func (conn *Connection) read() {
	defer func() {
		conn.Pool.Remove(conn)
		conn.Close()
	}()

	conn.connection.SetReadLimit(readLimit)
	conn.connection.SetReadDeadline(time.Now().Add(pongWait))
	conn.connection.SetPongHandler(
		func(string) error {
			conn.connection.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		},
	)

	for {
		_, msg, err := conn.connection.ReadMessage()
		if err != nil {
			// Если ошибка нестандартная, то выводим ее.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading: %v\n", err)
			}
			break
		}

		// Отправляем сообщение в ответ.
		if resp := conn.msgHandler(msg, conn); resp != nil && len(resp) > 0 {
			conn.send <- resp
		}
	}
}
