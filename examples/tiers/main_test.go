package main

import (
	"os/exec"
	"strings"
	"testing"
)

func runExample(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, output)
	}
	return string(output)
}

func TestTierExampleOutput(t *testing.T) {
	output := runExample(t, ".")
	for _, want := range []string{
		"escalate wraith -> shade",
		"escalate shade -> veil",
		"release veil -> shade",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q", output)
		}
	}
}
