package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type FuzzingInput struct {
	Vectors    map[string][]string
	Iterations int
	BuildLogic string // bsxsx
	Seed       int64
}

type Enc func(string) []byte

func mainFuzzer() {
	fs := flag.NewFlagSet("fuzzer", flag.ContinueOnError)
	input := fs.String("input", "./config/fuzzinginput.json", "fuzzing blocks input file")
	output := fs.String("output", "", "output file")
	param := fs.String("param", "", "parameter to replace")
	fs.Parse(os.Args[2:])

	inputContent, err := ioutil.ReadFile(*input)
	checkError(err)
	fi := FuzzingInput{}
	err = json.Unmarshal(inputContent, &fi)
	if err != nil {
		printSyntaxError(string(inputContent), err)
	}
	encodings := []Enc{NoEncode}

	genFuzzingOutput(&fi, encodings, func(bytesout []byte) bool {
		if *output != "" {
			err := ioutil.WriteFile(*output, bytesout, 0666)
			if err != nil {
				log.Println(err)
				return false
			}
		}

		args := make([]string, len(fs.Args()))
		copy(args, fs.Args())

		if len(args) >= 0 {
			if *param != "" {
				for k, v := range args {
					args[k] = strings.Replace(v, *param, string(bytesout), -1)
				}
			}
			log.Printf("%#v %#v", args[0], args[1:])
			cmd := exec.Command(args[0], args[1:]...)
			err = cmd.Run()
			if err != nil {
				log.Println(err)
				return false
			}
		}
		return true
	})
}

func UrlEncode(text string) []byte {
	return []byte(url.QueryEscape(text))
}

func NoEncode(text string) []byte {
	return []byte(text)
}

func HtmlEncode(text string) []byte {
	return []byte(html.EscapeString(text))
}

type ActionFunc func([]byte) bool

func genFuzzingOutput(fi *FuzzingInput, encodings []Enc, action ActionFunc) {
	nextType := 'x'
	hasBuildLogic := len(fi.BuildLogic) > 0
	rand.Seed(fi.Seed)
	text := ""
	iterPerRun := len(fi.BuildLogic)
	keys := getKeys(fi.Vectors)

	for x := 0; x < fi.Iterations; x++ {
		var b bytes.Buffer
		for y := 0; y < iterPerRun; y++ {
			if hasBuildLogic {
				nextType = infinityString(fi.BuildLogic, y)
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
		action(b.Bytes())
	}
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

func printSyntaxError(js string, err error) {
	syntax, ok := err.(*json.SyntaxError)
	if !ok {
		fmt.Println(err)
		return
	}

	start, end := strings.LastIndex(js[:syntax.Offset], "\n")+1, len(js)
	if idx := strings.Index(js[start:], "\n"); idx >= 0 {
		end = start + idx
	}

	line, pos := strings.Count(js[:start], "\n"), int(syntax.Offset)-start-1

	fmt.Printf("Error in line %d: %s \n", line, err)
	fmt.Printf("%s\n%s^", js[start:end], strings.Repeat(" ", pos))
}
