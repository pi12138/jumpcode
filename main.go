package main

import (
	"fmt"
	"os"
)

func main() {
	selectLanguage(os.Args[1:])
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "jc:", localizeError(err))
		os.Exit(1)
	}
}
