package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args
	if len(args) == 1 {
		fmt.Println("NCrawler V0.1.3")
		fmt.Println("missing tool command")
		fmt.Println("dns, crawler, report, portscan, wordlist, curl, httpscan, fuzzer, httpserver, bucketscan")
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
	} else if args[1] == "httppipe" {
		mainHttpCurl()
	} else if args[1] == "httpscan" {
		mainHTTPScan()
	} else if args[1] == "fuzzer" {
		mainFuzzer()
	} else if args[1] == "httpserver" {
		mainHttpServer()
	} else if args[1] == "wordlist" {
		mainWordList()
	} else if args[1] == "bucketscan" {
		mainBucketScan()
	} else {
		fmt.Println("tool " + args[1] + " not found")
	}
}
