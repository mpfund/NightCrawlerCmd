package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/BlackEspresso/crawlbase"
)

type appScannerSettings struct {
	InputFile       string
	ReportFile      string
	Host            string
	Scheme          string
	VectorFile      string
	URL             string
	ScanHTTPHeaders bool
	ReportTemplate  *template.Template
	outputFolder    string
}

type appScan struct {
	BaseResponse *http.Response
	BaseRequest  *http.Request
	Vectors      []*attackVector
}

type attackVector struct {
	Vector       string
	Test         string
	SQLInjection bool
	Section      string
}

type scanResult struct {
	AttackVector       *attackVector
	Duration           int
	Request            *crawlbase.PageRequest
	Response           *crawlbase.PageResponse
	Error              string
	Found              bool
	ResponseBodyLength int
	URL                string
	ParamTarget        string
	FilePath           string
}

func mainHTTPScan() {
	fs := flag.NewFlagSet("httpscan", flag.ExitOnError)

	inputFile := fs.String("input", "", "input file")
	hostFlag := fs.String("host", "", "set host")
	schemeFlag := fs.String("scheme", "", "set scheme (http, https, ...)")
	report := fs.String("report", "report.html", "report file")
	vectorFile := fs.String("vectors", "vectors.json", "file with attack vectors")
	urlFlag := fs.String("url", "", "url instead of input file")
	scanHeaderFlag := fs.Bool("scanheader", false, "scan HTTP headers, too")
	outputFolder := fs.String("output", "", "url instead of input file")

	fs.Parse(os.Args[2:])

	tmpl, err := template.ParseFiles("./template/httpscanresult.tmpl")
	checkError(err)

	settings := &appScannerSettings{}
	settings.ReportTemplate = tmpl
	settings.InputFile = *inputFile
	settings.ReportFile = *report
	settings.Host = *hostFlag
	settings.Scheme = *schemeFlag
	settings.VectorFile = *vectorFile
	settings.URL = *urlFlag
	settings.ScanHTTPHeaders = *scanHeaderFlag
	settings.outputFolder = *outputFolder

	scan := new(appScan)

	req := getRequest(settings)
	scan.BaseRequest = req
	baseResult := doRequest(settings, scan.BaseRequest, &attackVector{}, "BaseRequest")

	data, err := ioutil.ReadFile(settings.VectorFile)
	checkError(err)
	err = json.Unmarshal(data, &scan.Vectors)
	checkError(err)

	results := []*scanResult{}
	results = append(results, baseResult)
	results = append(results, scanURL(settings, scan)...)
	generateScanReport(results, settings)
}

func generateScanReport(results []*scanResult, settings *appScannerSettings) {
	file, err := os.OpenFile(settings.ReportFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	checkError(err)
	defer file.Close()

	err = settings.ReportTemplate.Execute(file, results)
	checkError(err)
}

func scanURL(settings *appScannerSettings, scan *appScan) []*scanResult {
	bQueries := scan.BaseRequest.URL.Query()
	results := []*scanResult{}

	for key := range bQueries {
		for _, vec := range scan.Vectors {
			req := copyRequest(scan.BaseRequest)

			queries := req.URL.Query()
			queries.Set(key, vec.Vector)

			req.URL.RawQuery = queries.Encode()
			fmt.Println(key, req.URL)
			result := doRequest(settings, req, vec, "urlquery "+key)
			results = append(results, result)
		}
	}

	if settings.ScanHTTPHeaders {
		for key := range scan.BaseRequest.Header {
			for _, vec := range scan.Vectors {
				req := copyRequest(scan.BaseRequest)
				header := req.Header.Get(key)
				req.Header.Set(key, header+vec.Vector)
				result := doRequest(settings, req, vec, "header "+key)
				results = append(results, result)
			}
		}
	}

	segments := getPathSegements(scan.BaseRequest.URL.EscapedPath())

	for i := range segments {
		if segments[i] == "" {
			continue
		}

		for _, vec := range scan.Vectors {
			if vec.Section != "" &&
				strings.Index(vec.Section, "urlsegment") == -1 {
				continue
			}
			req := copyRequest(scan.BaseRequest)
			reqSegments := getPathSegements(req.URL.EscapedPath())
			reqSegments[i] = vec.Vector
			segs := strings.Join(reqSegments, "/")
			req.URL, _ = newURLPath(req.URL, segs)
			result := doRequest(settings, req, vec, "urlsegment "+segments[i])
			results = append(results, result)
		}
	}
	return results
}

func getPathSegements(path string) []string {
	return strings.Split(path, "/")
}

func newURLPath(u *url.URL, path string) (*url.URL, error) {
	nURLText := u.Scheme + "://" + u.Host + path + "?" + u.RawQuery +
		"#" + u.Fragment
	return u.Parse(nURLText)
}

func doRequest(settings *appScannerSettings, req *http.Request,
	vector *attackVector, paramTarget string) *scanResult {
	startTime := time.Now()
	resp, err := http.DefaultClient.Do(req)
	dur := time.Now().Sub(startTime)
	var bodyData []byte
	var testVector string
	if err == nil {
		bodyData, err = ioutil.ReadAll(resp.Body)
		testVector = vector.Test
		if testVector == "" {
			testVector = vector.Vector
		}
	}
	index := strings.Index(string(bodyData), testVector)

	filePath := ""
	if settings.outputFolder != "" {
		fileName := strconv.FormatInt(startTime.UnixNano(), 10)
		filePath = path.Join(settings.outputFolder, fileName)
		ioutil.WriteFile(filePath, bodyData, 0666)
	}

	result := requestToResult(resp, vector, filePath, dur, err,
		index >= 0, bodyData, paramTarget)
	return result
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

func requestToResult(resp *http.Response, vec *attackVector, filePath string,
	duration time.Duration, err error, found bool, body []byte, paramTarget string) *scanResult {
	result := new(scanResult)
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
		result.ParamTarget = paramTarget
		result.ResponseBodyLength = len(body)
		result.FilePath = filePath
		result.URL = resp.Request.URL.String()
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
		req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/50.0.2661.94 Safari/537.36")
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
