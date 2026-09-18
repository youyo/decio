package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/youyo/decio/internal/decision"
)

type ActionOutcome struct {
	Executed bool `json:"executed"`
	ExitCode int  `json:"exit_code"`
}

type jsonOutput struct {
	Type       decision.Type  `json:"type"`
	Value      any            `json:"value"`
	Confidence *float64       `json:"confidence,omitempty"`
	Provider   string         `json:"provider"`
	Model      string         `json:"model"`
	Action     *ActionOutcome `json:"action,omitempty"`
}

// Write emits a normalized result in plain or JSON form.
func Write(w io.Writer, result decision.Result, action *ActionOutcome, asJSON bool) error {
	if !asJSON {
		_, err := fmt.Fprintln(w, result.String())
		return err
	}
	return json.NewEncoder(w).Encode(jsonOutput{
		Type:       result.Type,
		Value:      result.Value,
		Confidence: result.Confidence,
		Provider:   result.Provider,
		Model:      result.Model,
		Action:     action,
	})
}
