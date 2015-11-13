package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/goblin-ci/dispatch"
	"github.com/gorilla/websocket"
	"gopkg.in/redis.v3"
)

// ConnectionRequest represents ws connection payload
type ConnectionRequest struct {
	Type  string `json:"type"`
	Repo  string `json:"repo"`
	Token string `json:"token"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var conn map[string]*websocket.Conn

func init() {
	conn = make(map[string]*websocket.Conn)
}

func main() {
	mq, err := dispatch.NewRedis("redis:6379")
	if err != nil {
		log.Fatal(err)
	}

	stop := make(chan bool, 2)
	duration := time.Second * 1
	recv, err := mq.Subscribe("build_queue_update", stop, duration)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		stop <- true
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			select {
			case payload, ok := <-recv:
				if !ok {
					recv = nil
					break
				}
				go func() {
					msg := payload.(*redis.Message)
					dispatchMsg := new(dispatch.Message)
					s, _ := strconv.Unquote(msg.Payload)
					json.Unmarshal([]byte(s), &dispatchMsg)

					fmt.Println(dispatchMsg)

					if c, ok := conn[dispatchMsg.Repo]; ok {
						c.WriteMessage(websocket.TextMessage, []byte(s))
					}
				}()
			case <-sig:
				log.Println("OS Signal received, exiting...")
				recv = nil
			}

			if recv == nil {
				os.Exit(0)
				break
			}
		}
	}()

	http.HandleFunc("/", handle)
	log.Fatal(http.ListenAndServe(":8888", nil))
}

func handle(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		if mt == websocket.TextMessage {
			cr := new(ConnectionRequest)
			err = json.Unmarshal(message, cr)
			if err != nil {
				// TODO Respond with invalid JSON error
				break
			}

			conn[cr.Repo] = c

			// TODO Respond with status message
		}

		/*log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}*/
	}

	// TODO Remove connection from conn map
	log.Println("Client closed connection")
}
