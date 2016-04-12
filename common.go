package main

import (
	"log"
)

func checkError(e error) {
	if e != nil && DebugMode {
		panic(e)
		return
	}
	if e != nil {
		log.Fatal(e)
	}
}
