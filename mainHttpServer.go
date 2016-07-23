package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

func mainHttpServer() {
	fs := flag.NewFlagSet("httpserver", flag.ExitOnError)

	listenTo := fs.String("listen", ":8080", "listen on ip:port")
	folder := fs.String("folder", "", "root folder")

	fs.Parse(os.Args[2:])

	log.Fatal(http.ListenAndServe(*listenTo, http.FileServer(http.Dir(*folder))))
}
