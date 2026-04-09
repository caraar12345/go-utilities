package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	format := flag.String("format", "", "output format: ttml or ass (default: auto-detect from output extension, else ttml)")
	flag.Usage = func() {
		log.Println("Usage: vtt2ttml [-format ttml|ass] input.vtt [output]")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 || len(args) > 2 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := args[0]
	data, err := os.ReadFile(inputPath)
	if err != nil {
		log.Fatalf("reading input: %v", err)
	}

	vtt, err := ParseVTT(string(data))
	if err != nil {
		log.Fatalf("parsing VTT: %v", err)
	}

	// Determine output format: explicit flag > output extension > default ttml.
	outputFormat := strings.ToLower(*format)
	if outputFormat == "" && len(args) == 2 {
		switch strings.ToLower(filepath.Ext(args[1])) {
		case ".ass", ".ssa":
			outputFormat = "ass"
		case ".ttml", ".dfxp", ".xml":
			outputFormat = "ttml"
		}
	}
	if outputFormat == "" {
		outputFormat = "ttml"
	}

	var result []byte
	switch outputFormat {
	case "ttml":
		result, err = ConvertToTTML(vtt)
	case "ass":
		result, err = ConvertToASS(vtt)
	default:
		log.Fatalf("unknown format %q: must be ttml or ass", outputFormat)
	}
	if err != nil {
		log.Fatalf("converting to %s: %v", outputFormat, err)
	}

	var out io.Writer = os.Stdout
	if len(args) == 2 {
		f, err := os.Create(args[1])
		if err != nil {
			log.Fatalf("creating output: %v", err)
		}
		defer f.Close()
		out = f
	}

	if _, err := out.Write(result); err != nil {
		log.Fatalf("writing output: %v", err)
	}
}
