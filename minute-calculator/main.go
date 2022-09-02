package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	hiveTimeFormat = "02/01/06 15:04"
)

func main() {
	args := os.Args[1:]

	runCommand := os.Args[0]

	switch {
	case strings.Contains(runCommand, "T/go-build"):
		runCommand = "go run ."
	}

	if len(args) != 2 {
		log.Printf("Wrong number of args.")
		log.Printf("Usage:   %v \"alertTime\" \"caseTime\"", runCommand)
		log.Fatalf("Example: %v \"02/01/06 15:04\" \"02/01/06 15:04\"", runCommand)
	}

	alertTimeArg := args[0]
	caseTimeArg := args[1]

	alertTime, err := time.Parse(hiveTimeFormat, alertTimeArg)
	if err != nil {
		log.Fatalln(err)
	}

	caseTime, err := time.Parse(hiveTimeFormat, caseTimeArg)
	if err != nil {
		log.Fatalln(err)
	}

	tta := caseTime.Sub(alertTime).Minutes()

	fmt.Printf("Alert time: %s\nCase time: %s\nTTA: %v\n", alertTimeArg, caseTimeArg, tta)
}
