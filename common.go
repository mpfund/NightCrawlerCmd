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
