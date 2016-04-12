package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BlackEspresso/crawlbase"
)

func mainPortScan() {
	fs := flag.NewFlagSet("portscan", flag.ExitOnError)

	target := fs.String("target", "", "hostname or ip for portscan")
	start := fs.Int("start", 79, "startport")
	end := fs.Int("end", 81, "endport")
	showClosed := fs.Bool("show-closed", false, "show closed ports")
	timeout := fs.Int("timeout", 20, "connection timeout")
	list := fs.String("portlist", "", "list of port names seperated by ,")

	fs.Parse(os.Args[2:])

	ps := crawlbase.NewPortScanner()
	ps.AfterScan = func(pi *crawlbase.PortInfo) {
		if pi.Open || *showClosed {
			log.Println(pi.Port, pi.Open, pi.Error)
			if pi.Size > 0 {
				log.Println(string(pi.Response))
			}
		}
	}
	ps.ConnectionTimeOut = time.Duration(*timeout) * time.Second

	if *list == "" {
		ps.ScanPortRange(*target, *start, *end)
	} else {
		ports := toPortList(*list)
		ps.ScanPortList(*target, ports)
	}

}

func toPortList(list string) []int {
	numbers := strings.Split(list, ",")
	pList := []int{}
	for _, n := range numbers {
		port, err := strconv.Atoi(n)
		if err == nil {
			pList = append(pList, port)
		}
	}
	return pList
}
