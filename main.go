package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goblin-ci/dispatch"
	"gopkg.in/redis.v3"
)

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

	for {
		select {
		case payload, ok := <-recv:
			if !ok {
				recv = nil
				break
			}
			go func() {
				// TODO publish to ws
				msg := payload.(*redis.Message)
				fmt.Println(msg.Payload)
			}()
		case <-sig:
			log.Println("OS Signal received, exiting...")
			recv = nil
		}

		if recv == nil {
			break
		}
	}
}
