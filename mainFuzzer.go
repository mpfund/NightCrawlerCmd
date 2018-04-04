package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"math/rand"
	"os"
)

type FuzzingInput struct {
	Vectors    map[string][]string
	Iterations int
	BuildLogic string // bsxsx
	Seed       int64
}

type Enc func(string) []byte

func mainFuzzer() {
	fs := flag.NewFlagSet("fuzzer", flag.ExitOnError)

	input := fs.String("input", "fuzzinginput.json", "fuzzer blocks input file")
	output := fs.String("output", "output.txt", "output file")
	fs.Parse(os.Args[2:])

	inputContent, err := ioutil.ReadFile(*input)
	checkError(err)
	fi := FuzzingInput{}
	err = json.Unmarshal(inputContent, &fi)
	checkError(err)
	encodings := []Enc{NoEncode}
	outputContent := genFuzzingOutput(&fi, encodings)
	ioutil.WriteFile(*output, outputContent, 0666)
}

/*func UrlEncode(text string) []byte {
	return []byte(url.QueryEscape(text))
}*/

func NoEncode(text string) []byte {
	return []byte(text)
}

/*
func HtmlEncode(text string) []byte {
	return []byte(html.EscapeString(text))
}*/

func genFuzzingOutput(fi *FuzzingInput, encodings []Enc) []byte {
	b := bytes.Buffer{}
	nextType := 'x'
	hasBuildLogic := len(fi.BuildLogic) > 0
	rand.Seed(fi.Seed)
	text := ""

	keys := getKeys(fi.Vectors)

	for x := 0; x < fi.Iterations; x++ {

		if hasBuildLogic {
			nextType = infinityString(fi.BuildLogic, x)
		} else {
			nextType = []rune(keys[rand.Intn(len(keys))])[0]
		}

		vecs, ok := fi.Vectors[string(nextType)]

		if ok {
			brickN := rand.Intn(len(vecs))
			text = vecs[brickN]
		} else {
			text = string(nextType)
		}

		encodingN := rand.Intn(len(encodings))
		b.Write(encodings[encodingN](text))
	}
	return b.Bytes()
}

func getKeys(dict map[string][]string) []string {
	keys := make([]string, len(dict))

	i := 0
	for k := range dict {
		keys[i] = k
		i++
	}
	return keys
}

func infinityString(text string, pos int) rune {
	pos = pos % len(text)
	return []rune(text)[pos]
}
