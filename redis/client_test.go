package redis

import (
	"log"
	"strconv"
	"strings"
	"testing"
)

// TestRedisClient требует запущенный redis-server по адресу localhost:6379.
func TestRedisClient(t *testing.T) {
	conn, err := Connect("localhost:6379")
	if err != nil {
		t.Fatal("error connecting to redis:", err)
	}
	defer conn.Close()
	err = conn.PSubscribe("test:*")
	if err != nil {
		t.Fatal("error subscribing to pubsub topic:", err)
	}

	go func () {
		conn, err := Connect("localhost:6379")
		if err != nil {
			log.Fatal("error connecting to redis:", err)
		}
		defer conn.Close()
		for i := 0; i < 5; i++ {
			err = conn.Publish("test:foobar", "msg #" + strconv.Itoa(i))
			if err != nil {
				log.Fatal("error publishing pubsub message:", err)
			}
		}
	}()

	want := make(map[string]bool)
	want["msg #0"] = false
	want["msg #1"] = false
	want["msg #2"] = false
	want["msg #3"] = false
	want["msg #4"] = false

	for {
		result, err := conn.ReadResponse()
		if err != nil {
			t.Fatal("error reading pubsub message:", err)
		}
		if len(result) != 4 {
			t.Fatalf("incorrect result structure: %v", result)
		}
		if result[0] != "pmessage" {
			t.Fatalf(`expected "pmessage", got "%s"`, result[0])
		}
		if result[1] != "test:*" {
			t.Fatalf(`expected "test:*", got "%s"`, result[1])
		}
		if result[2] != "test:foobar" {
			t.Fatalf(`expected "test:foobar", got "%s"`, result[2])
		}
		msg := result[3]
		if !strings.HasPrefix(msg, "msg #") {
			t.Fatalf(
				`expected string starting with "msg #", got "%s"`,
				msg,
			)
		}
		if _, ok := want[msg]; !ok {
			t.Fatalf(`unexpected message: "%s"`, msg)
		}
		want[msg] = true
		// Проверяем, получили ли все сообщения.
		res := true
		for _, b := range want {
			res = res && b
		}
		if res {
			break
		}
	}
}
