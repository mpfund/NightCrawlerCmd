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
	"github.com/fatih/color"
)

type reportSettings struct {
	ReportFile    string
	StoragePath   string
	ProfileFolder string
	Profile       bool
	WordList      bool
	TagsFiles     string
}

type pageReport struct {
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

type wordInfo struct {
	Count int
	Page  string
}

func mainReport() {
	fs := flag.NewFlagSet("report", flag.ExitOnError)

	storagePathFlag := fs.String("storage-path", "./storage", "folder with crawled files from 'crawler'")
	reportFile := fs.String("reportsfolder", "./report", "folder for report files (*.csv)")
	profiling := fs.Bool("profiling", false, "enable profiling")
	wordlist := fs.Bool("wordlist", false, "generates a wordlist from crawled pages")
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
		color.Red("missing report file")
		return
	}

	generateReport(settings)
}

func filterInvalidHTMLByType(validations []*htmlcheck.ValidationError,
	reason htmlcheck.ErrorReason, max int) []*htmlcheck.ValidationError {

	var errors []*htmlcheck.ValidationError
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

func loadPage(file string, vdtr *htmlcheck.Validator, doWordlist bool) *pageReport {
	page, err := crawlbase.LoadPage(file, true)
	checkError(err)

	pr := &pageReport{}
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

	pURL, err := url.Parse(page.URL)
	if err != nil {
		color.Red("url invalid, skipping", page.URL)
		return nil
	}

	if page.Response != nil {
		isRedirect, location := crawlbase.LocationFromPage(page, pURL)
		if isRedirect {
			pr.Location = location
		}
	}

	pr.QueryKeys = map[string]bool{}
	for v := range pURL.Query() {
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

func genReportCrawledUrls(settings *reportSettings, pageReports map[string]*pageReport) {
	path := settings.ReportFile + "/crawledurls.csv"
	err := removeIfExists(path)
	checkError(err)
	file, err := os.OpenFile(path, os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)
	csv.Comma = ';'

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

func genReportAllUrls(settings *reportSettings, pageReports map[string]*pageReport) {
	path := settings.ReportFile + "/allUrls.csv"
	err := removeIfExists(path)
	checkError(err)
	file, err := os.OpenFile(path, os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)
	csv.Comma = ';'

	csv.Write([]string{"url"})

	urls := map[string]bool{}

	for _, info := range pageReports {
		for url := range info.Hrefs {
			if ok, _ := urls[url]; !ok {
				urls[url] = true
			}
		}
	}

	for url := range urls {
		csv.Write([]string{url})
	}

	csv.Flush()
	checkError(csv.Error())
}

func genReportQueryKeys(settings *reportSettings, usedURLQueryKeys map[string]string) {
	path := settings.ReportFile + "/querykeys.csv"
	err := removeIfExists(path)
	checkError(err)
	file, err := os.OpenFile(path, os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)
	csv.Comma = ';'

	for k, v := range usedURLQueryKeys {
		csv.Write([]string{k, v})
	}
	csv.Flush()
	checkError(csv.Error())
}

func genReportWordlist(settings *reportSettings, pageReports map[string]*pageReport) {
	path := settings.ReportFile + "/wordlist.csv"
	err := removeIfExists(path)
	checkError(err)

	words := map[string]*wordInfo{}

	for _, p := range pageReports {
		for _, u := range p.Words {
			if u == "" {
				continue
			}
			word := strings.ToLower(string(u))
			w, ok := words[word]
			if !ok {
				words[word] = &wordInfo{1, p.URL}
			} else {
				w.Count++
			}
		}
	}

	if len(words) == 0 {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)
	csv.Comma = ';'

	for u := range words {
		csv.Write([]string{u, strconv.Itoa(words[u].Count), words[u].Page})
	}

	csv.Flush()
	checkError(csv.Error())
}

func genReportInvalidTags(settings *reportSettings, pageReports map[string]*pageReport) {
	path := settings.ReportFile + "/invalidtags.csv"
	err := removeIfExists(path)
	checkError(err)
	file, err := os.OpenFile(path, os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)
	csv.Comma = ';'

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

func genReportFormsURL(settings *reportSettings, pageReports map[string]*pageReport) {
	path := settings.ReportFile + "/formtags.csv"
	err := removeIfExists(path)
	checkError(err)
	file, err := os.OpenFile(path, os.O_CREATE, 0655)
	checkError(err)
	defer file.Close()

	csv := csv.NewWriter(file)
	csv.Comma = ';'

	for pageURL, cPage := range pageReports {
		for _, form := range cPage.Forms {
			for _, input := range form.Inputs {
				csv.Write([]string{"", input.Name, input.Type, input.Value,
					pageURL, form.Url, form.Method})
			}
		}
	}
	csv.Flush()
	checkError(csv.Error())
}

func loadData(settings *reportSettings) (map[string]*pageReport, map[string]string) {
	pageReports := map[string]*pageReport{}
	usedURLQueryKeys := map[string]string{}

	vdtr := htmlcheck.Validator{}
	err := vdtr.LoadTagsFromFile(settings.TagsFiles)
	checkError(err)

	files, err := crawlbase.GetPageInfoFiles(settings.StoragePath)
	checkError(err)

	for _, file := range files {
		pr := loadPage(file, &vdtr, settings.WordList)
		pageReports[pr.URL] = pr
		for url := range pr.QueryKeys {
			usedURLQueryKeys[url] = pr.URL
		}
	}
	return pageReports, usedURLQueryKeys
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
		color.Yellow("loaded content in ", time.Now().Sub(startTime))
		writeHeap(settings.ProfileFolder, "0")
	}

	genReportCrawledUrls(settings, pages)
	genReportQueryKeys(settings, queryKeys)
	genReportInvalidTags(settings, pages)
	genReportWordlist(settings, pages)
	genReportFormsURL(settings, pages)
	genReportAllUrls(settings, pages)

	color.Green("report generated in %s", time.Now().Sub(startTime))
}

func removeIfExists(path string) error {
	ok, _ := exists(path)
	if ok {
		return os.Remove(path)
	}
	return nil
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
