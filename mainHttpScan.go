package main

import (
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
}

func mainHttpScan() {
	fs := flag.NewFlagSet("httpscan", flag.ExitOnError)

	inputFile := fs.String("input", "", "input file")
	hostFlag := fs.String("host", "", "set host")
	schemeFlag := fs.String("scheme", "", "set scheme (http, https, ...)")
	report := fs.String("report", "report.xlsx", "report file")
	vectorFile := fs.String("vectors", "vectors.json", "file with attack vectors")

	fs.Parse(os.Args[2:])

	settings := &appScannerSettings{}
	settings.InputFile = *inputFile
	settings.ReportFile = *report
	settings.Host = *hostFlag
	settings.Scheme = *schemeFlag
	settings.VectorFile = *vectorFile

	req := getRequest(settings)

	scan := appScan{}
	var vectors []*AttackVector

	var err error
	timeStart := time.Now()
	scan.BaseResponse, err = http.DefaultClient.Do(req)
	body, _ := ioutil.ReadAll(scan.BaseResponse.Body)
	dur := time.Now().Sub(timeStart)
	checkError(err)
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
		"Body Length", "Error", "Found"}, -1)

	for i, result := range results {
		row = sheetScan.AddRow()
		row.WriteSlice(&[]interface{}{i, result.AttackVector.Vector,
			result.Duration, result.Response.StatusCode,
			result.ResponseBodyLength, result.Error, result.Found}, -1)
	}
	err = file.Save(settings.ReportFile)
	checkError(err)
}

func scanUrl(baseRequest *http.Request, vectors []*AttackVector) []*ScanResult {
	bQueries := baseRequest.URL.Query()
	fmt.Println(bQueries)
	results := []*ScanResult{}
	for key, _ := range bQueries {
		for _, vec := range vectors {
			req := *baseRequest
			queries := baseRequest.URL.Query()
			queries.Set(key, vec.Vector)

			req.URL.RawQuery = queries.Encode()
			fmt.Println(key, req.URL)

			startTime := time.Now()
			resp, _ := http.DefaultClient.Do(&req)
			dur := time.Now().Sub(startTime)

			bodyData, err := ioutil.ReadAll(resp.Body)
			testVector := vec.Test
			if testVector == "" {
				testVector = vec.Vector
			}
			index := strings.Index(string(bodyData), testVector)
			result := requestToResult(resp, vec, dur, err, index >= 0, bodyData)
			results = append(results, result)
		}
	}
	return results
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
	}
	return result
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
