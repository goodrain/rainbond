package streamlog

import (
	"bufio"
	"io"
	"os"
	"testing"
	"time"

	"fmt"

	"github.com/docker/docker/daemon/logger"
	"github.com/pborman/uuid"
)

func TestStreamLogBeak(t *testing.T) {

	log, err := New(logger.Context{
		ContainerID:  uuid.New(),
		ContainerEnv: []string{"TENANT_ID=" + uuid.New(), "SERVICE_ID=" + uuid.New()},
		Config:       map[string]string{"stream-server": "127.0.0.1:6362"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	fi, err := os.Open("./test/log.txt")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()
	defer log.Close()
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
	time.Sleep(10 * time.Second)
}

func TestStreamLog(t *testing.T) {

	log, err := New(logger.Context{
		ContainerID:  uuid.New(),
		ContainerEnv: []string{"TENANT_ID=" + uuid.New(), "SERVICE_ID=" + uuid.New()},
		Config:       map[string]string{"stream-server": "127.0.0.1:6362"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	for i := 0; i < 5000; i++ {
		err := log.Log(&logger.Message{
			Line:      []byte("hello word!hello word!hello word!hello word!hello word!hello word!asdasfmaksmfkasmfkamsmakmskamsdaskdaksdmaksmdkamsdkamsdkmaksdmaksdmkamsdkamsdkaksdakdmklamdlkamdsklmalksdmlkamsdlkamdlkamsdlkmalksmdlkadmlkam"),
			Timestamp: time.Now(),
			Source:    "stdout",
		})
		if err != nil {
			return
		}
		//time.Sleep(time.Millisecond)
	}
	//time.Sleep(10 * time.Second)
	log.Close()
}

func BenchmarkStreamLog(t *testing.B) {
	log, err := New(logger.Context{
		ContainerID:  uuid.New(),
		ContainerEnv: []string{"TENANT_ID=" + uuid.New(), "SERVICE_ID=" + uuid.New()},
		Config:       map[string]string{"stream-server": "127.0.0.1:5031"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < t.N; i++ {
		log.Log(&logger.Message{
			Line:      []byte("hello word"),
			Timestamp: time.Now(),
			Source:    "stdout",
		})
	}
}
