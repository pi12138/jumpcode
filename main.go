package main

import (
	"fmt"
	"os"
)

func main() {
	handleOpenChild(os.Args[1:])
	selectLanguage(os.Args[1:])
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "jc:", localizeError(err))
		os.Exit(1)
	}
}
