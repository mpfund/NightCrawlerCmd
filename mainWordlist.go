package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func mainWordList() {
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

	keys := []string{}
	for k := range wordMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, v := range keys {
		file.Write([]byte(strings.ToLower(v)))
		file.Write([]byte("\n"))
	}
}

var regFindWordLower *regexp.Regexp = regexp.MustCompile("[a-zA-Z][a-z]{3,}")

func findAllWords(source string) map[string]bool {
	files, err := filepath.Glob(source)
	checkError(err)
	wordMap := map[string]bool{}

	for _, file := range files {
		fileContent, err := ioutil.ReadFile(file)
		if err != nil {
			log.Println(err)
		}

		allWords := -1
		wordsLower := regFindWordLower.FindAll(fileContent, allWords)

		addWords := func(words [][]byte) {
			for _, word := range words {
				wordStr := strings.ToLower(string(word))
				wordMap[wordStr] = true
			}
		}

		addWords(wordsLower)
	}
	return wordMap
}
