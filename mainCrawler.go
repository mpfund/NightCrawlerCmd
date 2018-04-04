package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/BlackEspresso/crawlbase"
	"github.com/fatih/color"
)

type crawlSettings struct {
	URL           *url.URL
	FileStoreURL  string
	WaitTime      int
	MaxPages      int
	StorageFolder string
	URLRegEx      string
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

var debugMode = false

func mainCrawler() {
	fs := flag.NewFlagSet("crawler", flag.ExitOnError)

	urlFlag := fs.String("url", "", "url, e.g. http://www.google.com")
	//urlRegEx := flag.String("regex", "", "only crawl links using this regex")
	waitFlag := fs.Int("wait", 1000, "delay, in milliseconds")
	maxPagesFlag := fs.Int("max-pages", -1, "max pages to crawl, -1 for infinite")
	//fs.String("storageType", "file", "type of storage. (http,file,ftp)")
	storagePathFlag := fs.String("storage-path", "./storage",
		"folder to store crawled files")
	debugFlag := fs.Bool("debug", false, "enable debugging")
	urlList := fs.String("url-list", "", "path to a list with urls")
	noNewLinks := fs.Bool("no-new-links", false,
		"dont crawl hrefs links. Use with url-list for example.")

	debugMode = *debugFlag

	fs.Parse(os.Args[2:])

	if *urlFlag == "" && *urlList == "" {
		color.Red("no url or url list provided.")
	}

	logf, err := os.OpenFile("crawler.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		color.Red("error opening file: %v", err)
	}

	log.SetOutput(io.MultiWriter(logf, os.Stdout))
	defer logf.Close()

	settings := crawlSettings{}
	settings.WaitTime = *waitFlag
	settings.MaxPages = *maxPagesFlag
	settings.StorageFolder = *storagePathFlag
	//settings.URLRegEx = *urlRegEx

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

	var baseURL *url.URL

	if *urlFlag != "" {
		// parse url & remove all out of scope urls
		baseURL, err = url.Parse(*urlFlag)
		checkError(err)
		cw.RemoveLinksNotSameHost(baseURL)
		settings.URL = baseURL
	}

	if *noNewLinks {
		// set all to crawled
		for k := range cw.Links {
			cw.Links[k] = true
		}
	}

	if *urlList != "" {
		lines, err := crawlbase.ReadWordlist(*urlList)
		checkError(err)
		var newURLs []string
		for _, l := range lines {
			if baseURL != nil {
				// use relative & absolute urls
				absURL := crawlbase.ToAbsUrl(baseURL, l)
				newURLs = append(newURLs, absURL)
			} else {
				// add only absolute ones
				newURL, err := url.Parse(l)
				checkError(err)
				if newURL.IsAbs() {
					newURLs = append(newURLs, l)
				}
			}
		}

		cw.AddAllLinks(newURLs)
		if baseURL != nil {
			cw.RemoveLinksNotSameHost(baseURL)
		}
	}

	cw.BeforeCrawlFn = func(url string) (string, error) {
		if settings.MaxPages >= 0 && cw.PageCount >= uint64(settings.MaxPages) {
			log.Println("crawled ", cw.PageCount, "link(s), max pages reached.")
			return "", errors.New("max pages reached")
		}
		return url, nil
	}

	if baseURL != nil {
		cw.FetchSites(baseURL)
	} else if *urlList != "" {
		cw.FetchSites(nil)
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
