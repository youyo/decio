package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/youyo/decio/internal/config"
)

func TestInitTemplatesValidate(t *testing.T) {
	for _, decisionType := range []string{"choice", "boolean", "score"} {
		t.Run(decisionType, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "decio.yaml")
			stdout, stderr, err := executeTestCommand(t, []string{
				"init", "--type", decisionType, "--output", path,
			}, "")
			if err != nil {
				t.Fatalf("init = %v", err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "wrote "+path+"\n" {
				t.Fatalf("stderr = %q, want write diagnostic", stderr)
			}

			cfg, err := configpkg.Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestInitRefusesExistingFileWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decio.yaml")
	original := "keep this content\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeTestCommand(t, []string{
		"init", "--type", "choice", "--output", path,
	}, "")
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want no command output", stdout, stderr)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("existing content = %q, want unchanged content", content)
	}
}

func TestInitForceOverwritesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decio.yaml")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := executeTestCommand(t, []string{
		"init", "--type", "boolean", "--output", path, "--force",
	}, "")
	if err != nil {
		t.Fatalf("init --force = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) == "old\n" || !strings.Contains(string(content), "type: boolean") {
		t.Fatalf("forced content = %q, want boolean template", content)
	}
}

func TestInitWritesEmbeddedTemplateToStdout(t *testing.T) {
	expected, err := templates.ReadFile("templates/score.yaml")
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeTestCommand(t, []string{
		"init", "--type", "score",
	}, "")
	if err != nil {
		t.Fatalf("init = %v", err)
	}
	if stdout != string(expected) {
		t.Fatalf("stdout does not match embedded score template")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestInitRejectsUnknownType(t *testing.T) {
	_, _, err := executeTestCommand(t, []string{"init", "--type", "other"}, "")
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
