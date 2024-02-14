package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
)

type EventDataType string

const (
	ConsoleMessageEventDataType EventDataType = "consoleMessage"
	EndMessageEventDataType     EventDataType = "endMessage"
	PromptEventDataType         EventDataType = "prompt"
)

type EventEnvelope struct {
	Type      EventDataType   `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type ConsoleMessage struct {
	Message string `json:"message"`
}

type PromptKind string

const (
	// text input
	PromptKindText PromptKind = "text"

	// yes/no confirmation
	PromptKindConfirm PromptKind = "confirm"

	// single selection
	PromptKindSingle PromptKind = "single"

	// multiselect
	PromptKindMulti PromptKind = "multi"
)

type Prompt struct {
	// Message displayed to the user
	Message string `json:"message"`

	// Kind of prompt (e.g. "text", "password", "list")
	Kind string `json:"kind"`

	// Default value for the prompt
	Default string `json:"default"`

	// Options that the user can choose from
	Options []string `json:"options"`
}

func run() error {
	dir, err := os.MkdirTemp("", "azdclient")
	if err != nil {
		return err
	}

	file, err := os.Create("interactions.log")
	if err != nil {
		return err
	}

	defer file.Close()
	log.SetOutput(file)

	stdin := chanReader{make(chan string, 1)}
	cmd := exec.Command("/home/weilim/repos/sec/cli/azd/azd", "init", "--machine", "--cwd", dir)
	cmd.Stderr = os.Stderr
	cmd.Stdin = &stdin
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(pipe)
	err = cmd.Start()
	if err != nil {
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		log.Print("azd:" + line)

		var envelope EventEnvelope
		err := json.Unmarshal([]byte(line), &envelope)
		if err != nil {
			return err
		}

		switch envelope.Type {
		case ConsoleMessageEventDataType:
			var data ConsoleMessage
			err := json.Unmarshal(envelope.Data, &data)
			if err != nil {
				return err
			}
			if len(data.Message) > 0 && data.Message[len(data.Message)-1] != '\n' {
				data.Message += "\n"
			}

			fmt.Print(data.Message)
		case PromptEventDataType:
			var data Prompt
			err := json.Unmarshal(envelope.Data, &data)
			if err != nil {
				return err
			}

			if err := prompt(data, stdin.ch); err != nil {
				return err
			}
		case EndMessageEventDataType:
			stdin.ch <- "\n\n"
			close(stdin.ch)
			pipe.Close()
			return cmd.Wait()
		default:
			panic("unknown event type: " + envelope.Type)
		}
	}

	if scanner.Err() != nil {
		return scanner.Err()
	}

	close(stdin.ch)
	pipe.Close()
	return cmd.Wait()
}

func prompt(p Prompt, stdin chan<- string) error {
	switch PromptKind(p.Kind) {
	case PromptKindText:
		var response string
		prompt := &survey.Input{
			Message: p.Message,
			Default: p.Default,
		}
		if err := survey.AskOne(prompt, &response); err != nil {
			return err
		}
		stdin <- response + "\n"
	case PromptKindConfirm:
		var response bool
		prompt := &survey.Confirm{
			Message: p.Message,
			Default: p.Default == "true",
		}
		if err := survey.AskOne(prompt, &response); err != nil {
			return err
		}
		stdin <- fmt.Sprintf("%t\n", response)
	case PromptKindSingle:
		response := ""
		prompt := &survey.Select{
			Message: p.Message,
			Options: p.Options,
			//Default: p.Default,
		}
		if err := survey.AskOne(prompt, &response); err != nil {
			return err
		}

		stdin <- response + "\n"
	case PromptKindMulti:
		var response []string
		prompt := &survey.MultiSelect{
			Message: p.Message,
			Options: p.Options,
			Default: p.Default,
		}
		if err := survey.AskOne(prompt, &response); err != nil {
			return err
		}
		stdin <- fmt.Sprintf("%s\n", strings.Join(response, ","))
	default:
		panic("unknown prompt kind: " + string(p.Kind))
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

type chanReader struct {
	ch chan string
}

func (r *chanReader) Read(p []byte) (n int, err error) {
	data, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}

	n = copy(p, []byte(data))
	log.Print("client:" + data)
	return
}
