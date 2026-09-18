package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/youyo/decio/internal/decision"
)

func TestWritePlain(t *testing.T) {
	tests := []struct {
		name string
		res  decision.Result
		want string
	}{
		{name: "choice", res: decision.Result{Type: decision.ChoiceType, Value: "security"}, want: "security\n"},
		{name: "boolean", res: decision.Result{Type: decision.Boolean, Value: false}, want: "false\n"},
		{name: "score", res: decision.Result{Type: decision.Score, Value: 87}, want: "87\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Write(&out, tt.res, nil, false); err != nil {
				t.Fatal(err)
			}
			if out.String() != tt.want {
				t.Fatalf("Write() = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

func TestWriteJSONOmitsOptionalFields(t *testing.T) {
	var out bytes.Buffer
	res := decision.Result{Type: decision.Boolean, Value: true, Provider: "jev", Model: "default"}
	if err := Write(&out, res, nil, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["confidence"]; ok {
		t.Fatal("JSON contains omitted confidence")
	}
	if _, ok := got["action"]; ok {
		t.Fatal("JSON contains omitted action")
	}
	if got["value"] != true {
		t.Fatalf("JSON value = %#v", got["value"])
	}
}

func TestWriteJSONIncludesAction(t *testing.T) {
	var out bytes.Buffer
	if err := Write(&out, decision.Result{Type: decision.Score, Value: 9}, &ActionOutcome{Executed: true, ExitCode: 0}, true); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Action *ActionOutcome `json:"action"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Action == nil || !got.Action.Executed || got.Action.ExitCode != 0 {
		t.Fatalf("JSON action = %#v", got.Action)
	}
}
