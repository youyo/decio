package decision

import (
	"errors"
	"fmt"
	"strconv"
)

// Type identifies the shape of a decision.
type Type string

const (
	ChoiceType  Type = "choice"
	BooleanType Type = "boolean"
	ScoreType   Type = "score"
	TypeChoice       = ChoiceType
	TypeBoolean      = BooleanType
	TypeScore        = ScoreType

	// Boolean and Score are short aliases for the non-ambiguous type names.
	Boolean = BooleanType
	Score   = ScoreType
)

// Choice is an allowed choice value and its human-readable description.
type Choice struct {
	ID          string
	Description string
}

// ScoreRange is an inclusive score range.
type ScoreRange struct {
	Min int
	Max int
}

// Input is the normalized context sent to a provider.
type Input struct {
	Text  string
	Named map[string]string
	Order []string
}

// Request describes the decision a provider must make.
type Request struct {
	Type    Type
	Prompt  string
	Input   Input
	Choices []Choice
	Range   *ScoreRange
	Levels  []string
}

// Result is a provider's typed decision.
type Result struct {
	Type       Type
	Value      any
	Confidence *float64
	Provider   string
	Model      string
}

// DefaultLevels are the standard labels for qualitative score prompts.
var DefaultLevels = []string{"very low", "low", "moderate", "high", "very high"}

// ErrInvalidResult indicates a result that does not satisfy a request.
var ErrInvalidResult = errors.New("invalid decision result")

// String returns the shell-friendly representation of a result value.
func (r Result) String() string {
	switch value := r.Value.(type) {
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case int:
		return strconv.Itoa(value)
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(value)
	default:
		return fmt.Sprint(value)
	}
}

// Validate checks that res is a valid answer to req.
func Validate(req Request, res Result) error {
	if req.Type != res.Type {
		return fmt.Errorf("%w: result type %q does not match request type %q", ErrInvalidResult, res.Type, req.Type)
	}

	switch req.Type {
	case ChoiceType:
		value, ok := res.Value.(string)
		if !ok {
			return fmt.Errorf("%w: choice value must be a string", ErrInvalidResult)
		}
		for _, choice := range req.Choices {
			if choice.ID == value {
				return nil
			}
		}
		return fmt.Errorf("%w: choice %q is not allowed", ErrInvalidResult, value)
	case BooleanType:
		if _, ok := res.Value.(bool); !ok {
			return fmt.Errorf("%w: boolean value must be bool", ErrInvalidResult)
		}
		return nil
	case ScoreType:
		value, ok := res.Value.(int)
		if !ok {
			return fmt.Errorf("%w: score value must be int", ErrInvalidResult)
		}
		if req.Range == nil || value < req.Range.Min || value > req.Range.Max {
			return fmt.Errorf("%w: score %d is outside the requested range", ErrInvalidResult, value)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown request type %q", ErrInvalidResult, req.Type)
	}
}
