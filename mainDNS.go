package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/BlackEspresso/crawlbase"
	"github.com/tealeg/xlsx"
)

type appSettings struct {
	SubdomainFile string
	Domain        string
	LogFile       string
	UseResume     bool
	History       map[string]bool
	DNSTypeNumber uint16
	ReportFile    string
}

func mainDNS() {
	fs := flag.NewFlagSet("dns", flag.ExitOnError)

	domain := fs.String("domain", "", "domain for dns scan")
	wordlist := fs.String("wordlist", "", "path to wordlist for subdomain scan")
	logFile := fs.String("log", "dnsscan.log", "")
	resume := fs.Bool("resume", false, "load log file and resume. skips already scanned urls")
	dnsType := fs.String("typeName", "", "request type by name (A,AAAA,MX,ANY)")
	dnsTypeNr := fs.Int("typeNumber", 1, "request type by number (1,28,15,255)")
	outputFile := fs.String("report", "", "output as excel file")

	fs.Parse(os.Args[2:])

	settings := appSettings{}
	settings.SubdomainFile = *wordlist
	settings.Domain = *domain
	settings.LogFile = *logFile
	settings.UseResume = *resume
	settings.History = map[string]bool{}
	settings.DNSTypeNumber = uint16(*dnsTypeNr)
	settings.ReportFile = *outputFile

	if *dnsType != "" {
		var ok bool
		settings.DNSTypeNumber, ok = crawlbase.DnsTypesByName[*dnsType]
		if !ok {
			log.Fatal("dnsType " + *dnsType + " not found")
			return
		}
	}

	if settings.UseResume {
		readReport(&settings)
	}

	if *domain == "" {
		log.Println("domain parameter missing")
		return
	}

	scanDNS(&settings)
}

func readReport(settings *appSettings) {
	_, err := os.Stat(settings.LogFile)
	if err != nil {
		return
	}
	data, err := ioutil.ReadFile(settings.LogFile)
	checkError(err)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		l := strings.Split(line, "\t")[0]
		l = strings.Split(l, " ")[0]
		l = strings.Trim(l, "\n\r")
		settings.History[l] = true
	}
}

func scanDNS(settings *appSettings) {
	ds := new(crawlbase.DNSScanner)
	ds.LoadConfigFromFile("./config/resolv.conf")
	dnsResp := map[string][]string{}
	if settings.SubdomainFile != "" {
		lines, err := crawlbase.ReadWordlist(settings.SubdomainFile)
		checkError(err)
		lines = filterLines(lines, settings)
		dnsResp = ds.ScanDNS(lines, settings.Domain, settings.DNSTypeNumber)
	} else {
		resp, _ := ds.ResolveDNS(settings.Domain, settings.DNSTypeNumber)
		dnsResp[settings.Domain] = resp
	}

	if settings.ReportFile == "" {
		dnsReport(dnsResp, settings)
	} else {
		dnsReportExcel(dnsResp, settings)
	}
}

func filterLines(lines []string, settings *appSettings) []string {
	var filteredLines []string
	for _, line := range lines {
		name := line + "." + settings.Domain + "."
		_, inHistory := settings.History[name]
		if !inHistory {
			filteredLines = append(filteredLines, line)
		}
	}
	return filteredLines
}

func dnsReportExcel(dnsResp map[string][]string, settings *appSettings) {
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("dns")
	checkError(err)

	for subDomain, entries := range dnsResp {
		row := sheet.AddRow()
		if len(entries) > 0 {
			for _, entry := range entries {
				row.WriteSlice(&[]string{"found", entry}, -1)
			}
		} else {
			row.WriteSlice(&[]string{"not found", subDomain + "." + settings.Domain + ".\n"}, -1)
		}
	}
	err = file.Save(settings.ReportFile)
	checkError(err)
}

func dnsReport(dnsResp map[string][]string, settings *appSettings) {
	buffer := bytes.Buffer{}
	for subDomain, entries := range dnsResp {
		if len(entries) > 0 {
			for _, entry := range entries {
				buffer.WriteString(entry + "\n")
				fmt.Println(entry)
			}
		} else {
			buffer.WriteString(subDomain + "." + settings.Domain + ".\n")
		}
	}

	logf, err := os.OpenFile(settings.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	logf.Write(buffer.Bytes())
}
