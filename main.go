package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
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

func main() {
	urlFlag := flag.String("url", "", "url, e.g. http://www.google.com")
	//fileStorage := flag.String("filestore", "http://localhost:8079/file/7363a35f-f411-4751-96ec-2d19b5a22323", "url to filestore")
	waitFlag := flag.Int("wait", 1000, "delay, in milliseconds, default is 1000ms=1sec")
	maxPagesFlag := flag.Int("maxpages", -1, "max pages to crawl, -1 for infinite")
	//storageFolder
	reportFile := flag.String("report", "", "generate report")
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
	settings.StorageFolder = "./storage"

	if *urlFlag != "" {
		baseUrl, err := url.Parse(*urlFlag)
		checkError(err)

		settings.Url = baseUrl

		fetchSites(&settings)
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

func IsValidScheme(url *url.URL) bool {
	scheme := url.Scheme
	if scheme == "http" || scheme == "https" {
		return true
	} else {
		return false
	}
}

type PageReport struct {
	URL          string
	FileName     string
	RespDuration int
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

	cLinks := map[string]*PageReport{}
	links := map[string]bool{}

	usedUrlQueryKeys := ""

	for _, k := range files {
		page, err := crawlbase.LoadPage(k, false)
		if err != nil {
			log.Fatal(err)
		}

		pr := &PageReport{}
		pr.RespDuration = page.RespDuration
		pr.FileName = strconv.Itoa(page.CrawlTime)
		pr.URL = page.URL

		pUrl, _ := url.Parse(page.URL)
		for v, _ := range pUrl.Query() {
			usedUrlQueryKeys += v + ","
		}

		cLinks[page.URL] = pr
		for _, href := range page.RespInfo.Hrefs {
			_, hasUrl := cLinks[href]
			if !hasUrl {
				links[href] = false
			}
		}
	}

	w := csv.NewWriter(f)

	w.Write([]string{"crawled links"})
	w.Write([]string{"FileName", "URL", "Duration (ms)", "Queries"})

	for _, info := range cLinks {
		dur := info.RespDuration
		w.Write([]string{
			info.FileName,
			info.URL,
			strconv.Itoa(dur),
		})
	}
	w.Write([]string{})
	w.Write([]string{"used query keys"})
	w.Write([]string{usedUrlQueryKeys})

	w.Flush()
}

func fetchSites(settings *crawlSettings) *crawlbase.Crawler {
	cw := crawlbase.NewCrawler()

	// html validator settings
	tags, err := crawlbase.LoadTagsFromFile("tags.json")
	if err != nil {
		log.Fatal(err)
	}
	cw.Validator.AddValidTags(tags)

	//resume
	cw.Links[settings.Url.String()] = false // startsite
	pagesLoaded, err := cw.LoadPages(settings.StorageFolder)
	if err != nil {
		log.Fatal("Loaded pages  error: ", err)
	}
	log.Println("Loaded pages: ", pagesLoaded)

	cw.IncludeHiddenLinks = false
	crawlCount := uint64(0)
	cw.WaitBetweenRequests = settings.WaitTime

	for {
		urlStr, found := cw.GetNextLink()
		if !found {
			log.Println("crawled ", crawlCount, "link(s). all links done.")
			return cw // done
		}
		if settings.MaxPages >= 0 && crawlCount >= uint64(settings.MaxPages) {
			log.Println("crawled ", crawlCount, "link(s), max pages reached.")
			return cw // done
		}

		cw.Links[urlStr] = true
		nextUrl, err := url.Parse(urlStr)

		if err != nil {
			log.Println("error while parsing url: " + err.Error())
			continue
		}
		if !IsValidScheme(nextUrl) {
			log.Println("scheme invalid, skipping url:" + nextUrl.String())
			continue
		}

		log.Println("parsing site: " + urlStr)

		ht, err := cw.GetPage(urlStr, "GET")

		cw.SavePage(ht)
		crawlCount += 1

		for _, newLink := range ht.RespInfo.Hrefs {
			val, hasLink := cw.Links[newLink]
			if hasLink && val == true {
				continue
			}
			newLinkUrl, err := url.Parse(newLink)
			if err != nil {
				continue
			}
			if newLinkUrl.Host == settings.Url.Host {
				cw.Links[newLink] = false
			}
		}

		time.Sleep(time.Duration(cw.WaitBetweenRequests) * time.Millisecond)
	}
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
