package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/BlackEspresso/crawlbase"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type BucketInfo struct {
	Name         string
	Urls         []string
	NoSuchBucket int
	AccessDenied int
}

var buckets []*BucketInfo

type settingsBucketScan struct {
	Prefix     string
	BucketType string
	WordList   string
	Splitter   string
	Url        string
	Filter     string
	Bucket     *BucketInfo
	Verbose    int
	Delay      int
}

func mainBucketScan() {
	fs := flag.NewFlagSet("bucketscan", flag.ExitOnError)
	prefix := fs.String("prefix", "{w}", "prefix or domain name like {w}.google-com")
	bucketTyp := fs.String("provider", "aws", "bucket type like s3 (digital ocean, google,..)")
	wordList := fs.String("wordlist", "", "path to word list for sub domain scan")
	splitter := fs.String("splitter", ".", "splitter character between suffix and domain")
	verbose := fs.Int("verbose", 0, "verbose level")
	url := fs.String("url", "", "overwrite url suffix of bucket type")
	delay := fs.Int("delay", 100, "delay in milliseconds")
	filter := fs.String("filter", "", "filter bucket urls (not wordlist items)")
	configFile := fs.String("config", "./config/bucketscan.json", "path to config file")
	fs.Parse(os.Args[2:])

	if *configFile != "" {
		configData, err := ioutil.ReadFile(*configFile)
		checkError(err)
		err = json.Unmarshal(configData, &buckets)
		checkError(err)
	} else {
		buckets = []*BucketInfo{
			{Name: "aws",
				Urls:         []string{"s3.amazonaws.com"},
				NoSuchBucket: 404,
				AccessDenied: 403,
			}}
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	verboseLevel = *verbose

	settings := settingsBucketScan{
		Prefix:     *prefix,
		BucketType: *bucketTyp,
		WordList:   *wordList,
		Splitter:   *splitter,
		Url:        *url,
		Delay:      *delay,
		Filter:     *filter,
	}

	for _, b := range buckets {
		if b.Name == *bucketTyp {
			settings.Bucket = b
			break
		}
	}
	if settings.Bucket == nil {
		fmt.Println("provider " + *bucketTyp + " not found")
		return
	}

	if settings.Url != "" {
		settings.Bucket.Urls = []string{settings.Url}
	}

	scanBucket(&settings)
}

func scanBucket(settings *settingsBucketScan) {
	lines, err := crawlbase.ReadWordlist(settings.WordList)
	checkError(err)
	useFilter := settings.Filter != ""

	for _, urlSuffix := range settings.Bucket.Urls {
		if useFilter && !strings.Contains(urlSuffix, settings.Filter) {
			logVerbose(1, "skipping", urlSuffix)
			continue
		}

		for _, line := range lines {
			time.Sleep(time.Duration(settings.Delay) * time.Millisecond)
			prefix := strings.Replace(settings.Prefix,"{w}",line,1)

			url := "https://" + prefix + "." + urlSuffix
			resp, err := http.Get(url)
			if err != nil {
				logVerbose(1, url, err)
				continue
			}

			if resp.StatusCode == settings.Bucket.NoSuchBucket {
				logVerbose(1, "error: ", url, "bucket not found")
				continue
			}

			if resp.StatusCode == settings.Bucket.AccessDenied {
				fmt.Println(url, "access denied")
				continue
			}

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logVerbose(1, url, err)
				continue
			}

			strData := string(data)
			fmt.Println(url, strData)
		}
	}
}
