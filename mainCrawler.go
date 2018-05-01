package main

import (
	"errors"
	"flag"
	"log"
	"net/url"
	"os"

	"github.com/BlackEspresso/crawlbase"
	"github.com/fatih/color"
	"strings"
)

type crawlSettings struct {
	URL             *url.URL
	FileStoreURL    string
	WaitTime        int
	MaxPages        int
	StorageFolder   string
	URLRegEx        string
	followLinks     []string
	dontFollowLinks []string
	NoNewLinks      bool
}

/* usage examples:
ncrawler.exe -url http://www.google.com -storage ./storage
=> starts crawl from site http://www.google.com, only sites with same host (google.com)
saves files to ./storage

ncrawler.exe -report test.csv -storage ./storage
=> just generates reports from prev. crawls files stored in ./storage. All urls.

*/

var debugMode = false

func mainCrawler() {
	fs := flag.NewFlagSet("crawler", flag.ExitOnError)

	urlFlag := fs.String("url", "", "url, e.g. http://www.google.com")
	//urlRegEx := flag.String("regex", "", "only crawl links using this regex")
	waitFlag := fs.Int("wait", 500, "delay, in milliseconds")
	maxPagesFlag := fs.Int("max-pages", -1, "max pages to crawl, -1 for infinite")
	//fs.String("storageType", "file", "type of storage. (http,file,ftp)")
	storagePathFlag := fs.String("storage-path", "",
		"folder to store crawled files")
	debugFlag := fs.Bool("debug", false, "enable debugging")
	urlList := fs.String("url-list", "", "path to a list with urls")
	noNewLinks := fs.Bool("no-new-links", false,
		"dont crawl hrefs links. Use with url-list for example.")
	scopeToDomain := fs.Bool("scoped-to-domain", true, "scope the crawler to the domain")

	var followLinks, followLinksNot arrayFlags
	fs.Var(&followLinks, "links-follow", "some test flag")
	fs.Var(&followLinksNot, "links-not-follow", "some test flag")

	debugMode = *debugFlag

	fs.Parse(os.Args[2:])

	if *urlFlag == "" && *urlList == "" {
		color.Red("no url or url list provided.")
	}

	log.SetOutput(os.Stdout)

	settings := crawlSettings{}
	settings.WaitTime = *waitFlag
	settings.MaxPages = *maxPagesFlag
	settings.StorageFolder = *storagePathFlag
	settings.followLinks = followLinks
	settings.dontFollowLinks = followLinksNot
	settings.NoNewLinks = *noNewLinks

	cw := crawlbase.NewCrawler()
	cw.WaitBetweenRequests = settings.WaitTime
	cw.StorageFolder = settings.StorageFolder
	cw.ScopeToDomain = *scopeToDomain
	cw.BeforeCrawlFn = func(url string) (string, error) {
		return BeforeCrawlFn(&settings, cw, url)
	}
	cw.AfterCrawlFn = func(p *crawlbase.Page, err error) ([]string, error) {
		return AfterCrawlFn(&settings, p, err)
	}

	if doesExists, _ := exists(settings.StorageFolder); !doesExists && settings.StorageFolder != "" {
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

	if baseURL != nil {
		cw.FetchSites(baseURL)
	} else if *urlList != "" {
		cw.FetchSites(nil)
	}
}

func BeforeCrawlFn(settings *crawlSettings, cw *crawlbase.Crawler, url string) (string, error) {
	if settings.MaxPages >= 0 && cw.PageCount >= uint64(settings.MaxPages) {
		log.Println("crawled ", cw.PageCount, "link(s), max pages reached.")
		return "", errors.New("max pages reached")
	}
	return url, nil
}

func AfterCrawlFn(settings *crawlSettings, page *crawlbase.Page, err error) ([]string, error) {
	if page == nil {
		return nil, err
	}

	var crawlLinks []string

	isRedirect := page.Response.StatusCode >= 300 && page.Response.StatusCode < 308
	if settings.NoNewLinks {
		if isRedirect {
			val, ok := page.Response.Header["Location"]
			if ok && len(val) > 0 {
				crawlLinks = append(crawlLinks, val[0])
			}
		}
		return crawlLinks, nil
	}

	hasFollowFilter := len(settings.followLinks) > 0
	hasDontFollowFilter := len(settings.dontFollowLinks) > 0

	if !hasFollowFilter && !hasDontFollowFilter {
		return page.RespInfo.Hrefs, err
	}

	for _, link := range page.RespInfo.Hrefs {
		matchFollow := hasFollowFilter && containsAllText(settings.followLinks, link)
		matchDontFollow := hasDontFollowFilter && containsAnyText(settings.dontFollowLinks, link)

		if matchFollow && !matchDontFollow {
			crawlLinks = append(crawlLinks, link)
		}
	}

	return crawlLinks, err
}

func containsAnyText(testStrings []string, text string) bool {
	for _, tester := range testStrings {
		if strings.Contains(text, tester) {
			return true
		}
	}
	return false
}

func containsAllText(testStrings []string, text string) bool {
	for _, tester := range testStrings {
		if !strings.Contains(text, tester) {
			return false
		}
	}
	return true
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

type arrayFlags []string

func (i *arrayFlags) String() string {
	// change this, this is just can example to satisfy the interface
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
}
