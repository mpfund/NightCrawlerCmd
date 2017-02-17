package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/fatih/color"
)

func mainHttpServer() {
	fs := flag.NewFlagSet("httpserver", flag.ExitOnError)

	listenTo := fs.String("listen", ":8080", "listen on ip:port")
	folder := fs.String("folder", "./", "root folder")

	fs.Parse(os.Args[2:])
	color.Green("Listening on " + *listenTo + " serving folder " + *folder)
	color.Green("Press CTRL+C to exit")

	log.Fatal(http.ListenAndServe(*listenTo, http.FileServer(http.Dir(*folder))))
}
