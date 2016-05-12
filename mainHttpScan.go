package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BlackEspresso/crawlbase"
	"github.com/tealeg/xlsx"
)

type appScannerSettings struct {
	InputFile  string
	ReportFile string
	Host       string
	Scheme     string
	VectorFile string
	URL        string
}

type appScan struct {
	BaseResponse *http.Response
}

type AttackVector struct {
	Vector       string
	Test         string
	SqlInjection bool
}

type ScanResult struct {
	AttackVector       *AttackVector
	Duration           int
	Request            *crawlbase.PageRequest
	Response           *crawlbase.PageResponse
	Error              string
	Found              bool
	ResponseBodyLength int
	Url                string
}

func mainHttpScan() {
	fs := flag.NewFlagSet("httpscan", flag.ExitOnError)

	inputFile := fs.String("input", "", "input file")
	hostFlag := fs.String("host", "", "set host")
	schemeFlag := fs.String("scheme", "", "set scheme (http, https, ...)")
	report := fs.String("report", "report.xlsx", "report file")
	vectorFile := fs.String("vectors", "vectors.json", "file with attack vectors")
	urlFlag := fs.String("url", "", "url instead of input file")

	fs.Parse(os.Args[2:])

	settings := &appScannerSettings{}
	settings.InputFile = *inputFile
	settings.ReportFile = *report
	settings.Host = *hostFlag
	settings.Scheme = *schemeFlag
	settings.VectorFile = *vectorFile
	settings.URL = *urlFlag

	req := getRequest(settings)

	scan := appScan{}
	var vectors []*AttackVector

	var err error
	timeStart := time.Now()
	scan.BaseResponse, err = http.DefaultClient.Do(req)
	checkError(err)
	body, _ := ioutil.ReadAll(scan.BaseResponse.Body)
	dur := time.Now().Sub(timeStart)
	baseResult := requestToResult(scan.BaseResponse, &AttackVector{},
		dur, err, false, body)

	data, err := ioutil.ReadFile(settings.VectorFile)
	checkError(err)
	err = json.Unmarshal(data, &vectors)
	checkError(err)

	results := []*ScanResult{}
	results = append(results, baseResult)
	results = append(results, scanUrl(req, vectors)...)
	generateScanReport(results, settings)
}

func generateScanReport(results []*ScanResult, settings *appScannerSettings) {
	file := xlsx.NewFile()
	sheetScan, err := file.AddSheet("Scanned Urls")
	checkError(err)

	row := sheetScan.AddRow()
	row.WriteSlice(&[]string{"Index", "Test", "Duration", "Status Code",
		"Body Length", "Error", "Found", "URL"}, -1)

	for i, result := range results {
		row = sheetScan.AddRow()
		if result.Response == nil {
			row.WriteSlice(&[]interface{}{i, result.AttackVector.Vector,
				result.Duration, -1,
				result.ResponseBodyLength, result.Error, result.Found,
				result.Url}, -1)
		} else {
			row.WriteSlice(&[]interface{}{i, result.AttackVector.Vector,
				result.Duration, result.Response.StatusCode,
				result.ResponseBodyLength, result.Error, result.Found,
				result.Url}, -1)
		}

	}
	err = file.Save(settings.ReportFile)
	if err != nil {
		logPrint(err)
	} else {
		return
	}
	err = file.Save("report2.xlsx")
	checkError(err)
}

func scanUrl(baseRequest *http.Request, vectors []*AttackVector) []*ScanResult {
	bQueries := baseRequest.URL.Query()
	fmt.Println(bQueries)
	results := []*ScanResult{}
	for key, _ := range bQueries {
		for _, vec := range vectors {
			req := copyRequest(baseRequest)

			queries := req.URL.Query()
			queries.Set(key, vec.Vector)

			req.URL.RawQuery = queries.Encode()
			fmt.Println(key, req.URL)
			startTime := time.Now()
			resp, err := http.DefaultClient.Do(req)
			dur := time.Now().Sub(startTime)
			var bodyData []byte
			var testVector string
			if err == nil {
				bodyData, err = ioutil.ReadAll(resp.Body)
				testVector = vec.Test
				if testVector == "" {
					testVector = vec.Vector
				}
			}
			index := strings.Index(string(bodyData), testVector)
			result := requestToResult(resp, vec, dur, err, index >= 0, bodyData)
			results = append(results, result)
		}
	}
	return results
}

func copyRequest(req *http.Request) *http.Request {
	buffer := new(bytes.Buffer)
	req.Write(buffer)
	newreq, err := http.ReadRequest(bufio.NewReader(buffer))
	checkError(err)
	newreq.URL.Host = req.URL.Host
	newreq.URL.Scheme = req.URL.Scheme
	newreq.RequestURI = ""
	return newreq
}

func requestToResult(resp *http.Response, vec *AttackVector,
	duration time.Duration, err error, found bool, body []byte) *ScanResult {
	result := new(ScanResult)
	result.Duration = int(duration.Seconds() * 1000)
	result.AttackVector = vec
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Request = new(crawlbase.PageRequest)
		result.Response = new(crawlbase.PageResponse)
		result.Request.Header = resp.Request.Header
		result.Request.ContentLength = resp.Request.ContentLength
		result.Request.Proto = resp.Request.Proto
		result.Response.ContentLength = resp.ContentLength
		result.Response.Header = resp.Header
		result.Response.Proto = resp.Proto
		result.Response.StatusCode = resp.StatusCode
		result.Found = found
		result.ResponseBodyLength = len(body)
		result.Url = resp.Request.URL.String()
	}
	return result
}

func getRequest(settings *appScannerSettings) *http.Request {
	var req *http.Request
	var err error
	if settings.InputFile != "" {
		req, err = readHttpRequest(settings.InputFile)
		checkError(err)
	} else {
		req, err = http.NewRequest("GET", settings.URL, nil)
		checkError(err)
	}
	if settings.Host != "" {
		req.URL.Host = settings.Host
	}
	if settings.Scheme != "" {
		req.URL.Scheme = settings.Scheme
	}
	return req
}
