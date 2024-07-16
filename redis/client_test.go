package redis

import (
	"testing"
	"fmt"
	"log"
	"strconv"
)

func TestConnect(t *testing.T) {
	conn := Connection{}
	err := conn.Connect("localhost:6379")
	if err != nil {
		t.Fatalf("connect: %s", err)
	}
	err = conn.Subscribe("test")
	if err != nil {
		t.Fatal(err)
	}

	go func () {
		conn := Connection{}
		err := conn.Connect("localhost:6379")
		if err != nil {
			log.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			err = conn.Publish("test", "message #" + strconv.Itoa(i))
			if err != nil {
				log.Fatal(err)
			}
		}
		log.Fatal("DONE")
	}()

	for {
		result, err := conn.ReadAnswer()
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range result {
			fmt.Println(r)
		}
	}
}
