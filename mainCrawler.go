package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"runtime/pprof"
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
	Profile       bool
	ProfileFolder string
}

type PageReport struct {
	URL               string
	FileName          string
	RespDuration      int
	StatusCode        int
	Location          string
	TextUrl           [][]byte
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

var DebugMode bool = false

func mainCrawler() {
	fs := flag.NewFlagSet("crawler", flag.ExitOnError)

	urlFlag := fs.String("url", "", "url, e.g. http://www.google.com")
	//urlRegEx := flag.String("regex", "", "only crawl links using this regex")
	waitFlag := fs.Int("wait", 1000, "delay, in milliseconds (default is 1000ms=1sec)")
	maxPagesFlag := fs.Int("max-pages", -1, "max pages to crawl, -1 for infinite (default is -1)")
	//fs.String("storageType", "file", "type of storage. (http,file,ftp)")
	storagePathFlag := fs.String("storage-path", "./storage", "folder to store crawled files")
	reportFile := fs.String("report", "", "generates report (xlsx-File)")
	noCrawlFlag := fs.Bool("no-crawl", false, "skips crawling. (used for reporting)")
	clearStorageFlag := fs.Bool("clear-storage", false, "delete all storage files")
	profiling := fs.Bool("profiling", false, "enable profiling")
	debugFlag := fs.Bool("debug", false, "enable debugging")
	urlList := fs.String("urllist", "", "path to list with urls")
	noNewLinks := fs.Bool("no-new-links", false, "dont crawl hrefs links.")

	DebugMode = *debugFlag

	fs.Parse(os.Args[2:])

	if *urlFlag == "" && *reportFile == "" && *urlList == "" {
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
	settings.Profile = *profiling
	settings.ProfileFolder = "./profiling/"

	cw := crawlbase.NewCrawler()
	cw.WaitBetweenRequests = settings.WaitTime
	cw.StorageFolder = settings.StorageFolder
	cw.NoNewLinks = *noNewLinks

	// resume
	if doesExists, _ := exists(settings.StorageFolder); !doesExists {
		os.Mkdir(settings.StorageFolder, 0777)
	}

	pagesLoaded, err := cw.LoadPages(settings.StorageFolder)
	checkError(err)

	log.Println("Loaded pages: ", pagesLoaded)

	var baseUrl *url.URL = nil

	if *urlFlag != "" {
		// parse url & remove all out of scope urls
		baseUrl, err = url.Parse(*urlFlag)
		checkError(err)
		cw.RemoveLinksNotSameHost(baseUrl)
		settings.Url = baseUrl
	}

	if *noNewLinks {
		// set all to crawled
		for k := range cw.Links {
			cw.Links[k] = true
		}
	}

	if *urlList != "" {
		data, err := ioutil.ReadFile(*urlList)
		checkError(err)
		lines := SplitByLines(string(data))
		newUrls := []string{}
		for _, l := range lines {
			if baseUrl != nil {
				// use relative & absolute urls
				absUrl := crawlbase.ToAbsUrl(baseUrl, l)
				newUrls = append(newUrls, absUrl)
			} else {
				// add only absolute ones
				newUrl, err := url.Parse(l)
				checkError(err)
				if newUrl.IsAbs() {
					newUrls = append(newUrls, l)
				}
			}
		}

		cw.AddAllLinks(newUrls)
		if baseUrl != nil {
			cw.RemoveLinksNotSameHost(baseUrl)
		}
	}

	cw.BeforeCrawlFn = func(url string) (string, error) {
		if settings.MaxPages >= 0 && cw.PageCount >= uint64(settings.MaxPages) {
			log.Println("crawled ", cw.PageCount, "link(s), max pages reached.")
			return "", errors.New("max pages reached")
		}
		return url, nil
	}

	if baseUrl != nil && !(*noCrawlFlag) {
		cw.FetchSites(baseUrl)
	} else if *urlList != "" && !(*noCrawlFlag) {
		cw.FetchSites(nil)
	}

	if settings.ReportFile != "" {
		generateReport(&settings)
	}

	if *clearStorageFlag {
		log.Println("delete storage files")
		clearStorage(&settings)
	}
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func clearStorage(settings *crawlSettings) {
	files, err := crawlbase.GetPageInfoFiles(settings.StorageFolder)
	checkError(err)
	for _, f := range files {
		os.Remove(f)
	}
}

func writeHeap(path, num string) {
	folder := path
	_, err := os.Stat(folder)
	if err != nil {
		err = os.Mkdir(folder, 0777)
		checkError(err)
	}

	f, err := os.Create(folder + "/heap_" + num + ".pprof")
	checkError(err)
	pprof.WriteHeapProfile(f)
	f.Close()
}

func logPrint(err error) {
	if err != nil {
		log.Print(err)
	}
}

func generateReport(settings *crawlSettings) {
	startTime := time.Now()

	file := xlsx.NewFile()
	sheetUrls, err := file.AddSheet("Crawled Urls")
	checkError(err)

	files, err := crawlbase.GetPageInfoFiles(settings.StorageFolder)
	checkError(err)

	pageReports := map[string]*PageReport{}
	links := map[string]bool{}
	usedUrlQueryKeys := map[string]string{}

	vdtr := htmlcheck.Validator{}
	tags, err := crawlbase.LoadTagsFromFile("tags.json")
	checkError(err)

	vdtr.AddValidTags(tags)

	if settings.Profile {
		f, err := os.Create(settings.ProfileFolder + "cpuprofile.pprof")
		checkError(err)
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	for _, k := range files {
		page, err := crawlbase.LoadPage(k, true)
		checkError(err)

		pr := &PageReport{}
		pr.RespDuration = page.RespDuration
		pr.FileName = strconv.Itoa(page.CrawlTime)
		pr.URL = page.URL
		pr.Location = ""
		pr.InvalidTags = []string{}
		pr.InvalidAttributes = []string{}

		pr.TextUrl = crawlbase.GetUrlsFromText(page.ResponseBody, 100)
		pr.Error = page.Error

		if page.Response != nil {
			pr.StatusCode = page.Response.StatusCode

			mime := crawlbase.GetContentMime(page.Response.Header)

			if mime == "text/html" {
				body := string(page.ResponseBody)
				vErros := vdtr.ValidateHtmlString(body)

				invs := filterInvalidHtmlByType(vErros, htmlcheck.InvTag, 10)
				htmlcheck.GetErrorLines(body, invs)
				pr.InvalidTags = validationErrorToText(invs)

				invs = filterInvalidHtmlByType(vErros, htmlcheck.InvAttribute, 10)
				htmlcheck.GetErrorLines(body, invs)
				pr.InvalidAttributes = validationErrorToText(invs)
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
			usedUrlQueryKeys[v] = pUrl.String()
		}

		pageReports[page.URL] = pr
		for _, href := range page.RespInfo.Hrefs {
			_, hasUrl := pageReports[href]
			if !hasUrl {
				links[href] = false
			}
		}

		// free page body
		page.ResponseBody = []byte{}
	}

	if settings.Profile {
		log.Println("loaded content in ", time.Now().Sub(startTime))
		writeHeap(settings.ProfileFolder, "0")
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
	for k, v := range usedUrlQueryKeys {
		row = sQueryKeys.AddRow()
		row.WriteSlice(&[]string{k, v}, -1)
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
	textUrls := map[string]string{}

	for _, p := range pageReports {
		for _, u := range p.TextUrl {
			textUrls[string(u)] = p.URL
		}
	}

	// removed crawled urls, keep only new, uncralwed ones
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
		row.WriteSlice(&[]string{u, textUrls[u]}, -1)
	}

	err = file.Save(settings.ReportFile)
	checkError(err)

	if settings.Profile {
		log.Println("report generated in ", time.Now().Sub(startTime))
	}
}

func validationErrorToText(validations []*htmlcheck.ValidationError) []string {
	list := []string{}
	for _, k := range validations {
		col := strconv.Itoa(k.TextPos.Column)
		line := strconv.Itoa(k.TextPos.Line)
		attr := k.AttributeName
		list = append(list, "<"+k.TagName+"> "+attr+" ("+line+", "+col+")")
	}
	return list
}

func filterInvalidHtmlByType(validations []*htmlcheck.ValidationError,
	reason htmlcheck.ErrorReason, max int) []*htmlcheck.ValidationError {

	errors := []*htmlcheck.ValidationError{}
	c := 0
	for _, k := range validations {
		if k.Reason == reason {
			errors = append(errors, k)
		}

		if c > max {
			break
		}
	}

	return errors
}

func saveCrawlHttp(crawledUri string, fileName string, content []byte) {
	params := map[string]string{"meta": crawledUri}

	req, err := newfileUploadRequest(fileStorageUrl, params, "upload", fileName, content)
	if err != nil {
		log.Println("cant create file store request ")
		checkError(err)
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
