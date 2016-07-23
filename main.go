package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args
	if len(args) == 1 {
		fmt.Println("missing tool command")
		fmt.Println("dns, crawler, report, portscan, httpscan, fuzzer, ...")
		return
	}
	if args[1] == "dns" {
		mainDNS()
	} else if args[1] == "crawler" {
		mainCrawler()
	} else if args[1] == "report" {
		mainReport()
	} else if args[1] == "portscan" {
		mainPortScan()
	} else if args[1] == "curl" {
		mainHttpCurl()
	} else if args[1] == "httpscan" {
		mainHttpScan()
	} else if args[1] == "fuzzer" {
		mainFuzzer()
	} else if args[1] == "httpserver" {
		mainHttpServer()
	} else {
		fmt.Println("tool " + args[1] + " not found")
	}
}
