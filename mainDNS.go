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
)

type appSettings struct {
	SubdomainFile string
	Domain        string
	LogFile       string
	UseResume     bool
	History       map[string]bool
	DnsTypeNumber uint16
}

func mainDNS() {
	fs := flag.NewFlagSet("dns", flag.ExitOnError)

	domain := fs.String("domain", "", "domain for dns scan")
	subdomains := fs.String("subdomains", "", "subdomain list for bf scan")
	logFile := fs.String("output", "dnsscan.log", "")
	resume := fs.Bool("resume", false, "resume from old scan")
	dnsType := fs.String("typeName", "any", "request type by name")
	dnsTypeNr := fs.Int("typeNumber", 1, "request type by number")

	fs.Parse(os.Args[2:])

	settings := appSettings{}
	settings.SubdomainFile = *subdomains
	settings.Domain = *domain
	settings.LogFile = *logFile
	settings.UseResume = *resume
	settings.History = map[string]bool{}
	settings.DnsTypeNumber = uint16(*dnsTypeNr)

	if *dnsType != "" {
		var ok bool
		settings.DnsTypeNumber, ok = crawlbase.DnsTypesByName[*dnsType]
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

func checkError(e error) {
	if e != nil {
		log.Fatal(e)
	}
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
	ds.LoadConfigFromFile("./resolv.conf")
	dnsResp := map[string][]string{}
	if settings.SubdomainFile != "" {
		data, err := ioutil.ReadFile(settings.SubdomainFile)
		checkError(err)
		lines := SplitByLines(string(data))
		lines = filterLines(lines, settings)
		dnsResp = ds.ScanDNS(lines, settings.Domain, settings.DnsTypeNumber)
	} else {
		resp, _ := ds.ResolveDNS(settings.Domain, settings.DnsTypeNumber)
		dnsResp[settings.Domain] = resp
	}

	dnsReport(dnsResp, settings)
}

func filterLines(lines []string, settings *appSettings) []string {
	filteredLines := []string{}
	for _, line := range lines {
		name := line + "." + settings.Domain + "."
		_, inHistory := settings.History[name]
		if !inHistory {
			filteredLines = append(filteredLines, line)
		}
	}
	return filteredLines
}

func dnsReport(dnsResp map[string][]string, settings *appSettings) {
	buffer := bytes.Buffer{}
	for subdomain, _ := range dnsResp {
		entries := dnsResp[subdomain]
		if len(entries) > 0 {
			for _, entry := range entries {
				buffer.WriteString(entry + "\n")
				fmt.Println(entry)
			}
		} else {
			buffer.WriteString(subdomain + "." + settings.Domain + ".\n")
		}
	}

	logf, err := os.OpenFile(settings.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	logf.Write(buffer.Bytes())
}

func SplitByLines(text string) []string {
	lines := strings.Split(text, "\n")
	cleanLines := make([]string, len(lines))
	for _, k := range lines {
		line := strings.Trim(k, "\n\r")
		cleanLines = append(cleanLines, line)
	}
	return cleanLines
}
