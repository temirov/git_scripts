package audit

import (
	"bufio"
	"io"
	"strings"
)

// IOConfirmationPrompter reads confirmation responses from an io.Reader.
type IOConfirmationPrompter struct {
	reader *bufio.Reader
	writer io.Writer
}

// NewIOConfirmationPrompter constructs a prompter from the provided reader and writer.
func NewIOConfirmationPrompter(input io.Reader, output io.Writer) *IOConfirmationPrompter {
	return &IOConfirmationPrompter{reader: bufio.NewReader(input), writer: output}
}

// Confirm writes the prompt and interprets affirmative responses (y/yes).
func (prompter *IOConfirmationPrompter) Confirm(prompt string) (bool, error) {
	if prompter.writer != nil {
		if _, writeError := io.WriteString(prompter.writer, prompt); writeError != nil {
			return false, writeError
		}
	}

	response, readError := prompter.reader.ReadString('\n')
	if readError != nil && readError != io.EOF {
		return false, readError
	}

	trimmedResponse := strings.TrimSpace(strings.ToLower(response))
	switch trimmedResponse {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
