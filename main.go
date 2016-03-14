package main

import (
	"bytes"
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
	"github.com/BlackEspresso/htmlcheck"
	"github.com/tealeg/xlsx"
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
	URL               string
	FileName          string
	RespDuration      int
	StatusCode        int
	Location          string
	TextUrl           []string
	Error             string
	InvalidTags       []string
	InvalidAttributes []string
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
	clearStorageFlag := flag.Bool("clearstorage", false, "delete all storage files")
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

	cw := crawlbase.NewCrawler()
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

	if *clearStorageFlag {
		log.Println("delete storage files")
		clearStorage(&settings)
	}
}

func clearStorage(settings *crawlSettings) {
	files, err := crawlbase.GetPageInfoFiles(settings.StorageFolder)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		os.Remove(f)
	}
}

func checkError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func generateReport(settings *crawlSettings) {
	file := xlsx.NewFile()
	sheetUrls, err := file.AddSheet("Crawled Urls")
	if err != nil {
		log.Fatal(err)
	}

	files, err := crawlbase.GetPageInfoFiles(settings.StorageFolder)
	if err != nil {
		log.Fatal(err)
	}

	pageReports := map[string]*PageReport{}
	links := map[string]bool{}
	usedUrlQueryKeys := map[string]bool{}

	vdtr := htmlcheck.Validator{}
	tags, err := crawlbase.LoadTagsFromFile("tags.json")
	if err != nil {
		log.Fatal(err)
	}

	vdtr.AddValidTags(tags)

	for _, k := range files {
		page, err := crawlbase.LoadPage(k, true)
		if err != nil {
			log.Fatal(err)
		}

		pr := &PageReport{}
		pr.RespDuration = page.RespDuration
		pr.FileName = strconv.Itoa(page.CrawlTime)
		pr.URL = page.URL
		pr.Location = ""
		pr.InvalidTags = []string{}
		pr.InvalidAttributes = []string{}

		body := string(page.ResponseBody)
		pr.TextUrl = crawlbase.GetUrlsFromText(body)
		pr.Error = page.Error

		if page.Response != nil {
			pr.StatusCode = page.Response.StatusCode

			mime := crawlbase.GetContentMime(page.Response.Header)
			if mime == "text/html" {
				vErros := vdtr.ValidateHtmlString(body)
				pr.InvalidTags = findInvalidHtmlByType(vErros,
					htmlcheck.InvTag)
				pr.InvalidAttributes = findInvalidHtmlByType(vErros,
					htmlcheck.InvAttribute)
			}
		}

		pUrl, err := url.Parse(page.URL)
		if err != nil {
			log.Println("url invalid, skipping", page.URL)
			continue
		}

		if page.Response != nil {
			isRedirect, location := crawlbase.LocationFromPage(page, pUrl)
			if isRedirect {
				pr.Location = location
			}
		}

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

	row := sheetUrls.AddRow()
	row.WriteSlice(&[]string{"timestamp", "url", "Http code", "duration (ms)", "redirect url", "error"}, -1)

	for _, info := range pageReports {
		dur := info.RespDuration
		row = sheetUrls.AddRow()
		row.WriteSlice(&[]string{
			info.FileName,
			info.URL,
			strconv.Itoa(info.StatusCode),
			strconv.Itoa(dur),
			info.Location,
			info.Error,
		}, -1)
	}

	sQueryKeys, _ := file.AddSheet("query keys")
	for k, _ := range usedUrlQueryKeys {
		row = sQueryKeys.AddRow()
		row.WriteSlice(&[]string{k}, -1)
	}

	sInvTags, _ := file.AddSheet("invalid tags")
	for _, info := range pageReports {
		if len(info.InvalidTags) > 0 || len(info.InvalidAttributes) > 0 {
			row = sInvTags.AddRow()
			row.WriteSlice(&[]string{
				info.FileName,
				info.URL,
			}, -1)

			for _, inv := range info.InvalidTags {
				row = sInvTags.AddRow()
				row.WriteSlice(&[]string{"tag", inv}, -1)
			}
			for _, inv := range info.InvalidAttributes {
				row = sInvTags.AddRow()
				row.WriteSlice(&[]string{"attr", inv}, -1)
			}
		}
	}

	// text urls
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

	sheetTextUrls, _ := file.AddSheet("text urls")

	for _, u := range textUrlsArr {
		row = sheetTextUrls.AddRow()
		row.WriteSlice(&[]string{u}, -1)
	}

	err = file.Save(settings.ReportFile)
	if err != nil {
		log.Fatal(err)
	}
}

func findInvalidHtmlByType(validations []*htmlcheck.ValidationError,
	reason htmlcheck.ErrorReason) []string {

	list := []string{}
	for _, k := range validations {
		if k.Reason == reason {
			col := strconv.Itoa(k.TextPos.Column)
			line := strconv.Itoa(k.TextPos.Line)
			attr := k.AttributeName
			list = append(list, "<"+k.TagName+"> "+attr+" ("+col+", "+line+")")
		}

	}
	return list
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
