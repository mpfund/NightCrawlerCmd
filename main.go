package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
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

func main() {
	urlFlag := flag.String("url", "", "url, e.g. http://www.google.com")
	fileStorage := flag.String("filestore", "http://localhost:8079/file/7363a35f-f411-4751-96ec-2d19b5a22323", "url to filestore")
	delayFlag := flag.Int("delay", 1000, "delay, in milliseconds, default is 1000ms=1sec")
	maxPagesFlag := flag.Int("maxpages", -1, "max pages to crawl, -1 for infinite")
	inputFolderFlag := flag.String("inputfolder", "", "crawl from folder")
	flag.Parse()

	fileStorageUrl = *fileStorage

	if *urlFlag == "" {
		log.Fatal("no url provided.")
	}

	_, err := url.Parse(*urlFlag)
	checkerror(err)

	logf, err := os.OpenFile("nightcrawler.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logf.Close()

	log.SetOutput(io.MultiWriter(logf, os.Stdout))

	links := make(map[string]bool)
	links[*urlFlag] = false // startsite
	fetchSites(links, *delayFlag, *maxPagesFlag, *inputFolderFlag)
}

func IsValidScheme(url *url.URL) bool {
	scheme := url.Scheme
	if scheme == "http" || scheme == "https" {
		return true
	} else {
		return false
	}
}

func fetchSites(links map[string]bool, delayMs int, maxPages int, folder string) {
	cw := crawlbase.Crawler{}
	tags, err := crawlbase.LoadTagsFromFile("tags.json")
	if err != nil {
		log.Fatal(err)
	}
	cw.Validator.AddValidTags(tags)
	cw.IncludeHiddenLinks = false
	crawlCount := uint64(0)

	for {
		urlStr, found := getNextSite(links)
		if !found {
			return // done
		}
		if maxPages >= 0 && crawlCount >= uint64(maxPages) {
			return // done
		}

		links[urlStr] = true
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
		content, err := json.Marshal(ht)
		checkerror(err)

		fileName := strconv.FormatInt(int64(ht.CrawlTime), 10) + ".httpt"
		savePagePerFile(urlStr, fileName, content)
		crawlCount += 1

		for _, newLink := range ht.RespInfo.Hrefs {
			val, hasLink := links[newLink]
			if hasLink && val == true {
				continue
			}
			links[newLink] = false
		}

		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
}

func getNextSite(links map[string]bool) (string, bool) {
	for i, l := range links {
		if l == false {
			return i, true
		}
	}
	return "", false
}

func checkerror(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func savePagePerFile(crawledUri string, fileName string, content []byte) {
	_, err := os.Stat("./storage")
	if err != nil && os.IsNotExist(err) {
		err := os.Mkdir("storage", 0777)
		checkerror(err)
	}

	err = ioutil.WriteFile("./storage/"+fileName, content, 0666)
	checkerror(err)
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
