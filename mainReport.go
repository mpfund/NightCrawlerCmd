package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BlackEspresso/crawlbase"
	"github.com/BlackEspresso/html2text"
	"github.com/BlackEspresso/htmlcheck"
	"github.com/tealeg/xlsx"
)

type reportSettings struct {
	ReportFile    string
	StoragePath   string
	ProfileFolder string
	Profile       bool
	WordList      bool
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
	reportFile := fs.String("report", "report.xlsx", "generates report (xlsx-File)")
	profiling := fs.Bool("profiling", false, "enable profiling")
	wordlist := fs.Bool("wordlist", false, "enable wordlist")

	fs.Parse(os.Args[2:])

	settings := &reportSettings{}
	settings.ProfileFolder = "./profiling/"
	settings.ReportFile = *reportFile
	settings.StoragePath = *storagePathFlag
	settings.Profile = *profiling
	settings.WordList = *wordlist

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

func loadPage(file string, vdtr *htmlcheck.Validator, h2tSettings *html2text.TexterSettings, doWordlist bool) *PageReport {
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
				plainText, err := html2text.Html2Text(body, *h2tSettings)
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

/*
type Cache struct {
	dir map[string]interface{}
}

func (c *Cache) Get(name string) (interface{}, err) {
	return dir[name]
}

func (c *Cache) StartLoadingCache(f func(string) interface{}) {

}
*/
func generateReport(settings *reportSettings) {
	startTime := time.Now()

	file := xlsx.NewFile()
	sheetUrls, err := file.AddSheet("Crawled Urls")
	checkError(err)

	files, err := crawlbase.GetPageInfoFiles(settings.StoragePath)
	checkError(err)

	pageReports := map[string]*PageReport{}
	usedUrlQueryKeys := map[string]string{}

	vdtr := htmlcheck.Validator{}
	err = vdtr.LoadTagsFromFile("tags.json")
	checkError(err)

	if settings.Profile {
		f, err := os.Create(settings.ProfileFolder + "cpuprofile.pprof")
		checkError(err)
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	conf := html2text.NewSettings()

	for _, file := range files {
		pr := loadPage(file, &vdtr, &conf, settings.WordList)
		pageReports[pr.URL] = pr
		for url, _ := range pr.QueryKeys {
			usedUrlQueryKeys[url] = pr.URL
		}
	}

	if settings.Profile {
		log.Println("loaded content in ", time.Now().Sub(startTime))
		writeHeap(settings.ProfileFolder, "0")
	}

	row := sheetUrls.AddRow()
	row.WriteSlice(&[]string{"timestamp", "url", "Http code", "duration (ms)",
		"redirect url", "error"}, -1)

	for _, info := range pageReports {
		dur := info.RespDuration
		row = sheetUrls.AddRow()
		row.WriteSlice(&[]interface{}{
			info.FileName,
			info.URL,
			info.StatusCode,
			dur,
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
		if len(info.InvalidTags) > 0 {
			for _, inv := range info.InvalidTags {
				row = sInvTags.AddRow()
				reason := fmt.Sprint(inv.Reason)
				col := fmt.Sprint(inv.TextPos.Column)
				row.WriteSlice(&[]string{reason, inv.TagName, inv.AttributeName,
					col, info.FileName, info.URL}, -1)
			}
		}
	}

	// text urls
	textUrls := map[string]string{}

	for _, p := range pageReports {
		for _, u := range p.TextUrls {
			if u == "" {
				continue
			}
			textUrls[u] = p.URL
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

	// wordlist
	// text urls
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
	sheetWordList, _ := file.AddSheet("wordlist")

	for u, _ := range words {
		row = sheetWordList.AddRow()
		row.WriteSlice(&[]interface{}{u, words[u].Count, words[u].Page}, -1)
	}

	// form urls
	sheetFormUrls, _ := file.AddSheet("form urls")

	for pageUrl, cPage := range pageReports {
		for _, form := range cPage.Forms {
			for _, input := range form.Inputs {
				row = sheetFormUrls.AddRow()
				row.WriteSlice(&[]interface{}{"", input.Name, input.Type, input.Value, pageUrl, form.Url, form.Method}, -1)
			}
		}
	}

	sheetIPs, _ := file.AddSheet("ips")
	textIPs := map[string]string{}

	for _, p := range pageReports {
		for _, u := range p.TextIPs {
			if u == "" {
				continue
			}
			textIPs[u] = p.URL
		}
	}

	for ip, url := range textIPs {
		row = sheetIPs.AddRow()
		row.WriteSlice(&[]interface{}{ip, url}, -1)
	}

	err = file.Save(settings.ReportFile)
	checkError(err)

	log.Println("report generated in", time.Now().Sub(startTime))
}
