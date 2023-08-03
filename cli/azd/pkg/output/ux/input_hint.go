package ux

import (
	"fmt"
	"strings"

	"github.com/azure/azure-dev/cli/azd/pkg/output"
	"github.com/nathan-fiscaletti/consolesize-go"
)

const frameChar = "─"

func ConsoleLine(text string, indent int) string {
	width, _ := consolesize.GetConsoleSize()
	if indent > 0 && len(text) > 0 && text[0] != ' ' {
		text = " " + text
	}

	if len(text) > 0 && text[len(text)-1] != ' ' {
		text = text + " "
	}

	return fmt.Sprintf(
		"%s%s%s",
		strings.Repeat(frameChar, indent),
		text,
		strings.Repeat(frameChar, width-len(text)-indent))
}

type InputHint struct {
	Title string

	Text string

	Examples []string
}

func (i InputHint) ToString() string {
	sb := strings.Builder{}
	sb.WriteString(output.WithBold(i.Title))
	sb.WriteString("\n")
	sb.WriteString(i.Text)

	if len(i.Text) > 0 && i.Text[len(i.Text)-1:] != "\n" {
		sb.WriteString("\n")
	}

	if len(i.Examples) > 0 {
		sb.WriteString("\n")
		sb.WriteString(output.WithBold("Examples:\n  "))
		sb.WriteString(strings.Join(i.Examples, "\n  "))
		sb.WriteString("\n")
	}

	return sb.String()
}
