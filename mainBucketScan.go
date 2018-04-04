package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/BlackEspresso/crawlbase"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type BucketInfo struct {
	Name         string
	Urls         []string
	NoSuchBucket int
	AccessDenied int
}

var buckets = []*BucketInfo{{
	Name: "aws",
	Urls: []string{"s3.amazonaws.com",
		"s3.eu-west-1.amazonaws.com",
		"s3.us-east-2.amazonaws.com",
		"s3.us-west-2.amazonaws.com",
	},
	NoSuchBucket: 404,
	AccessDenied: 403,
}, {
	Name:         "azure",
	Urls:         []string{"blob.core.windows.net/?comp=list"},
	NoSuchBucket: 404,
	AccessDenied: 403,
}, {
	Name:         "dc",
	Urls:         []string{"nyc3.digitaloceanspaces.com"},
	NoSuchBucket: 404,
	AccessDenied: 403,
},
}

type settingsBucketScan struct {
	Suffix     string
	BucketType string
	WordList   string
	Splitter   string
	Url        string
	Bucket     *BucketInfo
	Verbose    int
	Delay      int
}

func mainBucketScan() {
	fs := flag.NewFlagSet("bucketscan", flag.ExitOnError)
	suffix := fs.String("suffix", "", "suffix or domain name like google.com")
	bucketTyp := fs.String("provider", "aws", "bucket type like s3 (digital ocean, google,..)")
	wordList := fs.String("wordlist", "", "path to word list for sub domain scan")
	splitter := fs.String("splitter", ".", "splitter character between suffix and domain")
	verbose := fs.Int("verbose", 0, "verbose level")
	url := fs.String("url", "", "overwrite url suffix of bucket type")
	delay := fs.Int("delay", 100, "delay in milliseconds")
	fs.Parse(os.Args[2:])

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	verboseLevel = *verbose

	settings := settingsBucketScan{
		Suffix:     *suffix,
		BucketType: *bucketTyp,
		WordList:   *wordList,
		Splitter:   *splitter,
		Url:        *url,
		Delay:      *delay,
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

	for _, urlSuffix := range settings.Bucket.Urls {
		for _, line := range lines {
			time.Sleep(time.Duration(settings.Delay) * time.Millisecond)
			url := "https://" + line + settings.Splitter + settings.Suffix + "." + urlSuffix
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
