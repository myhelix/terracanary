package main

import (
	"github.com/myhelix/terracanary/canarrors"
	"github.com/myhelix/terracanary/cmd"
	"github.com/spf13/cobra/doc"
	"io/ioutil"
	"os"
	"strings"
)

var exitIf = canarrors.ExitIf

// Generate documentation
func main() {
	err := doc.GenMarkdownTreeCustom(cmd.RootCmd, ".",
		func(s string) string { return "" },
		func(link string) string {
			if link == "terracanary.md" {
				return "../README.md"
			} else {
				return "docs/" + link
			}
		},
	)
	exitIf(err)
	readmeFile := "../README.md"
	err = os.Rename("terracanary.md", readmeFile)
	readmeBytes, err := ioutil.ReadFile(readmeFile)
	exitIf(err)
	readme := string(readmeBytes)
	section :=
		`#### Directory Layout

Terracanary must be run from a working directory which has subdirectories containing the terraform definitions for each stack. Terracanary will create a ".terracanary" config file when you run "terracanary init" to set up the working directory. So for the above example, the layout would look like:

` + "```" + `
.
|-- database
|   \-- *.tf
|
|-- main
|   \-- *.tf
|
|-- routing
|   \-- *.tf
|
\-- .terracanary
` + "```" + `
`
	readme = strings.Replace(readme, "### Options", section+"\n### Options", 1)
	err = ioutil.WriteFile(readmeFile, []byte(readme), os.ModePerm)
	exitIf(err)
}
