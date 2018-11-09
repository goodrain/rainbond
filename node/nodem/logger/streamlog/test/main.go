package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
	"yiyun/common/log"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/streamlog"
	"github.com/pborman/uuid"
)

func main() {
	address := "127.0.0.1:6362"
	if len(os.Args) > 1 {
		address = os.Args[1]
	}

	var logs []logger.Logger
	for i := 0; i < 20; i++ {
		log, err := streamlog.New(logger.Context{
			ContainerID:  uuid.New(),
			ContainerEnv: []string{"TENANT_ID=" + uuid.New(), "SERVICE_ID=" + uuid.New()},
			Config:       map[string]string{"stream-server": address},
		})
		if err != nil {
			logrus.Error(err)
			return
		}
		go work(log)
		logs = append(logs, log)
	}
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		log.Warn("Received SIGTERM, exiting gracefully...")
	}
	for _, l := range logs {
		l.Close()
	}
}

func work(log logger.Logger) {
	for i := 0; i < 1; i++ {
		fi, err := os.Open("./log.txt")
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
		defer fi.Close()
		br := bufio.NewReader(fi)
		for {
			a, _, c := br.ReadLine()
			if c == io.EOF {
				break
			}
			err := log.Log(&logger.Message{
				Line:      a,
				Timestamp: time.Now(),
				Source:    "stdout",
			})
			if err != nil {
				return
			}
		}
	}

}
