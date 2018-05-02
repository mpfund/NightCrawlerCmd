package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type MutatorFunc func(string) []string

var regFindWordLower = regexp.MustCompile(`[a-zA-Z][a-z]{3,}`)
var regFindUrlsRel = regexp.MustCompile(`[a-zA-Z0-9]*[\/\\][a-zA-Z0-9-\._\\]{4,}`)
var regFindUrlsAbs = regexp.MustCompile(`[a-zA-Z]{2,}://[\w:-\\-\.\/]+`)
var regFindStringsOnly = regexp.MustCompile(`\"([[:print:]]*?)\"`)
var regFindStringsOnly2 = regexp.MustCompile(`\'([[:print:]]*?)\'`)
var regFindEmail = regexp.MustCompile(`[a-zA-Z0-9_öäüÄÖÜß\-\.]{3,}@[a-zA-Z0-9_öäüÄÖÜß\.\-]{2,}\.[a-zA-Z0-9_öäüÄÖÜß\.\-]{2,}`)

var mutators = map[string]MutatorFunc{}

type settingsWordlist struct {
	Template     string
	Input        string
	Output       string
	Extractor    string
	MutatorName  string
	ShowFileName bool
}

func mainWordList() {
	fs := flag.NewFlagSet("wordlist", flag.ExitOnError)

	mutatorName := fs.String("mutator", "", "mutator")
	template := fs.String("template", "", "template file")
	source := fs.String("input", "", "files to read e.g. folder/*.txt")
	output := fs.String("output", "wordlist.txt", "wordlist output")
	extractor := fs.String("extractor", "word",
		"how to extract words from text: none, word, url,url_abs, url_rel, string, email")
	showFileName := fs.Bool("show-file-name", false, "")

	fs.Parse(os.Args[2:])

	settings := settingsWordlist{
		Template:     *template,
		MutatorName:  *mutatorName,
		Extractor:    *extractor,
		Input:        *source,
		Output:       *output,
		ShowFileName: *showFileName,
	}

	mutators["username"] = usernameMutator

	createWordList(&settings)
}

func createWordList(settings *settingsWordlist) {
	wordMap := findAllWords(settings)
	os.Remove(settings.Output)
	file, err := os.OpenFile(settings.Output, os.O_RDWR|os.O_CREATE, 0660)
	checkError(err)
	defer file.Close()

	var templates = []string{"<word>"}
	if settings.Template != "" {
		templateContent, err := ioutil.ReadFile(settings.Template)
		templates = strings.Split(string(templateContent), "\n")
		checkError(err)
	}

	newWordMap := permute(wordMap, settings.MutatorName)

	finalWords := map[string]bool{}

	for _, template := range templates {
		for v := range newWordMap {
			templated := strings.Replace(template, "<word>", v, 1)
			k := strings.ToLower(strings.TrimSpace(templated))
			finalWords[k] = true
		}
	}

	writeToFile(finalWords, file)
}

func writeToFile(words map[string]bool, file *os.File) {
	var wordList []string
	for k := range words {
		wordList = append(wordList, k)
	}
	sort.Strings(wordList)

	for _, word := range wordList {
		if strings.TrimSpace(word) == "" {
			continue
		}
		file.Write([]byte(word + "\n"))
	}
}

func permute(wordMap map[string]bool, permuter string) map[string]bool {
	if permuter == "" {
		return wordMap
	}
	permuteFunc := mutators[permuter]

	newWords := map[string]bool{}

	for k := range wordMap {
		words := permuteFunc(k)
		for _, word := range words {
			newWords[word] = true
		}
	}

	return newWords
}

var usernameRegEx = regexp.MustCompile("\\w+")

func usernameMutator(line string) []string {
	words := usernameRegEx.FindAllString(line, -1)
	var newUsernames []string

	addWithSeperator := func(sep string) {
		allWords := strings.Join(words, sep)
		newUsernames = append(newUsernames, allWords)
	}

	addWithSeperator("")
	addWithSeperator("_")
	addWithSeperator(".")
	addWithSeperator("-")

	for i := range words {
		prev := words[:i]
		middle := words[i]
		last := words[i+1:]
		if len(prev) == 0 && len(last) == 0 {
			continue
		}
		username := strings.Join(prev, "") + string([]rune(middle)[0]) + strings.Join(last, "")
		newUsernames = append(newUsernames, username)
		username = strings.Join(prev, "") + strings.Join(last, "")
		newUsernames = append(newUsernames, username)
	}
	return newUsernames
}

func findAllWords(settings *settingsWordlist) map[string]bool {
	var files []string
	err := filepath.Walk(settings.Input, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})

	checkError(err)
	wordMap := map[string]bool{}

	addWords := func(words []string, file string) {
		for _, word := range words {
			wordStr := strings.ToLower(word)
			wordStr = strings.TrimSpace(wordStr)
			if file != "" {
				wordMap[wordStr+" ["+file+"]"] = true
			} else {
				wordMap[wordStr] = true
			}
		}
	}

	for _, file := range files {
		stat, err := os.Stat(file)
		logError(err)
		if stat.IsDir() {
			continue
		}

		fileContent, err := ioutil.ReadFile(file)
		logError(err)
		textContent := string(fileContent)

		var words []string

		switch settings.Extractor {
		case "url_rel":
			words = regFindUrlsRel.FindAllString(textContent, -1)
		case "url_abs":
			words = regFindUrlsAbs.FindAllString(textContent, -1)
		case "url":
			words = regFindUrlsRel.FindAllString(textContent, -1)
			words2 := regFindUrlsAbs.FindAllString(textContent, -1)
			words = append(words, words2...)
		case "email":
			words = regFindEmail.FindAllString(textContent, -1)
		case "word":
			words = regFindWordLower.FindAllString(textContent, -1)
		case "string":
			words = regFindStringsOnly.FindAllString(textContent, -1)
			words2 := regFindStringsOnly2.FindAllString(textContent, -1)
			words = append(words, words2...)
			wordsCleared := make([]string, len(words))
			for _, t := range words {
				wordsCleared = append(wordsCleared, strings.Trim(t, "\"'"))
			}
			words = wordsCleared
		case "none":
			words = strings.Split(textContent, "\n")
		default:
			fmt.Print(settings.Extractor + " not found")
		}

		if settings.ShowFileName {
			addWords(words, file)
		} else {
			addWords(words, "")
		}

	}
	return wordMap
}

func logError(err error) {
	if err != nil {
		log.Println(err)
	}
}
