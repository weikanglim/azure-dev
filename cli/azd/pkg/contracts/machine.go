package contracts

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
	Kind PromptKind `json:"kind"`

	// Default value for the prompt
	Default string `json:"default"`

	// Options that the user can choose from
	Options []string `json:"options"`
}
