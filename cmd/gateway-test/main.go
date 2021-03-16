package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/gosuri/uitable"
)

func main() {
	if os.Args[1] == "tcp" {
		conn, err := net.Dial("tcp", "127.0.0.1:18081")
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		_, err = conn.Write([]byte("GET\r\n"))
		if err != nil {
			log.Fatal(err)
		}
		print(conn)
	} else {
		res, err := http.Get("http://127.0.0.1:18080/config/backends")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		if res.Body != nil {
			defer res.Body.Close()
			print(res.Body)
		}
	}
}

func print(reader io.Reader) {
	decoder := json.NewDecoder(reader)
	var backends []*model.Backend
	if err := decoder.Decode(&backends); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	table := uitable.New()
	table.Wrap = true // wrap columns
	for _, b := range backends {
		table.AddRow(b.Name, strings.Join(func() []string {
			var re []string
			for _, e := range b.Endpoints {
				re = append(re, fmt.Sprintf("%s:%s %d", e.Address, e.Port, e.Weight))
			}
			return re
		}(), ";"))
	}
	fmt.Println(table)
}
