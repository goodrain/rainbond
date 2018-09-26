package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"gopkg.in/gotsunami/coquelicot.v1"
)

func main() {
	flag.Parse()
	if *version {
		fmt.Printf("version: %s\n", appVersion)
		return
	}

	s := coquelicot.NewStorage(*storage)
	s.Option(coquelicot.Convert(*convert))

	logger := log.New(os.Stdout, "", log.LstdFlags)

	routes := map[string]http.HandlerFunc{
		"/files":  s.UploadHandler,
		"/resume": s.ResumeHandler,
	}
	for path, handler := range routes {
		http.Handle(path, coquelicot.Adapt(http.HandlerFunc(handler),
			coquelicot.CORSMiddleware(),
			coquelicot.LogMiddleware(logger)),
		)
	}

	log.Printf("Storage place in: %s", s.StorageDir())
	log.Printf("Start server on %s", *host)
	log.Fatal(http.ListenAndServe(*host, nil))
}
