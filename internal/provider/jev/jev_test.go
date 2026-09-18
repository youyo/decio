package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youyo/decio/internal/decision"
	"github.com/youyo/decio/internal/provider"
)

func TestNewRequiresAPIKey(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")

	_, err := New("")
	if !errors.Is(err, provider.ErrConfig) {
		t.Fatalf("New() error = %v, want provider.ErrConfig", err)
	}
}

func TestRegistersJev(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", t.Name())

	registered, err := provider.New("jev", "")
	if err != nil {
		t.Fatalf("provider.New() error = %v", err)
	}
	if registered == nil {
		t.Fatal("provider.New() returned nil provider")
	}
}

func TestChoiceRequestAndResponse(t *testing.T) {
	apiKey := t.Name()
	t.Setenv("TYPESAFE_API_KEY", apiKey)

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("path = %q, want /v1/systemone", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Errorf("authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type = %q, want application/json", got)
		}

		var body struct {
			State     map[string]string         `json:"state"`
			Model     string                    `json:"model"`
			Questions map[string]map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if got := body.State["event"]; got != "payload" {
			t.Errorf("state.event = %q, want payload", got)
		}
		if body.Model != "jev-test" {
			t.Errorf("model = %q, want jev-test", body.Model)
		}
		question := body.Questions["decision"]
		if question["type"] != "choice" || question["instructions"] != "pick one" {
			t.Errorf("question = %#v, want choice question", question)
		}
		criteria, ok := question["criteria"].(map[string]any)
		if !ok || criteria["yes"] != "accept" || criteria["no"] != nil {
			t.Errorf("criteria = %#v, want descriptions and null", question["criteria"])
		}

		writeJSON(t, w, map[string]any{
			"model": "jev-response",
			"answers": map[string]any{
				"decision": map[string]any{
					"type":       "choice",
					"choice":     "yes",
					"confidence": 0.91,
				},
			},
		})
	}), "jev-test")
	result, err := client.Decide(context.Background(), decision.Request{
		Type:   decision.ChoiceType,
		Prompt: "pick one",
		Input: decision.Input{
			Named: map[string]string{"event": "payload"},
			Order: []string{"event"},
		},
		Choices: []decision.Choice{
			{ID: "yes", Description: "accept"},
			{ID: "no"},
		},
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if result.Type != decision.ChoiceType || result.Value != "yes" {
		t.Fatalf("result = %#v, want choice yes", result)
	}
	if result.Provider != "jev" || result.Model != "jev-response" {
		t.Fatalf("provider/model = %q/%q, want jev/jev-response", result.Provider, result.Model)
	}
	if result.Confidence == nil || *result.Confidence != 0.91 {
		t.Fatalf("confidence = %v, want 0.91", result.Confidence)
	}
}

func TestBooleanBoundaries(t *testing.T) {
	tests := []struct {
		name string
		noul float64
		want bool
	}{
		{name: "true at boundary", noul: 0.5, want: true},
		{name: "false below boundary", noul: 0.499, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, map[string]any{
					"model": "jev-boolean",
					"answers": map[string]any{
						"decision": map[string]any{
							"type": "noul",
							"noul": tt.noul,
						},
					},
				})
			}), "")
			result, err := client.Decide(context.Background(), decision.Request{
				Type:   decision.BooleanType,
				Prompt: "is it true?",
				Input:  decision.Input{Text: "payload"},
			})
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if result.Value != tt.want {
				t.Fatalf("result.Value = %v, want %v", result.Value, tt.want)
			}
			if result.Confidence != nil {
				t.Fatalf("confidence = %v, want nil", result.Confidence)
			}
		})
	}
}

func TestScoreConversion(t *testing.T) {
	tests := []struct {
		score float64
		want  int
	}{
		{score: 2, want: 50},
		{score: 4, want: 100},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%g", tt.score), func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Questions map[string]map[string]any `json:"questions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request body: %v", err)
				}
				levels, ok := body.Questions["decision"]["criteria"].([]any)
				if !ok || len(levels) != 5 {
					t.Errorf("criteria = %#v, want five levels", body.Questions["decision"]["criteria"])
				}
				writeJSON(t, w, map[string]any{
					"model": "jev-score",
					"answers": map[string]any{
						"decision": map[string]any{
							"type":       "score",
							"score":      tt.score,
							"confidence": 0.8,
						},
					},
				})
			}), "")
			result, err := client.Decide(context.Background(), decision.Request{
				Type:   decision.ScoreType,
				Prompt: "rate it",
				Input:  decision.Input{Text: "payload"},
				Range:  &decision.ScoreRange{Min: 0, Max: 100},
			})
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if result.Value != tt.want {
				t.Fatalf("result.Value = %v, want %v", result.Value, tt.want)
			}
		})
	}
}

func TestScoreConversionDoesNotOverflowIntRange(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"model": "jev-score-extreme",
			"answers": map[string]any{
				"decision": map[string]any{
					"type":       "score",
					"score":      2,
					"confidence": 0.8,
				},
			},
		})
	}, "")

	result, err := client.Decide(context.Background(), decision.Request{
		Type:  decision.ScoreType,
		Range: &decision.ScoreRange{Min: minInt, Max: maxInt},
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	want := int(math.Round(float64(minInt) + 0.5*(float64(maxInt)-float64(minInt))))
	if result.Value != want {
		t.Fatalf("result.Value = %v, want %d", result.Value, want)
	}
}

func TestMalformedJSONIsInvalidResult(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}, "")

	_, err := client.Decide(context.Background(), decision.Request{
		Type:   decision.BooleanType,
		Prompt: "is it true?",
	})
	if !errors.Is(err, decision.ErrInvalidResult) {
		t.Fatalf("Decide() error = %v, want ErrInvalidResult", err)
	}
}

func TestChoiceOutsideRequestIsInvalidResult(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"model": "jev-choice",
			"answers": map[string]any{
				"decision": map[string]any{
					"type":       "choice",
					"choice":     "unexpected",
					"confidence": 0.7,
				},
			},
		})
	}, "")

	_, err := client.Decide(context.Background(), decision.Request{
		Type:    decision.ChoiceType,
		Choices: []decision.Choice{{ID: "allowed"}, {ID: "also-allowed"}},
	})
	if !errors.Is(err, decision.ErrInvalidResult) {
		t.Fatalf("Decide() error = %v, want ErrInvalidResult", err)
	}
}

func TestUnauthorizedIsProviderError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "")

	_, err := client.Decide(context.Background(), decision.Request{Type: decision.BooleanType})
	if !errors.Is(err, provider.ErrProvider) {
		t.Fatalf("Decide() error = %v, want ErrProvider", err)
	}
}

func TestRetriesRateLimitThenSucceeds(t *testing.T) {
	var requests int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, map[string]any{
			"model": "jev-retry",
			"answers": map[string]any{
				"decision": map[string]any{"type": "noul", "noul": 1},
			},
		})
	}, "")

	result, err := client.Decide(context.Background(), decision.Request{Type: decision.BooleanType})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if result.Value != true || requests != 3 {
		t.Fatalf("result/requests = %v/%d, want true/3", result.Value, requests)
	}
}

func TestRetriesRateLimitFourTimesThenFails(t *testing.T) {
	var requests int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}, "")

	_, err := client.Decide(context.Background(), decision.Request{Type: decision.BooleanType})
	if !errors.Is(err, provider.ErrProvider) {
		t.Fatalf("Decide() error = %v, want ErrProvider", err)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want 4", requests)
	}
}

func TestContextTimeoutReturnsError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}, "")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.Decide(ctx, decision.Request{Type: decision.BooleanType})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Decide() error = %v, want context.DeadlineExceeded", err)
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc, model string) provider.Provider {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", t.Name())
	t.Setenv("TYPESAFE_BASE_URL", "http://jev.test")

	providerClient, err := New(model)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	client := providerClient.(*client)
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			handler.ServeHTTP(recorder, r)
			close(done)
		}()
		select {
		case <-done:
			return recorder.Result(), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
