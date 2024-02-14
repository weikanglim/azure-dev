// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package contracts

import "time"

type EventDataType string

const (
	ConsoleMessageEventDataType EventDataType = "consoleMessage"
	EndMessageEventDataType     EventDataType = "endMessage"
	PromptEventDataType         EventDataType = "prompt"
)

type EventEnvelope struct {
	Type      EventDataType `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	Data      any           `json:"data"`
}
