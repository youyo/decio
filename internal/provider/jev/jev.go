// Package jev implements the TypeSafe Jev decision provider.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/youyo/decio/internal/decision"
	"github.com/youyo/decio/internal/provider"
)

const (
	defaultBaseURL   = "https://api.typesafe.ai"
	defaultModel     = "jev-latest"
	maxRetries       = 3
	statusOverloaded = 529
)

func init() {
	provider.Register("jev", New)
}

type client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

type requestPayload struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]question `json:"questions"`
}

type question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type responsePayload struct {
	Model   string            `json:"model"`
	Answers map[string]answer `json:"answers"`
}

type answer struct {
	Type       string   `json:"type"`
	Noul       *float64 `json:"noul"`
	Choice     *string  `json:"choice"`
	Score      *float64 `json:"score"`
	Confidence *float64 `json:"confidence"`
}

// New constructs a Jev provider using the environment configuration.
func New(model string) (provider.Provider, error) {
	apiKey := os.Getenv("TYPESAFE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("%w: TYPESAFE_API_KEY is not set", provider.ErrConfig)
	}
	if model == "" {
		model = defaultModel
	}
	baseURL := os.Getenv("TYPESAFE_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{},
	}, nil
}

func (c *client) Decide(ctx context.Context, req decision.Request) (decision.Result, error) {
	body, err := json.Marshal(c.request(req))
	if err != nil {
		return decision.Result{}, fmt.Errorf("%w: encode request: %v", provider.ErrConfig, err)
	}

	responseBody, err := c.post(ctx, body)
	if err != nil {
		return decision.Result{}, err
	}

	result, err := c.result(req, responseBody)
	if err != nil {
		return decision.Result{}, err
	}
	return result, nil
}

func (c *client) request(req decision.Request) requestPayload {
	state := any(req.Input.Text)
	if len(req.Input.Named) != 0 {
		named := make(map[string]string, len(req.Input.Named))
		for name, value := range req.Input.Named {
			named[name] = value
		}
		state = named
	}

	return requestPayload{
		State:     state,
		Model:     c.model,
		Questions: map[string]question{"decision": buildQuestion(req)},
	}
}

func buildQuestion(req decision.Request) question {
	result := question{Instructions: req.Prompt}
	switch req.Type {
	case decision.BooleanType:
		result.Type = "noul"
	case decision.ChoiceType:
		result.Type = "choice"
		criteria := make(map[string]*string, len(req.Choices))
		for _, choice := range req.Choices {
			if choice.Description == "" {
				criteria[choice.ID] = nil
				continue
			}
			description := choice.Description
			criteria[choice.ID] = &description
		}
		result.Criteria = criteria
	case decision.ScoreType:
		result.Type = "score"
		levels := req.Levels
		if len(levels) == 0 {
			levels = decision.DefaultLevels
		}
		result.Criteria = levels
	}
	return result
}

func (c *client) post(ctx context.Context, body []byte) ([]byte, error) {
	endpoint := c.baseURL + "/v1/systemone"
	for attempt := 0; attempt <= maxRetries; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("%w: create request: %v", provider.ErrProvider, err)
		}
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
		request.Header.Set("Content-Type", "application/json")

		response, err := c.http.Do(request)
		if err != nil {
			return nil, wrapProviderRequestError(err)
		}

		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == statusOverloaded {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if attempt == maxRetries {
				return nil, fmt.Errorf("%w: status %d after %d retries", provider.ErrProvider, response.StatusCode, maxRetries)
			}
			if err := wait(ctx, retryDelay(response.Header.Get("Retry-After"), attempt)); err != nil {
				return nil, wrapProviderRequestError(err)
			}
			continue
		}

		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return nil, fmt.Errorf("%w: status %d", provider.ErrProvider, response.StatusCode)
		}

		data, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			return nil, wrapProviderRequestError(err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("%w: retry loop exhausted", provider.ErrProvider)
}

func retryDelay(header string, attempt int) time.Duration {
	if header != "" {
		seconds, err := strconv.ParseFloat(strings.TrimSpace(header), 64)
		if err == nil && seconds >= 0 {
			return time.Duration(seconds * float64(time.Second))
		}
	}
	return []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}[attempt]
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *client) result(req decision.Request, body []byte) (decision.Result, error) {
	var response responsePayload
	if err := json.Unmarshal(body, &response); err != nil {
		return decision.Result{}, invalidResult("decode response: %v", err)
	}
	if response.Model == "" {
		return decision.Result{}, invalidResult("response model is missing")
	}
	answer, ok := response.Answers["decision"]
	if !ok {
		return decision.Result{}, invalidResult("answers.decision is missing")
	}

	var result decision.Result
	switch req.Type {
	case decision.BooleanType:
		if answer.Type != "noul" || answer.Noul == nil || !validProbability(*answer.Noul) {
			return decision.Result{}, invalidResult("invalid boolean answer")
		}
		result = decision.Result{Type: decision.BooleanType, Value: *answer.Noul >= 0.5}
	case decision.ChoiceType:
		if answer.Type != "choice" || answer.Choice == nil || answer.Confidence == nil || !validProbability(*answer.Confidence) {
			return decision.Result{}, invalidResult("invalid choice answer")
		}
		result = decision.Result{
			Type:       decision.ChoiceType,
			Value:      *answer.Choice,
			Confidence: answer.Confidence,
		}
	case decision.ScoreType:
		if answer.Type != "score" || answer.Score == nil || answer.Confidence == nil || !validProbability(*answer.Confidence) {
			return decision.Result{}, invalidResult("invalid score answer")
		}
		levels := req.Levels
		if len(levels) == 0 {
			levels = decision.DefaultLevels
		}
		if req.Range == nil || len(levels) < 2 || *answer.Score < 0 || *answer.Score > float64(len(levels)-1) {
			return decision.Result{}, invalidResult("score is outside the requested levels")
		}
		rangeWidth := float64(req.Range.Max) - float64(req.Range.Min)
		value := int(math.Round(float64(req.Range.Min) + (*answer.Score/float64(len(levels)-1))*rangeWidth))
		result = decision.Result{
			Type:       decision.ScoreType,
			Value:      value,
			Confidence: answer.Confidence,
		}
	default:
		return decision.Result{}, invalidResult("unsupported decision type %q", req.Type)
	}

	result.Provider = "jev"
	result.Model = response.Model
	if err := decision.Validate(req, result); err != nil {
		return decision.Result{}, invalidResult("provider result failed validation")
	}
	return result, nil
}

func validProbability(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func invalidResult(format string, args ...any) error {
	return fmt.Errorf("%w: %s", decision.ErrInvalidResult, fmt.Sprintf(format, args...))
}

func wrapProviderRequestError(err error) error {
	if errors.Is(err, provider.ErrProvider) {
		return err
	}
	return fmt.Errorf("%w: %w", provider.ErrProvider, err)
}
