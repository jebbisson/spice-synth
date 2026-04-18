package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}
	err := runEdit(os.Args[1])

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage:\n")
	fmt.Fprintf(os.Stderr, "  instruments <path/to/instruments.yaml>\n")
}
