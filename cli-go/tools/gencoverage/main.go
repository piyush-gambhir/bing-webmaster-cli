package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/cmd"
)

func main() {
	fmt.Print(cmd.APICoverageMarkdown(cmd.NewRoot(strings.NewReader(""), io.Discard, io.Discard)))
}
