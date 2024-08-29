// Простой клиент для Redis. Не знает про тип int, работает только со строками.
// Общается по протоколу RESP2.
// Документация: https://redis.io/docs/latest/develop/reference/protocol-spec/
package redis

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

type Connection struct {
	conn        *net.TCPConn
	pingTicker  *time.Ticker
	pingStopper chan bool
	topics      []string
}

const (
	// Максимальное время отправки данных. При превышении соединение
	// закрывается. При выставлении значения 0 время отправки сообщения не
	// ограничивается.
	writeDeadline = 10
	// Время ожидания ответа от сервера Redis, включая PONG.
	// При неполученном в срок ответе соединение закрывается.
	pongWait = 180 * time.Second
	// Промежуток времени между ping-пакетами. Должен быть меньше, чем
	// pongWait.
	pingPeriod = pongWait / 5
)

func Connect(address string) (*Connection, error) {
	c := Connection{}
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("address %s is not correct", address)
	}
	err = c.connect(tcpAddr)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Reconnect закрывает текущее соединение и открывает новое к тому же адресу.
func (c *Connection) Reconnect() error {
	topics := c.topics
	err := c.Close()
	if err != nil {
		return err
	}
	err = c.connect(c.conn.RemoteAddr().(*net.TCPAddr))
	if err != nil {
		return err
	}
	// resubscribe to topics
	for _, t := range topics {
		err = c.PSubscribe(t)
		if err != nil {
			return err
		}
	}
	return nil
}

// PSubscribe подписывает Redis-соединение на pubsub-топик (или топики).
func (c *Connection) PSubscribe(pattern string) error {
	if c.conn == nil {
		return errors.New("no connection")
	}
	err := c.write("PSUBSCRIBE", pattern)
	if err != nil {
		return err
	}
	_, err = c.ReadResponse()
	if err != nil {
		return err
	}
	c.topics = append(c.topics, pattern)
	return nil
}

// Publish публикует message в канале topic.
func (c *Connection) Publish(topic string, message string) error {
	if c.conn == nil {
		return errors.New("no connection")
	}
	err := c.write("publish", topic, message)
	if err != nil {
		return err
	}
	_, err = c.ReadResponse()
	return err
}

// ReadResponse парсит ответ из Redis, блокирующая операция. Возвращает массив
// строк, даже если ответ был один. Если Redis отдал ошибку, она вернется во
// втором значении error. Если был получен PONG, то автоматически продлевает
// read deadline соединения.
func (c *Connection) ReadResponse() ([]string, error) {
	for {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		res, err := c.readResponse(bufio.NewReader(c.conn), []string{})
		// Ловим PONG и отрабатываем его отдельно. По какой-то причине
		// ответ приходит в другом формате, если до этого были
		// выполнены команды subscribe/psubscribe, поэтому проверку
		// делаем более общую.
		if len(res) > 0 && strings.ToLower(res[0]) == "pong" {
			c.conn.SetReadDeadline(time.Now().Add(pongWait))
			continue
		}
		return res, err
	}
}

func (c *Connection) connect(addr *net.TCPAddr) error {
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return fmt.Errorf("could not connect to %v: %s", addr, err)
	}
	c.conn = conn
	c.conn.SetKeepAlive(true)
	go c.livelinessLoop()
	return nil
}

func (c *Connection) readResponse(
	reader *bufio.Reader,
	result []string,
) ([]string, error) {
	str, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	// Вырезаем \r\n.
	str = str[:len(str)-2]

	if str[0] == '-' {
		return nil, errors.New("from redis: " + str[1:])
	} else if str[0] == '+' || str[0] == ':' {
		return append(result, str[1:]), nil
	} else if str[0] == '$' {
		blen, err := strconv.Atoi(str[1:])
		if err != nil {
			return nil, err
		}
		if blen == -1 {
			// Здесь по хорошему нужно добавлять null, т.к. это
			// Null Bulk String по спецификации RESP2, но для этого
			// нужен тип результата []*string, что усложнило бы
			// код, поэтому пока что просто вставляем пустую
			// строку, что не совсем правильно.
			return append(result, ""), nil
		}
		buf := make([]byte, blen)
		_, err = reader.Read(buf)
		if err != nil {
			return nil, err
		}
		// Пропускаем \r\n.
		_, err = reader.Discard(2)
		if err != nil {
			return nil, err
		}
		result = append(result, string(buf))
		return result, nil
	} else if str[0] == '*' {
		cmdlen, err := strconv.Atoi(str[1:])
		if err != nil {
			return nil, err
		}
		for i := 0; i < cmdlen; i++ {
			r, err := c.readResponse(reader, result)
			if err != nil {
				return nil, err
			}
			result = r
		}
		return result, nil
	}
	return nil, fmt.Errorf("undefined type of answer: %s.", str)
}

func (c *Connection) Ping() error {
	return c.write("PING")
}

func (c *Connection) Close() error {
	if c.conn == nil {
		return errors.New("no connection")
	}
	if c.pingTicker != nil {
		c.pingTicker.Stop()
		c.pingStopper <- true
	}
	c.topics = []string{}
	err := c.conn.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (c *Connection) livelinessLoop() {
	// Каждые pingPeriod секунд проверяем, что соединение живо. Если видим
	// проблему, то пытаемся переподключиться.
	c.pingStopper = make(chan bool)
	c.pingTicker = time.NewTicker(pingPeriod)
	reconnectLoop := func() {
		for i := 0; i < 50; i++ {
			err := c.Reconnect()
			if err != nil {
				log.Printf(
					"redis: could not reconnect: %s\n",
					err,
				)
				time.Sleep(time.Second * time.Duration(i*5))
				continue
			}
			log.Println("redis: successfully reconnected")
			break
		}
	}

	for {
		select {
		case <-c.pingTicker.C:
			if c.Ping() == nil {
				continue
			}
			go reconnectLoop()

		case <-c.pingStopper:
			break
		}
	}
}

func (c *Connection) write(args ...string) error {
	if c.conn == nil {
		return errors.New("no connection")
	}
	if writeDeadline > 0 {
		c.conn.SetWriteDeadline(
			time.Now().Add(time.Second * writeDeadline),
		)
	}
	// Пишем данные в Redis массивом. Формат по RESP2.
	_, err := c.conn.Write([]byte("*" + strconv.Itoa(len(args)) + "\r\n"))
	if err != nil {
		return err
	}
	for _, str := range args {
		bytestr := []byte(str)
		bytelen := strconv.Itoa(len(bytestr))
		_, err = c.conn.Write([]byte("$" + bytelen + "\r\n"))
		if err != nil {
			return err
		}
		_, err = c.conn.Write(bytestr)
		if err != nil {
			return err
		}
		_, err = c.conn.Write([]byte("\r\n"))
		if err != nil {
			return err
		}
	}
	return err
}
