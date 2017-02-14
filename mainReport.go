package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/BlackEspresso/crawlbase"
	"github.com/BlackEspresso/html2text"
	"github.com/BlackEspresso/htmlcheck"
)

type reportSettings struct {
	ReportFile    string
	StoragePath   string
	ProfileFolder string
	Profile       bool
	WordList      bool
	TagsFiles     string
}

type PageReport struct {
	URL               string
	FileName          string
	RespDuration      int
	StatusCode        int
	Location          string
	Words             []string
	TextUrls          []string
	TextIPs           []string
	Error             string
	InvalidTags       []*htmlcheck.ValidationError
	InvalidAttributes []string
	QueryKeys         map[string]bool
	Hrefs             map[string]bool
	Forms             []crawlbase.Form
}

type WordInfo struct {
	Count int
	Page  string
}

func mainReport() {
	fs := flag.NewFlagSet("report", flag.ExitOnError)

	storagePathFlag := fs.String("storage-path", "./storage", "folder to store crawled files")
	reportFile := fs.String("reportsfolder", "./report", "folder for report files (*.csv)")
	profiling := fs.Bool("profiling", false, "enable profiling")
	wordlist := fs.Bool("wordlist", false, "enable wordlist")
	tagsFile := fs.String("tagsfile", "./config/tags.json", "path to tags file")

	fs.Parse(os.Args[2:])

	settings := &reportSettings{}
	settings.ProfileFolder = "./profiling/"
	settings.ReportFile = *reportFile
	settings.StoragePath = *storagePathFlag
	settings.Profile = *profiling
	settings.WordList = *wordlist
	settings.TagsFiles = *tagsFile

	if *reportFile == "" {
		log.Println("missing report file")
		return
	}

	generateReport(settings)
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

func loadPage(file string, vdtr *htmlcheck.Validator, doWordlist bool) *PageReport {
	page, err := crawlbase.LoadPage(file, true)
	checkError(err)

	pr := &PageReport{}
	pr.RespDuration = page.RespDuration
	pr.FileName = strconv.Itoa(page.CrawlTime)
	pr.URL = page.URL
	pr.Location = ""
	pr.InvalidTags = []*htmlcheck.ValidationError{}
	pr.InvalidAttributes = []string{}
	pr.Error = page.Error

	h2tSettings := html2text.NewSettings()
	h2tSettings.IncludeLinkUrls = false

	if doWordlist {
		rawText := crawlbase.GetUrlsFromText(page.ResponseBody, 100)
		pr.TextUrls = bytesToStrings(rawText)
		rawText = crawlbase.GetIPsFromText(page.ResponseBody, 100)
		pr.TextIPs = bytesToStrings(rawText)
	}

	if page.Response != nil {
		pr.StatusCode = page.Response.StatusCode

		mime := crawlbase.GetContentMime(page.Response.Header)
		if mime == "text/html" {
			body := string(page.ResponseBody)
			vErros := vdtr.ValidateHtmlString(body)
			htmlcheck.UpdateErrorLines(body, vErros)
			pr.InvalidTags = vErros

			if doWordlist {
				plainText, err := html2text.Html2Text(body, h2tSettings)
				if err != nil {
					log.Println(err)
				}
				rawWords := crawlbase.GetWordListFromText([]byte(plainText), 2000)
				pr.Words = bytesToStrings(rawWords)
			}
		}
		/* else {
			rawWords := crawlbase.GetWordListFromText(page.ResponseBody, 2000)
			pr.Words = bytesToStrings(rawWords)
		}*/
	}

	pUrl, err := url.Parse(page.URL)
	if err != nil {
		log.Println("url invalid, skipping", page.URL)
		return nil
	}

	if page.Response != nil {
		isRedirect, location := crawlbase.LocationFromPage(page, pUrl)
		if isRedirect {
			pr.Location = location
		}
	}

	pr.QueryKeys = map[string]bool{}
	for v, _ := range pUrl.Query() {
		pr.QueryKeys[v] = true
	}

	pr.Hrefs = map[string]bool{}
	for _, href := range page.RespInfo.Hrefs {
		if href == "" {
			continue
		}
		pr.Hrefs[href] = true
	}
	pr.Forms = page.RespInfo.Forms

	return pr
}

func bytesToStrings(arr [][]byte) []string {
	ret := make([]string, len(arr))
	for _, val := range arr {
		ret = append(ret, string(val))
	}
	return ret
}

func genReportCrawledUrls(settings *reportSettings, pageReports map[string]*PageReport) {
	file, err := os.OpenFile(settings.ReportFile+"/crawledurls.csv", os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)

	csv.Write([]string{"timestamp", "url", "Http code", "duration (ms)",
		"redirect url", "error"})

	for _, info := range pageReports {
		dur := info.RespDuration
		csv.Write([]string{
			info.FileName,
			info.URL,
			strconv.Itoa(info.StatusCode),
			strconv.Itoa(dur),
			info.Location,
			info.Error,
		})
	}

	csv.Flush()
	checkError(csv.Error())
}

func genReportQueryKeys(settings *reportSettings, usedUrlQueryKeys map[string]string) {
	file, err := os.OpenFile(settings.ReportFile+"/querykeys.csv", os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)

	for k, v := range usedUrlQueryKeys {
		csv.Write([]string{k, v})
	}
	csv.Flush()
	checkError(csv.Error())
}

func genReportWordlist(settings *reportSettings, pageReports map[string]*PageReport) {
	file, err := os.OpenFile(settings.ReportFile+"/wordlist.csv", os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)

	words := map[string]*WordInfo{}

	for _, p := range pageReports {
		for _, u := range p.Words {
			if u == "" {
				continue
			}
			word := strings.ToLower(string(u))
			w, ok := words[word]
			if !ok {
				words[word] = &WordInfo{1, p.URL}
			} else {
				w.Count += 1
			}
		}
	}

	for u, _ := range words {
		csv.Write([]string{u, strconv.Itoa(words[u].Count), words[u].Page})
	}

	csv.Flush()
	checkError(csv.Error())
}

func genReportInvalidTags(settings *reportSettings, pageReports map[string]*PageReport) {
	file, err := os.OpenFile(settings.ReportFile+"/invalidtags.csv", os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)

	csv.Write([]string{"reason", "tag", "attribute", "line",
		"file name", "url"})

	for _, info := range pageReports {
		if len(info.InvalidTags) > 0 {
			for _, inv := range info.InvalidTags {
				reason := fmt.Sprint(inv.Reason)
				line := fmt.Sprint(inv.TextPos.Line)
				csv.Write([]string{reason, inv.TagName, inv.AttributeName,
					line, info.FileName, info.URL})
			}
		}
	}

	csv.Flush()
	checkError(csv.Error())
}

func genReportFormsUrl(settings *reportSettings, pageReports map[string]*PageReport) {
	file, err := os.OpenFile(settings.ReportFile+"/formtags.csv", os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)

	for pageUrl, cPage := range pageReports {
		for _, form := range cPage.Forms {
			for _, input := range form.Inputs {
				csv.Write([]string{"", input.Name, input.Type, input.Value,
					pageUrl, form.Url, form.Method})
			}
		}
	}
	csv.Flush()
	checkError(csv.Error())
}

func loadData(settings *reportSettings) (map[string]*PageReport, map[string]string) {
	pageReports := map[string]*PageReport{}
	usedUrlQueryKeys := map[string]string{}

	vdtr := htmlcheck.Validator{}
	err := vdtr.LoadTagsFromFile(settings.TagsFiles)
	checkError(err)

	files, err := crawlbase.GetPageInfoFiles(settings.StoragePath)
	checkError(err)

	for _, file := range files {
		pr := loadPage(file, &vdtr, settings.WordList)
		pageReports[pr.URL] = pr
		for url, _ := range pr.QueryKeys {
			usedUrlQueryKeys[url] = pr.URL
		}
	}
	return pageReports, usedUrlQueryKeys
}

func generateReport(settings *reportSettings) {
	startTime := time.Now()

	if settings.Profile {
		f, err := os.Create(settings.ProfileFolder + "cpuprofile.pprof")
		checkError(err)
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	pages, queryKeys := loadData(settings)

	if settings.Profile {
		log.Println("loaded content in ", time.Now().Sub(startTime))
		writeHeap(settings.ProfileFolder, "0")
	}

	genReportCrawledUrls(settings, pages)
	genReportQueryKeys(settings, queryKeys)
	genReportInvalidTags(settings, pages)
	genReportWordlist(settings, pages)

	log.Println("report generated in", time.Now().Sub(startTime))
}
