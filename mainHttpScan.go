package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type appScannerSettings struct {
	InputFile  string
	ReportFile string
	Host       string
	Scheme     string
	VectorFile string
}

type appScan struct {
	BaseResponse *http.Response
}

type AttackVector struct {
	Vector       string
	Test         string
	SqlInjection bool
}

func mainHttpScan() {
	fs := flag.NewFlagSet("httpscan", flag.ExitOnError)

	inputFile := fs.String("input", "", "input file")
	hostFlag := fs.String("host", "", "set host")
	schemeFlag := fs.String("scheme", "", "set scheme (http, https, ...)")
	report := fs.String("report", "", "report file")
	vectorFile := fs.String("vectors", "vectors.json", "file with attack vectors")

	fs.Parse(os.Args[2:])

	settings := appScannerSettings{}
	settings.InputFile = *inputFile
	settings.ReportFile = *report
	settings.Host = *hostFlag
	settings.Scheme = *schemeFlag
	settings.VectorFile = *vectorFile

	req := getRequest(&settings)

	scan := appScan{}
	var vectors []*AttackVector

	var err error
	scan.BaseResponse, err = http.DefaultClient.Do(req)
	checkError(err)

	data, err := ioutil.ReadFile(settings.VectorFile)
	checkError(err)
	err = json.Unmarshal(data, &vectors)
	checkError(err)

	scanUrl(req, vectors)
}

func scanUrl(baseRequest *http.Request, vectors []*AttackVector) {
	bQueries := baseRequest.URL.Query()
	fmt.Println(bQueries)
	for key, _ := range bQueries {
		for _, vec := range vectors {
			req := *baseRequest
			queries := baseRequest.URL.Query()
			queries.Set(key, vec.Vector)

			req.URL.RawQuery = queries.Encode()
			fmt.Println(key, req.URL)
			resp, _ := http.DefaultClient.Do(&req)
			bodyData, err := ioutil.ReadAll(resp.Body)
			testVector := vec.Test
			if testVector == "" {
				testVector = vec.Vector
			}
			index := strings.Index(string(bodyData), testVector)
			fmt.Println(resp.Status, len(bodyData), index, err)
		}
	}
}

func getRequest(settings *appScannerSettings) *http.Request {
	req := readHttpRequest(settings.InputFile)
	if settings.Host != "" {
		req.URL.Host = settings.Host
	}
	if settings.Scheme != "" {
		req.URL.Scheme = settings.Scheme
	}
	return req
}
