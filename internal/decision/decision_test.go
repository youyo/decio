package decision

import (
	"errors"
	"testing"
)

func TestResultString(t *testing.T) {
	tests := []struct {
		name string
		res  Result
		want string
	}{
		{name: "choice", res: Result{Type: ChoiceType, Value: "security"}, want: "security"},
		{name: "true", res: Result{Type: Boolean, Value: true}, want: "true"},
		{name: "false", res: Result{Type: Boolean, Value: false}, want: "false"},
		{name: "score", res: Result{Type: Score, Value: 87}, want: "87"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.res.String(); got != tt.want {
				t.Fatalf("Result.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		res  Result
		want bool
	}{
		{
			name: "choice in set",
			req:  Request{Type: ChoiceType, Choices: []Choice{{ID: "normal"}, {ID: "security"}}},
			res:  Result{Type: ChoiceType, Value: "security"},
		},
		{
			name: "choice outside set",
			req:  Request{Type: ChoiceType, Choices: []Choice{{ID: "normal"}, {ID: "security"}}},
			res:  Result{Type: ChoiceType, Value: "none"},
			want: true,
		},
		{
			name: "boolean wrong type",
			req:  Request{Type: BooleanType},
			res:  Result{Type: BooleanType, Value: "true"},
			want: true,
		},
		{
			name: "score in range",
			req:  Request{Type: ScoreType, Range: &ScoreRange{Min: 0, Max: 100}},
			res:  Result{Type: ScoreType, Value: 100},
		},
		{
			name: "score outside range",
			req:  Request{Type: ScoreType, Range: &ScoreRange{Min: 0, Max: 100}},
			res:  Result{Type: ScoreType, Value: 101},
			want: true,
		},
		{
			name: "result type mismatch",
			req:  Request{Type: ChoiceType, Choices: []Choice{{ID: "yes"}, {ID: "no"}}},
			res:  Result{Type: Boolean, Value: true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.req, tt.res)
			if (err != nil) != tt.want {
				t.Fatalf("Validate() error = %v, want error %v", err, tt.want)
			}
			if tt.want && !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("Validate() error = %v, want ErrInvalidResult", err)
			}
		})
	}
}
