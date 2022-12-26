package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"runner"
	rhttp "runner/http"
	"runner/message"
	"runner/websocket"
	"runtime"
)

var jobs *runner.Jobs
func main() {
	n := flag.Int("n", runtime.NumCPU(), "Number of workers to start")
	addr := flag.String("l", "127.0.0.1:3000", "Address HTTP-server will listen to")
	static := flag.String("s", "", "Directory to serve statically")
	ma := flag.Int("ma", 0, "Max-age for statically served files (in seconds). Default is 0.")
	cors := flag.Bool("cors", false, "Add CORS headers to responses with \"*\" values")
	rpcAddr := flag.String("rpc", "", "Start RPC handler on specified address")
	flag.Parse()

	// RPC
	if *rpcAddr != "" {
		var jobsWrks runner.Pool
		// Jobs
		if err := jobsWrks.Start([]string{"php", "php/jobs.php"}, 2); err != nil {
			log.Fatal("error starting: ", err)
		}
		defer jobsWrks.Stop()
		jobs = runner.NewJobs(&jobsWrks)
		go jobs.Start()

		go startRPC(*rpcAddr)
	}

	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("there must 1 argument specified: path to php file to execute")
	}
	exe := args[0]
	if _, err := os.Stat(exe); errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file \"%s\" does not exist", exe)
	}

	// HTTP
	wrks := runner.Pool{}
	if err := wrks.Start([]string{"php", exe}, *n); err != nil {
		log.Fatal("error starting: ", err)
	}
	defer wrks.Stop()
	log.Print("Listening on " + *addr)
	http.Handle("/", rhttp.NewHTTPHandler(&wrks, *static, *ma, *cors))

	// Websocket
	wsPool := websocket.NewPool()
	http.Handle("/ws", websocket.NewWSHandler(
		func(msg []byte, conn *websocket.Connection) []byte {
			wsPool.Subscribe(conn, "topic")
			wsPool.Publish("topic", []byte("hello!"), "")
			wsPool.Remove(conn)
			return nil
		},
		func(conn *websocket.Connection) {
			fmt.Println("Connection closed!")
		},
	))

	// RPC
	if *rpcAddr != "" {
		go startRPC(*rpcAddr)
	}

	err := http.ListenAndServe(*addr, nil)

	if err != nil {
		log.Fatal(err)
	}
}

func startRPC(addr string) {
	l, e := net.Listen("tcp", addr)
	if e != nil {
		log.Fatal("RPC listen error:", e)
	}
	defer l.Close()
	rpc.Register(new(RPCHandler))
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("accept RPC connection error: ", err)
		}
		go func(c net.Conn) {
			jsonrpc.ServeConn(conn)
			c.Close()
		}(conn)
	}
}

type RPCHandler int

func (r *RPCHandler) PublishMessage(msg string, reply *string) error {
	fmt.Printf("write %s", msg)
	*reply = "test reply!"
func (r *RPCHandler) RunJob(args []any, reply *bool) error {
	// Первый аргумент -- название фоновой работы, второй -- payload.
	req := message.JobRequest{}
	req.Name = args[0].(string)
	req.Payload = args[1].(string)
	jobs.Queue(&req)
	*reply = true
	return nil
}
