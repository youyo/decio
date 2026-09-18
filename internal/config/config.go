package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/youyo/decio/internal/decision"
	"gopkg.in/yaml.v3"
)

var ErrConfig = errors.New("configuration error")

type Provider struct {
	Type  string `yaml:"type"`
	Model string `yaml:"model"`
}

type Source struct {
	Stdin   bool
	Command string
	File    string
	Literal string
	Timeout time.Duration
}

type Input struct {
	Sources map[string]Source
	Order   []string
}

type Choice struct {
	Description string `yaml:"description"`
}

type Decision struct {
	Type    decision.Type
	Prompt  string
	Choices map[string]Choice
	Range   *decision.ScoreRange
	Levels  []string
}

type Action struct {
	Command string            `yaml:"command"`
	Stdin   string            `yaml:"stdin"`
	Env     map[string]string `yaml:"env"`
}

// Actions contains actions for choice and boolean decisions.
type Actions = map[string]Action

type ScoreAction struct {
	Min     int               `yaml:"-"`
	Max     int               `yaml:"-"`
	HasMax  bool              `yaml:"-"`
	Command string            `yaml:"command"`
	Stdin   string            `yaml:"stdin"`
	Env     map[string]string `yaml:"env"`
}

// ScoreActions contains inclusive score dispatch ranges.
type ScoreActions = []ScoreAction

type Config struct {
	Version  int
	Provider Provider
	Input    Input
	Decision Decision
	Actions  any

	sourceKinds map[string]sourceKind
}

type sourceKind struct {
	stdin, command, file, literal bool
}

type rawConfig struct {
	Version  int         `yaml:"version"`
	Provider Provider    `yaml:"provider"`
	Input    rawInput    `yaml:"input"`
	Decision rawDecision `yaml:"decision"`
	Actions  yaml.Node   `yaml:"actions"`
}

type rawInput struct {
	Sources map[string]rawSource `yaml:"sources"`
}

type rawSource struct {
	Stdin   *bool   `yaml:"stdin"`
	Command *string `yaml:"command"`
	File    *string `yaml:"file"`
	Literal *string `yaml:"literal"`
	Timeout string  `yaml:"timeout"`
}

type rawDecision struct {
	Type    decision.Type        `yaml:"type"`
	Prompt  string               `yaml:"prompt"`
	Choices map[string]Choice    `yaml:"choices"`
	Range   *decision.ScoreRange `yaml:"range"`
	Levels  []string             `yaml:"levels"`
}

type rawScoreAction struct {
	Min     int               `yaml:"min"`
	Max     *int              `yaml:"max"`
	Command string            `yaml:"command"`
	Stdin   string            `yaml:"stdin"`
	Env     map[string]string `yaml:"env"`
}

// Load parses a YAML configuration with strict field checking.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrConfig, path, err)
	}

	var raw rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: decode %s: %v", ErrConfig, path, err)
	}

	var document yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&document); err != nil {
		return nil, fmt.Errorf("%w: decode %s: %v", ErrConfig, path, err)
	}

	cfg := &Config{
		Version:  raw.Version,
		Provider: raw.Provider,
		Decision: Decision{
			Type:    raw.Decision.Type,
			Prompt:  raw.Decision.Prompt,
			Choices: raw.Decision.Choices,
			Range:   raw.Decision.Range,
			Levels:  raw.Decision.Levels,
		},
		Input: Input{
			Sources: make(map[string]Source, len(raw.Input.Sources)),
		},
		sourceKinds: make(map[string]sourceKind, len(raw.Input.Sources)),
	}

	for name, source := range raw.Input.Sources {
		converted, err := convertSource(source)
		if err != nil {
			return nil, fmt.Errorf("%w: source %q: %v", ErrConfig, name, err)
		}
		cfg.Input.Sources[name] = converted
		cfg.sourceKinds[name] = sourceKind{
			stdin:   source.Stdin != nil && *source.Stdin,
			command: source.Command != nil,
			file:    source.File != nil,
			literal: source.Literal != nil,
		}
	}
	if inputNode := mappingValue(documentRoot(&document), "input"); inputNode != nil {
		if sourcesNode := mappingValue(inputNode, "sources"); sourcesNode != nil && sourcesNode.Kind == yaml.MappingNode {
			for index := 0; index+1 < len(sourcesNode.Content); index += 2 {
				cfg.Input.Order = append(cfg.Input.Order, sourcesNode.Content[index].Value)
			}
		}
	}

	if raw.Actions.Kind != 0 && raw.Actions.Tag != "!!null" {
		actions, err := decodeActions(raw.Actions, cfg.Decision.Type)
		if err != nil {
			return nil, fmt.Errorf("%w: actions: %v", ErrConfig, err)
		}
		cfg.Actions = actions
	}
	return cfg, nil
}

func convertSource(source rawSource) (Source, error) {
	converted := Source{}
	if source.Stdin != nil {
		converted.Stdin = *source.Stdin
	}
	if source.Command != nil {
		converted.Command = *source.Command
	}
	if source.File != nil {
		converted.File = *source.File
	}
	if source.Literal != nil {
		converted.Literal = *source.Literal
	}
	if source.Timeout != "" {
		timeout, err := time.ParseDuration(source.Timeout)
		if err != nil {
			return Source{}, fmt.Errorf("invalid timeout %q", source.Timeout)
		}
		converted.Timeout = timeout
	}
	return converted, nil
}

func documentRoot(document *yaml.Node) *yaml.Node {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) == 0 {
		return nil
	}
	return document.Content[0]
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func decodeActions(node yaml.Node, kind decision.Type) (any, error) {
	switch kind {
	case decision.ChoiceType, decision.BooleanType:
		var actions Actions
		if err := decodeStrictNode(node, &actions); err != nil {
			return nil, err
		}
		return actions, nil
	case decision.ScoreType:
		var rawActions []rawScoreAction
		if err := decodeStrictNode(node, &rawActions); err != nil {
			return nil, err
		}
		actions := make(ScoreActions, 0, len(rawActions))
		for _, action := range rawActions {
			converted := ScoreAction{
				Min: action.Min, Command: action.Command, Stdin: action.Stdin, Env: action.Env,
			}
			if action.Max != nil {
				converted.Max = *action.Max
				converted.HasMax = true
			}
			actions = append(actions, converted)
		}
		return actions, nil
	default:
		return nil, fmt.Errorf("unknown decision type %q", kind)
	}
}

func decodeStrictNode(node yaml.Node, destination any) error {
	data, err := yaml.Marshal(&node)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(destination)
}

// Find returns the first conventional config file in the current directory.
func Find() (string, bool) {
	for _, name := range []string{".decio.yaml", ".decio.yml"} {
		info, err := os.Stat(name)
		if err == nil && !info.IsDir() {
			return name, true
		}
	}
	return "", false
}

// Validate checks semantic constraints that YAML decoding cannot express.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: configuration is nil", ErrConfig)
	}
	if c.Version != 0 && c.Version != 1 {
		return fmt.Errorf("%w: unsupported version %d", ErrConfig, c.Version)
	}
	for name, source := range c.Input.Sources {
		if source.Timeout < 0 {
			return fmt.Errorf("%w: source %q timeout cannot be negative", ErrConfig, name)
		}
		kind := c.sourceKinds[name]
		if c.sourceKinds == nil {
			kind = sourceKind{
				stdin:   source.Stdin,
				command: source.Command != "",
				file:    source.File != "",
				literal: source.Literal != "",
			}
		}
		count := 0
		if kind.stdin {
			count++
		}
		if kind.command {
			count++
		}
		if kind.file {
			count++
		}
		if kind.literal {
			count++
		}
		if count != 1 {
			return fmt.Errorf("%w: source %q must specify exactly one source type", ErrConfig, name)
		}
		if kind.command && strings.TrimSpace(source.Command) == "" {
			return fmt.Errorf("%w: source %q command is required", ErrConfig, name)
		}
		if kind.file && strings.TrimSpace(source.File) == "" {
			return fmt.Errorf("%w: source %q file is required", ErrConfig, name)
		}
	}

	switch c.Decision.Type {
	case decision.ChoiceType:
		if len(c.Decision.Choices) < 2 {
			return fmt.Errorf("%w: choice decisions require at least two choices", ErrConfig)
		}
	case decision.BooleanType:
	case decision.ScoreType:
		if c.Decision.Range == nil || c.Decision.Range.Min >= c.Decision.Range.Max {
			return fmt.Errorf("%w: score range must have min < max", ErrConfig)
		}
		if c.Decision.Levels != nil && len(c.Decision.Levels) < 2 {
			return fmt.Errorf("%w: score decisions require at least two levels", ErrConfig)
		}
	default:
		return fmt.Errorf("%w: unknown decision type %q", ErrConfig, c.Decision.Type)
	}

	switch actions := c.Actions.(type) {
	case Actions:
		if c.Decision.Type == decision.ScoreType {
			return fmt.Errorf("%w: score decisions require score actions", ErrConfig)
		}
		for key, action := range actions {
			switch c.Decision.Type {
			case decision.BooleanType:
				if key != "true" && key != "false" {
					return fmt.Errorf("%w: boolean action key %q must be true or false", ErrConfig, key)
				}
			case decision.ChoiceType:
				if _, ok := c.Decision.Choices[key]; !ok {
					return fmt.Errorf("%w: action key %q is not a configured choice", ErrConfig, key)
				}
			}
			if err := validateAction(key, action); err != nil {
				return err
			}
		}
	case ScoreActions:
		if c.Decision.Type == decision.ChoiceType || c.Decision.Type == decision.BooleanType {
			return fmt.Errorf("%w: %s decisions require map actions", ErrConfig, c.Decision.Type)
		}
		for index, action := range actions {
			if err := validateScoreAction(index, action); err != nil {
				return err
			}
		}
		if err := validateScoreOverlap(actions); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("%w: unsupported actions shape %T", ErrConfig, c.Actions)
	}
	return nil
}

func validateAction(key string, action Action) error {
	if strings.TrimSpace(action.Command) == "" {
		return fmt.Errorf("%w: action %q command is required", ErrConfig, key)
	}
	if action.Stdin != "" && action.Stdin != "none" && action.Stdin != "original" {
		return fmt.Errorf("%w: action %q has invalid stdin %q", ErrConfig, key, action.Stdin)
	}
	return nil
}

func validateScoreAction(index int, action ScoreAction) error {
	if strings.TrimSpace(action.Command) == "" {
		return fmt.Errorf("%w: score action %d command is required", ErrConfig, index)
	}
	if scoreActionHasMax(action) && action.Min > action.Max {
		return fmt.Errorf("%w: score action %d has min > max", ErrConfig, index)
	}
	if action.Stdin != "" && action.Stdin != "none" && action.Stdin != "original" {
		return fmt.Errorf("%w: score action %d has invalid stdin %q", ErrConfig, index, action.Stdin)
	}
	return nil
}

func validateScoreOverlap(actions ScoreActions) error {
	for left := 0; left < len(actions); left++ {
		for right := left + 1; right < len(actions); right++ {
			leftMax, rightMax := actions[left].Max, actions[right].Max
			if !scoreActionHasMax(actions[left]) {
				leftMax = int(^uint(0) >> 1)
			}
			if !scoreActionHasMax(actions[right]) {
				rightMax = int(^uint(0) >> 1)
			}
			if actions[left].Min <= rightMax && actions[right].Min <= leftMax {
				return fmt.Errorf("%w: score actions %d and %d overlap", ErrConfig, left, right)
			}
		}
	}
	return nil
}

func scoreActionHasMax(action ScoreAction) bool {
	return action.HasMax || action.Max != 0
}
