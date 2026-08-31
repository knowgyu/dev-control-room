package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/knowgyu/dev-control-room/internal/measurement"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify-measurement-contract", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "path to a measurement manifest")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *manifestPath == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "usage: verify-measurement-contract --manifest <path>")
		return 2
	}

	file, err := os.Open(*manifestPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "measurement manifest cannot be opened")
		return 1
	}
	defer file.Close()
	if _, err := measurement.DecodeManifest(file); err != nil {
		_, _ = fmt.Fprintf(stderr, "measurement manifest is invalid: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "PASS measurement contract verified")
	return 0
}
