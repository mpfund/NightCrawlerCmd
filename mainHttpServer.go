package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func mainHttpServer() {
	fs := flag.NewFlagSet("httpserver", flag.ExitOnError)

	listenTo := fs.String("listen", ":8080", "listen on ip:port")
	folder := fs.String("folder", "./", "root folder")

	fs.Parse(os.Args[2:])
	fmt.Println("Listening on " + *listenTo + " serving folder " + *folder)
	fmt.Println("Press CTRL+C to exit")

	log.Fatal(http.ListenAndServe(*listenTo, http.FileServer(http.Dir(*folder))))
}
