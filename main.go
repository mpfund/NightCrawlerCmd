package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args
	if len(args) == 1 {
		fmt.Println("missing tool command")
		fmt.Println("dns, crawler, report, portscanner ...")
		return
	}
	if args[1] == "dns" {
		mainDNS()
	} else if args[1] == "crawler" {
		mainCrawler()
	} else if args[1] == "report" {
		mainReport()
	} else if args[1] == "portscanner" {
		mainPortScan()
	} else {
		fmt.Println("tool " + args[1] + " not found")
	}
}
