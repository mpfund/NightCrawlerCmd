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
	Bricks       []string
	Spaces       []string
	Surroundings []string
	Iterations   int
	BuildLogic   string // bsxsx
	Seed         int64
}

func mainFuzzer() {
	fs := flag.NewFlagSet("fuzzer", flag.ExitOnError)

	input := fs.String("input", "fuzzinginput.json", "fuzzer blocks input file")
	output := fs.String("output", "output.html", "output file")
	fs.Parse(os.Args[2:])

	inputContent, err := ioutil.ReadFile(*input)
	checkError(err)
	fi := FuzzingInput{}
	err = json.Unmarshal(inputContent, &fi)
	checkError(err)
	outputContent := genFuzzingOutput(&fi)
	ioutil.WriteFile(*output, outputContent, 0666)
}

func genFuzzingOutput(fi *FuzzingInput) []byte {
	b := bytes.Buffer{}
	nextType := 'x'

	rand.Seed(fi.Seed)

	for x := 0; x < fi.Iterations; x++ {
		nextType = infinityString(fi.BuildLogic, x)
		switch nextType {
		case 'b':
			brickN := rand.Intn(len(fi.Bricks))
			b.WriteString(fi.Bricks[brickN])
		case 's':
			spaceN := rand.Intn(len(fi.Spaces))
			b.WriteString(fi.Spaces[spaceN])
		case 'x':
			surrN := rand.Intn(len(fi.Surroundings))
			b.WriteString(fi.Surroundings[surrN])
		default:
			b.WriteString(string(nextType))
		}
	}
	return b.Bytes()
}

func infinityString(text string, pos int) rune {
	pos = pos % len(text)
	return []rune(text)[pos]
}
