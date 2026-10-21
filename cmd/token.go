package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"jumpcode/internal/i18n"

	"golang.org/x/term"
)

func readPairToken(input io.Reader, output io.Writer) (string, error) {
	fmt.Fprint(output, i18n.T("配对密钥: ", "Pairing token: "))
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		data, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(output)
		return strings.TrimSpace(string(data)), err
	}
	data, err := io.ReadAll(io.LimitReader(input, 256))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
