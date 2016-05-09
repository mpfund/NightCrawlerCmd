package main

import (
	"bufio"
	"flag"
	"net/http"
	"os"
)

type appScannerSettings struct {
	InputFile string
}

func mainHttpScan() {
	fs := flag.NewFlagSet("scanner", flag.ExitOnError)

	inputFile := fs.String("file", "", "input file")
	hostFlag := fs.String("host", "", "set host")
	schemeFlag := fs.String("set scheme", "", "set scheme (http, https, ...)")
	output := fs.String("output", "output.txt", "output file")

	fs.Parse(os.Args[2:])

	settings := appScannerSettings{}
	settings.InputFile = *inputFile

	req := readHttpRequest(settings.InputFile)
	if *hostFlag != "" {
		req.URL.Host = *hostFlag
	}
	if *schemeFlag != "" {
		req.URL.Scheme = *schemeFlag
	}

	resp, err := http.DefaultClient.Do(req)
	checkError(err)
	writeHttpResponse(*output, resp)
}

func writeHttpResponse(fileName string, resp *http.Response) {
	f, err := os.Create(fileName)
	checkError(err)
	defer f.Close()

	resp.Write(f)
}

func readHttpRequest(fileName string) *http.Request {
	f, err := os.Open(fileName)
	checkError(err)
	defer f.Close()
	bf := bufio.NewReader(f)
	req, err := http.ReadRequest(bf)
	checkError(err)
	req.RequestURI = ""

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	return req
}
