package main

import (
	"log"
)

func checkError(e error) {
	if e != nil && debugMode {
		panic(e)
	}
	if e != nil {
		log.Fatal(e)
	}
}

var verboseLevel = 0

func logVerbose(level int, v ...interface{}) {
	if level <= verboseLevel {
		log.Println(v)
	}
}
