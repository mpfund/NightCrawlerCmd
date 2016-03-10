package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/BlackEspresso/crawlbase"
)

var fileStorageUrl string = ""

type crawlSettings struct {
	Url           *url.URL
	FileStoreUrl  string
	WaitTime      int
	MaxPages      int
	StorageFolder string
	ReportFile    string
}

type PageReport struct {
	URL          string
	FileName     string
	RespDuration int
	StatusCode   int
	Location     string
	TextUrl      []string
	Error        string
}

/* usage examples:
nightcrawler.exe -url http://www.google.com
=> starts crawl from site http://www.google.com, only sites with same host (google.com)
saves files to ./storage

nightcrawler.exe -report test.csv
=> just generates reports from prev. crawls files stored in ./storage. All urls.

nightcrawler.exe -url http://www.google.com -report test.csv
=> starts crawl http://www.google.com and generate report for url in the end

nightcrawler.exe -url http://www.google.com -report test.csv -nocrawl
=> just generate report for url

*/
func main() {
	urlFlag := flag.String("url", "", "url, e.g. http://www.google.com")
	//urlRegEx := flag.String("regex", "", "only crawl links using this regex")
	waitFlag := flag.Int("wait", 1000, "delay, in milliseconds, default is 1000ms=1sec")
	maxPagesFlag := flag.Int("maxpages", -1, "max pages to crawl, -1 for infinite (default)")
	flag.String("storagetype", "file", "type of storage. (http,file,ftp)")
	storagePathFlag := flag.String("storagepath", "./storage", "folder to store crawled files")
	reportFile := flag.String("report", "", "generate report")
	noCrawlFlag := flag.Bool("nocrawl", false, "skips crawling. Can be used for reporting")
	flag.Parse()

	if *urlFlag == "" && *reportFile == "" {
		log.Fatal("no url or report file provided.")
	}

	logf, err := os.OpenFile("nightcrawler.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(io.MultiWriter(logf, os.Stdout))
	defer logf.Close()

	settings := crawlSettings{}
	settings.WaitTime = *waitFlag
	settings.MaxPages = *maxPagesFlag
	settings.ReportFile = *reportFile
	settings.StorageFolder = *storagePathFlag

	// html validator settings
	tags, err := crawlbase.LoadTagsFromFile("tags.json")
	if err != nil {
		log.Fatal(err)
	}

	cw := crawlbase.NewCrawler()
	cw.Validator.AddValidTags(tags)
	cw.IncludeHiddenLinks = false
	cw.WaitBetweenRequests = settings.WaitTime

	// resume
	pagesLoaded, err := cw.LoadPages(settings.StorageFolder)
	if err != nil {
		log.Fatal("Loaded pages  error: ", err)
	}
	log.Println("Loaded pages: ", pagesLoaded)

	var baseUrl *url.URL = nil

	if *urlFlag != "" {
		// parse url & remove all out of scope urls
		baseUrl, err = url.Parse(*urlFlag)
		checkError(err)
		cw.RemoveLinksNotSameHost(baseUrl)
		settings.Url = baseUrl
	}

	if baseUrl != nil && !(*noCrawlFlag) {
		cw.BeforeCrawlFn = func(url string) (string, error) {
			if settings.MaxPages >= 0 && cw.PageCount >= uint64(settings.MaxPages) {
				log.Println("crawled ", cw.PageCount, "link(s), max pages reached.")
				return "", errors.New("max pages reached")
			}
			return url, nil
		}

		cw.FetchSites(baseUrl)
	}

	if settings.ReportFile != "" {
		generateReport(&settings)
	}
}

func checkError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func generateReport(settings *crawlSettings) {
	f, err := os.Create(settings.ReportFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	files, err := crawlbase.GetPageInfoFiles(settings.StorageFolder)
	if err != nil {
		log.Fatal(err)
	}

	pageReports := map[string]*PageReport{}
	links := map[string]bool{}
	usedUrlQueryKeys := map[string]bool{}

	for _, k := range files {
		page, err := crawlbase.LoadPage(k, false)
		if err != nil {
			log.Fatal(err)
		}

		pr := &PageReport{}
		pr.RespDuration = page.RespDuration
		pr.FileName = strconv.Itoa(page.CrawlTime)
		pr.URL = page.URL
		pr.StatusCode = page.Response.StatusCode
		pr.Location = ""
		pr.TextUrl = page.RespInfo.TextUrls
		pr.Error = page.Error

		isRedirect, location := crawlbase.LocationFromPage(page)
		if isRedirect {
			pr.Location = location
		}

		pUrl, _ := url.Parse(page.URL)
		for v, _ := range pUrl.Query() {
			usedUrlQueryKeys[v] = false
		}

		pageReports[page.URL] = pr
		for _, href := range page.RespInfo.Hrefs {
			_, hasUrl := pageReports[href]
			if !hasUrl {
				links[href] = false
			}
		}
	}

	w := csv.NewWriter(f)

	w.Write([]string{"crawled links"})
	w.Write([]string{"timestamp", "url", "Http code", "duration (ms)", "redirect url", "error"})

	for _, info := range pageReports {
		dur := info.RespDuration
		w.Write([]string{
			info.FileName,
			info.URL,
			strconv.Itoa(info.StatusCode),
			strconv.Itoa(dur),
			info.Location,
			info.Error,
		})
	}
	w.Write([]string{})
	w.Write([]string{"used query keys"})

	for k, _ := range usedUrlQueryKeys {
		w.Write([]string{k})
	}

	textUrls := map[string]bool{}

	for _, p := range pageReports {
		for _, u := range p.TextUrl {
			textUrls[u] = false
		}
	}

	for _, k := range pageReports {
		delete(textUrls, k.URL)
	}

	textUrlsArr := []string{}

	for u, _ := range textUrls {
		textUrlsArr = append(textUrlsArr, u)
	}

	sort.Strings(textUrlsArr)

	w.Write([]string{})
	w.Write([]string{"found text urls"})

	for _, u := range textUrlsArr {
		w.Write([]string{u})
	}

	w.Flush()
}

func saveCrawlHttp(crawledUri string, fileName string, content []byte) {
	params := map[string]string{"meta": crawledUri}

	req, err := newfileUploadRequest(fileStorageUrl, params, "upload", fileName, content)
	if err != nil {
		log.Fatal("cant create file store request ", err)
	}

	c := http.Client{}
	c.Timeout = time.Duration(200) * time.Second

	uploadSuccess := false
	for retries := 0; retries < 3; retries++ {
		cresp, err := c.Do(req)
		if err != nil {
			log.Println("file store", err)
			continue
		}
		if cresp.StatusCode != 200 {
			log.Println("file store response ", cresp.StatusCode)
			continue
		}
		uploadSuccess = true
		break
	}

	if !uploadSuccess {
		log.Println("error while saving")
		log.Println(fileName, len(content), fileStorageUrl)
		log.Fatal("exiting")
	}
}

func newfileUploadRequest(uri string, params map[string]string, paramName string, fName string,
	fileContent []byte) (*http.Request, error) {

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, fName)
	if err != nil {
		return nil, err
	}
	part.Write(fileContent)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}
