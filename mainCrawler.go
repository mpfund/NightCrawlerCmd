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
}

/* usage examples:
ncrawler.exe -url http://www.google.com
=> starts crawl from site http://www.google.com, only sites with same host (google.com)
saves files to ./storage

ncrawler.exe -report test.csv
=> just generates reports from prev. crawls files stored in ./storage. All urls.

ncrawler.exe -url http://www.google.com -report test.csv
=> starts crawl http://www.google.com and generate report for url in the end

ncrawler.exe -url http://www.google.com -report test.csv -nocrawl
=> just generate report for url

*/

var DebugMode bool = false

func mainCrawler() {
	fs := flag.NewFlagSet("crawler", flag.ExitOnError)

	urlFlag := fs.String("url", "", "url, e.g. http://www.google.com")
	//urlRegEx := flag.String("regex", "", "only crawl links using this regex")
	waitFlag := fs.Int("wait", 1000, "delay, in milliseconds")
	maxPagesFlag := fs.Int("max-pages", -1, "max pages to crawl, -1 for infinite")
	//fs.String("storageType", "file", "type of storage. (http,file,ftp)")
	storagePathFlag := fs.String("storage-path", "./storage", "folder to store crawled files")
	clearStorageFlag := fs.Bool("clear-storage", false, "delete all storage files")
	debugFlag := fs.Bool("debug", false, "enable debugging")
	urlList := fs.String("urllist", "", "path to list with urls")
	noNewLinks := fs.Bool("no-new-links", false, "dont crawl hrefs links.")

	DebugMode = *debugFlag

	fs.Parse(os.Args[2:])

	if *urlFlag == "" && *urlList == "" {
		log.Fatal("no url or url list provided.")
	}

	logf, err := os.OpenFile("websec.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	log.SetOutput(io.MultiWriter(logf, os.Stdout))
	defer logf.Close()

	settings := crawlSettings{}
	settings.WaitTime = *waitFlag
	settings.MaxPages = *maxPagesFlag
	settings.StorageFolder = *storagePathFlag

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

	if baseUrl != nil {
		cw.FetchSites(baseUrl)
	} else if *urlList != "" {
		cw.FetchSites(nil)
	}

	if *clearStorageFlag {
		log.Println("delete storage files")
		clearStorage(&settings)
	}
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
