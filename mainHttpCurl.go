package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type appHttpCurlSettings struct {
	InputFile string
}

type stringslice []string

func (i *stringslice) String() string {
	return fmt.Sprintf("%s", *i)
}

// The second method is Set(value string) error
func (i *stringslice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func mainHttpCurl() {
	fs := flag.NewFlagSet("httpcurl", flag.ExitOnError)

	headers := stringslice{}

	inputFile := fs.String("input", "", "input file")
	hostFlag := fs.String("host", "", "set host")
	schemeFlag := fs.String("scheme", "", "set scheme (http, https, ...)")
	output := fs.String("output", "", "output file")
	fs.Var(&headers, "H", "header")

	fs.Parse(os.Args[2:])

	settings := appHttpCurlSettings{}
	settings.InputFile = *inputFile

	req, err := readHttpRequest(settings.InputFile)
	checkError(err)
	if *hostFlag != "" {
		req.URL.Host = *hostFlag
	}
	if *schemeFlag != "" {
		req.URL.Scheme = *schemeFlag
	}

	// apply headers
	for _, header := range headers {
		kv := strings.SplitN(header, ":", 2)
		if len(kv) > 1 {
			req.Header.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
		} else {
			req.Header.Set(strings.TrimSpace(kv[0]), "")
		}
	}

	resp, err := http.DefaultClient.Do(req)
	checkError(err)
	if *output != "" {
		writeHttpResponseToFile(*output, resp)
	} else {
		resp.Write(os.Stdout)
	}

}

func writeHttpResponseToFile(fileName string, resp *http.Response) {
	f, err := os.Create(fileName)
	checkError(err)
	defer f.Close()

	resp.Write(f)
}

func readHttpRequest(fileName string) (*http.Request, error) {
	f, err := os.Open(fileName)
	checkError(err)
	defer f.Close()
	bf := bufio.NewReader(f)
	req, err := http.ReadRequest(bf)
	if err != nil {
		return nil, err
	}
	req.RequestURI = ""

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	return req, nil
}
