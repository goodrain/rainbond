package main

import "flag"

var (
	// Root for storage
	storage = flag.String("storage", "./dummy/root_storage", "Root for storage")

	// Host and port falco server
	host    = flag.String("host", "localhost:9073", "host:port for pavo server")
	version = flag.Bool("version", false, "App version")
	convert = flag.Bool("convert", false, "Use ImageMagick convert to create a thumbnail image")
)
