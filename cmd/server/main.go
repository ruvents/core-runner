package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"runtime"
	"strings"
	"time"

	runner "github.com/ruvents/corerunner"
	rhttp "github.com/ruvents/corerunner/http"
	"github.com/ruvents/corerunner/http/websocket"
	"github.com/ruvents/corerunner/jobs"
	"github.com/ruvents/corerunner/redis"
)

var wsPool *websocket.Pool
var jobsPool *jobs.Pool

// Пример приложения, собранного из библиотеки corerunner.
func main() {
	httpExe := flag.String("p", "", "Run specified PHP-file for HTTP handling. HTTP workers will not be started if flag is omitted.")
	wrksNum := flag.Int("n", runtime.NumCPU(), "Number of HTTP-workers to start")
	addr := flag.String("l", "127.0.0.1:3000", "Address HTTP-server will listen to")
	static := flag.String("s", "", "Directory to serve statically")
	maxAge := flag.Int("ma", 0, "Max-age for statically served files (in seconds). Default is 0.")
	cors := flag.Bool("cors", false, "Add CORS headers to responses with \"*\" values")
	jobsExe := flag.String("j", "", "Run specified PHP-file for jobs handling. Jobs will not be started if flag is omitted.")
	rpcAddr := flag.String("rpc", "", "Start RPC handler on specified address")
	redisAddr := flag.String("r", "", "Start Redis listener to specified address")
	flag.Parse()

	env := os.Environ()
	// RPC
	if *rpcAddr != "" {
		if *jobsExe != "" {
			mustExist(*jobsExe)
			var wrks runner.Pool
			// Jobs
			if err := wrks.Start([]string{"php", *jobsExe}, 2, env); err != nil {
				log.Fatal("error starting: ", err)
			}
			defer wrks.Stop()
			jobsPool = jobs.New(&wrks)
			go jobsPool.Run()
		}
		go startRPC(*rpcAddr)
	}

	// HTTP
	if *httpExe != "" && *wrksNum > 0 {
		mustExist(*httpExe)
		wrks := runner.Pool{}
		if err := wrks.Start([]string{"php", *httpExe}, *wrksNum, env); err != nil {
			log.Fatal("error starting: ", err)
		}
		defer wrks.Stop()
		// Простая цепочка обработчиков: сначала пытаемся отдать
		// статический файл. При его отсутствии передаем запрос
		// PHP-приложению.
		handler := rhttp.NewStaticHandler(*static, *maxAge, *cors)
		timeout := time.Second * 30
		handler.Next(rhttp.NewWorkerHandler(
			&wrks, *cors, timeout, uint(*wrksNum) * 2,
		))
		http.Handle("/", handler)
	}

	// Websocket
	wsPool = websocket.NewPool()
	http.Handle("/ws", websocket.NewHandler(
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

	if redisAddr != nil && *redisAddr != "" {
		addr := *redisAddr
		if strings.HasPrefix(addr, "redis://") {
			addr, _ = strings.CutPrefix(addr, "redis://")
		}
		if strings.HasPrefix(addr, "tcp://") {
			addr, _ = strings.CutPrefix(addr, "tcp://")
		}
		addr = strings.Trim(addr, "/")

		// TODO: Вместо pub/sub лучше реализовать такой механизм:
		// https://panlw.github.io/15459021437244.html
		rListener, err := redis.Connect(addr)
		if err != nil {
			log.Fatalf("redis connection error: %s", err)
		}
		defer rListener.Close()
		err = rListener.PSubscribe("chat:*")
		if err != nil {
			log.Fatalf("redis PSubscribe error: %s", err)
		}
		go func() {
			for {
				res, err := rListener.ReadResponse()
				if err != nil {
					log.Println("redis error:", err)
				}
				if len(res) != 4 && res[0] != "pmessage" {
					continue
				}
				topic := res[2]
				if len(topic) < 6 {
					continue
				}
				payload := res[3]
				wsPool.Publish(topic[5:], []byte(payload), "")
			}
		}()
		log.Println("redis: listening to " + addr)
	}

	if *jobsExe != "" {
		log.Println("jobs: waiting for requests")
	}
	if *rpcAddr != "" {
		log.Printf("rpc: listening on %s", *rpcAddr)
	}
	if *static != "" {
		log.Printf(
			`http: serving files statically from directory "%s"; CORS: %t; Max-Age: %d`,
			*static, *cors, *maxAge,
		)
	}
	if *httpExe != "" {
		log.Printf(`http: serving PHP application "%s"`, *httpExe)
	}
	log.Println("http: listening on " + *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
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

func (r *RPCHandler) RunJob(args []any, reply *bool) error {
	// Первый аргумент -- название фоновой работы, второй -- payload.
	req := runner.JobRequest{}
	req.Name = args[0].(string)
	req.Payload = []byte(args[1].(string))
	req.Timeout = uint64(args[2].(float64))
	jobsPool.Queue(&req)
	*reply = true
	return nil
}

func mustExist(file string) {
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file \"%s\" does not exist", file)
	}
}
