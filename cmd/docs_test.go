package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	configpkg "github.com/youyo/decio/internal/config"
)

func TestDocsListMatchesEmbeddedTopics(t *testing.T) {
	stdout, stderr, err := executeTestCommand(t, []string{"docs", "--list"}, "")
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	files, err := docsContent.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	var embedded []string
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".md" {
			continue
		}
		content, err := docsContent.ReadFile(filepath.Join("docs", file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(content)) == "" {
			t.Fatalf("embedded topic %q is empty", file.Name())
		}
		if !strings.HasPrefix(string(content), "# ") {
			t.Fatalf("embedded topic %q must start with a level-one heading", file.Name())
		}
		embedded = append(embedded, strings.TrimSuffix(file.Name(), ".md"))
	}
	sort.Strings(embedded)

	var listed []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 || strings.TrimSpace(fields[1]) == "" {
			t.Fatalf("list line = %q, want name<TAB>description", line)
		}
		listed = append(listed, fields[0])
	}
	sort.Strings(listed)

	if strings.Join(listed, "\n") != strings.Join(embedded, "\n") {
		t.Fatalf("listed topics = %v, embedded topics = %v", listed, embedded)
	}
}

func TestDocsConfigurationExamplesValidate(t *testing.T) {
	yamlBlocks := regexp.MustCompile("(?s)```yaml\\n(.*?)```")
	for _, topic := range docTopics {
		content, err := docsContent.ReadFile("docs/" + topic.Name + ".md")
		if err != nil {
			t.Fatal(err)
		}
		for index, match := range yamlBlocks.FindAllSubmatch(content, -1) {
			if !strings.Contains(string(match[1]), "version:") {
				continue
			}
			path := filepath.Join(t.TempDir(), topic.Name+"-"+strconv.Itoa(index)+".yaml")
			if err := os.WriteFile(path, match[1], 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := configpkg.Load(path)
			if err != nil {
				t.Fatalf("topic %q block %d load = %v", topic.Name, index, err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("topic %q block %d validate = %v", topic.Name, index, err)
			}
		}
	}
}

func TestDocsCLIContainsEveryRootFlag(t *testing.T) {
	content, err := docsContent.ReadFile("docs/cli.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range rootFlagNames(NewRootCmd().Flags().FlagUsages()) {
		if !strings.Contains(string(content), "--"+name) {
			t.Errorf("cli documentation does not contain --%s", name)
		}
	}
}

func rootFlagNames(usages string) []string {
	var names []string
	for _, line := range strings.Split(usages, "\n") {
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "--") {
				name := strings.TrimSuffix(field, ",")
				names = append(names, strings.TrimPrefix(name, "--"))
			}
		}
	}
	return names
}

func TestDocsUnknownTopicReturnsUsageExitCode(t *testing.T) {
	_, _, err := executeTestCommand(t, []string{"docs", "unknown"}, "")
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(err.Error(), `unknown topic "unknown" (available:`) {
		t.Fatalf("error = %q, want unknown-topic message", err)
	}
}
