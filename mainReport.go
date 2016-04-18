package main

import (
	"flag"
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
}

type PageReport struct {
	URL               string
	FileName          string
	RespDuration      int
	StatusCode        int
	Location          string
	Words             []string
	TextUrls          []string
	Error             string
	InvalidTags       []string
	InvalidAttributes []string
	QueryKeys         map[string]string
	Hrefs             map[string]bool
}

func mainReport() {
	fs := flag.NewFlagSet("report", flag.ExitOnError)

	storagePathFlag := fs.String("storage-path", "./storage", "folder to store crawled files")
	reportFile := fs.String("report", "report.xlsx", "generates report (xlsx-File)")
	profile := fs.Bool("profile", false, "enable profiling")

	fs.Parse(os.Args[2:])

	settings := &reportSettings{}
	settings.ProfileFolder = "./profiling/"
	settings.ReportFile = *reportFile
	settings.StoragePath = *storagePathFlag
	settings.Profile = *profile

	if *reportFile == "" {
		log.Println("missing report file")
		return
	}

	generateReport(settings)
}

func validationErrorToText(validations []*htmlcheck.ValidationError) []string {
	list := []string{}
	for _, k := range validations {
		col := strconv.Itoa(k.TextPos.Column)
		line := strconv.Itoa(k.TextPos.Line)
		attr := k.AttributeName
		list = append(list, "<"+k.TagName+"> "+attr+" ("+line+", "+col+")")
	}
	return list
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

func loadPage(file string, vdtr *htmlcheck.Validator, h2tSettings *html2text.TexterSettings) *PageReport {
	page, err := crawlbase.LoadPage(file, true)
	checkError(err)

	pr := &PageReport{}
	pr.RespDuration = page.RespDuration
	pr.FileName = strconv.Itoa(page.CrawlTime)
	pr.URL = page.URL
	pr.Location = ""
	pr.InvalidTags = []string{}
	pr.InvalidAttributes = []string{}
	rawUrls := crawlbase.GetUrlsFromText(page.ResponseBody, 100)
	pr.TextUrls = bytesToStrings(rawUrls)
	pr.Error = page.Error

	if page.Response != nil {
		pr.StatusCode = page.Response.StatusCode

		mime := crawlbase.GetContentMime(page.Response.Header)

		if mime == "text/html" {
			body := string(page.ResponseBody)
			vErros := vdtr.ValidateHtmlString(body)

			invs := filterInvalidHtmlByType(vErros, htmlcheck.InvTag, 10)
			htmlcheck.GetErrorLines(body, invs)
			pr.InvalidTags = validationErrorToText(invs)

			invs = filterInvalidHtmlByType(vErros, htmlcheck.InvAttribute, 10)
			htmlcheck.GetErrorLines(body, invs)
			pr.InvalidAttributes = validationErrorToText(invs)

			plainText, err := html2text.Html2Text(string(page.RequestBody),
				*h2tSettings)
			if err != nil {
				log.Println(err)
			}
			rawWords := crawlbase.GetWordListFromText([]byte(plainText), 500)
			pr.Words = bytesToStrings(rawWords)
		} else {
			rawWords := crawlbase.GetWordListFromText(page.ResponseBody, 500)
			pr.Words = bytesToStrings(rawWords)
		}
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

	pr.QueryKeys = map[string]string{}
	for v, _ := range pUrl.Query() {
		pr.QueryKeys[v] = page.URL
	}

	pr.Hrefs = map[string]bool{}
	for _, href := range page.RespInfo.Hrefs {
		pr.Hrefs[href] = true
	}

	return pr
}

func bytesToStrings(arr [][]byte) []string {
	ret := make([]string, len(arr))
	for _, val := range arr {
		ret = append(ret, string(val))
	}
	return ret
}

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
	tags, err := crawlbase.LoadTagsFromFile("tags.json")
	checkError(err)

	vdtr.AddValidTags(tags)

	if settings.Profile {
		f, err := os.Create(settings.ProfileFolder + "cpuprofile.pprof")
		checkError(err)
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	conf := html2text.NewSettings()

	for _, file := range files {
		pr := loadPage(file, &vdtr, &conf)
		pageReports[pr.URL] = pr
		for _, querykey := range pr.QueryKeys {
			usedUrlQueryKeys[querykey] = pr.URL
		}
	}

	if settings.Profile {
		log.Println("loaded content in ", time.Now().Sub(startTime))
		writeHeap(settings.ProfileFolder, "0")
	}

	row := sheetUrls.AddRow()
	row.WriteSlice(&[]string{"timestamp", "url", "Http code", "duration (ms)", "redirect url", "error"}, -1)

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
		if len(info.InvalidTags) > 0 || len(info.InvalidAttributes) > 0 {
			row = sInvTags.AddRow()
			row.WriteSlice(&[]string{
				info.FileName,
				info.URL,
			}, -1)

			for _, inv := range info.InvalidTags {
				row = sInvTags.AddRow()
				row.WriteSlice(&[]string{"tag", inv}, -1)
			}
			for _, inv := range info.InvalidAttributes {
				row = sInvTags.AddRow()
				row.WriteSlice(&[]string{"attr", inv}, -1)
			}
		}
	}

	// text urls
	textUrls := map[string]string{}

	for _, p := range pageReports {
		for _, u := range p.TextUrls {
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
	words := map[string]int{}

	for _, p := range pageReports {
		for _, u := range p.Words {
			word := strings.ToLower(string(u))
			i := words[word]
			words[word] = i + 1
		}
	}
	sheetWordList, _ := file.AddSheet("wordlist")

	for u, _ := range words {
		row = sheetWordList.AddRow()
		row.WriteSlice(&[]interface{}{u, words[u]}, -1)
	}

	err = file.Save(settings.ReportFile)
	checkError(err)

	log.Println("report generated in", time.Now().Sub(startTime))
}
