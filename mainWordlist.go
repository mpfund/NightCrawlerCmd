package main

import (
	"flag"
	"github.com/BlackEspresso/crawlbase"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func mainWordlist() {
	fs := flag.NewFlagSet("wordlist", flag.ExitOnError)

	source := fs.String("source", "", "files to read e.g. folder/*.txt")
	output := fs.String("output", "wordlist.txt", "wordlist output")

	fs.Parse(os.Args[2:])

	createWordList(*source, *output)
}

func createWordList(source, target string) {
	wordMap := findAllWords(source)
	os.Remove(target) // skip error
	file, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE, 0660)
	checkError(err)
	defer file.Close()

	for k := range wordMap {
		file.Write([]byte(strings.ToLower(k)))
		file.Write([]byte("\n"))
	}
}

func findAllWords(source string) map[string]bool {
	files, err := filepath.Glob(source)
	checkError(err)
	wordMap := map[string]bool{}

	for _, file := range files {
		fileContent, err := ioutil.ReadFile(file)
		if err != nil {
			log.Println(err)
		}
		words := crawlbase.GetWordListFromText(fileContent, -1)
		for _, word := range words {
			wordMap[string(word)] = true
		}
	}
	return wordMap
}
