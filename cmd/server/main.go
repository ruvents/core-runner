package main

import (
	"encoding/json"
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

var wsPool *websocket.Pool
var jobs *runner.Jobs

// Пример приложения, собранного из библиотеки runner.
func main() {
	php := flag.String("p", "", "Run specified PHP-file for HTTP handling. HTTP workers will not be started if flag is omitted.")
	wrksNum := flag.Int("n", runtime.NumCPU(), "Number of HTTP-workers to start")
	addr := flag.String("l", "127.0.0.1:3000", "Address HTTP-server will listen to")
	static := flag.String("s", "", "Directory to serve statically")
	maxAge := flag.Int("ma", 0, "Max-age for statically served files (in seconds). Default is 0.")
	cors := flag.Bool("cors", false, "Add CORS headers to responses with \"*\" values")
	jobs := flag.String("j", "", "Run specified PHP-file for jobs handling. Jobs will not be started if flag is omitted.")
	rpcAddr := flag.String("rpc", "", "Start RPC handler on specified address")
	flag.Parse()

	// RPC
	if *rpcAddr != "" {
		if *jobs != "" {
			mustExist(*jobs)
			var wrks runner.Pool
			// Jobs
			if err := wrks.Start([]string{"php", *jobs}, 2); err != nil {
				log.Fatal("error starting: ", err)
			}
			defer wrks.Stop()
			go runner.NewJobs(&wrks).Start()
		}
		go startRPC(*rpcAddr)
	}

	// HTTP
	if *php != "" && *wrksNum > 0 {
		mustExist(*php)
		wrks := runner.Pool{}
		if err := wrks.Start([]string{"php", *php}, *wrksNum); err != nil {
			log.Fatal("error starting: ", err)
		}
		defer wrks.Stop()
		http.Handle("/", rhttp.NewHTTPHandler(&wrks, *static, *maxAge, *cors))
	}

	// Websocket
	wsPool = websocket.NewPool()
	http.Handle("/ws", websocket.NewWSHandler(
		func(msg []byte, conn *websocket.Connection) []byte {
			cmd := struct {
				Command string
				Topics  []string
			}{}
			err := json.Unmarshal(msg, &cmd)
			if err != nil {
				return nil
			}
			if cmd.Command == "join" && len(cmd.Topics) > 0 {
				wsPool.Subscribe(conn, cmd.Topics[0])
			}
			return []byte("ok")
		},
		func(conn *websocket.Connection) {
			wsPool.Remove(conn)
		},
	))

	if *jobs != "" {
		log.Println("jobs: waiting for requests")
	}
	if *rpcAddr != "" {
		log.Printf("rpc: listening on %s", *rpcAddr)
	}
	if *static != "" {
		log.Printf(
			`http: serving files statically from directory "%s"; CORS: %t; Max-Age: %d`,
			*static,
			*cors,
			*maxAge,
		)
	}
	if *php != "" {
		log.Printf(`http: serving PHP application "%s"`, *static)
	}
	log.Print("http: listening on " + *addr)
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

func (r *RPCHandler) PublishMessage(args []string, reply *bool) error {
	// Первый аргумент -- топик, второй текстовое сообщение.
	m := struct {
		Topic   string `json:"topic"`
		Payload string `json:"payload"`
	}{
		Topic:   args[0],
		Payload: args[1],
	}
	msg, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("json marshaling error: %v", err)
	}
	wsPool.Publish(args[0], []byte(msg), "")
	*reply = true
	return nil
}

func (r *RPCHandler) RunJob(args []any, reply *bool) error {
	// Первый аргумент -- название фоновой работы, второй -- payload.
	req := message.JobRequest{}
	req.Name = args[0].(string)
	req.Payload = args[1].(string)
	jobs.Queue(&req)
	*reply = true
	return nil
}

func mustExist(file string) {
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file \"%s\" does not exist", file)
	}
}
