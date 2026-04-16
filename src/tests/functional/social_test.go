package functional

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSocialSubcommandHelp(t *testing.T) {
	ensureMstackBuilt(t)

	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "social help",
			args:     []string{"social", "--help"},
			expected: []string{"x", "fb", "ig", "campaign", "check"},
		},
		{
			name:     "social x help",
			args:     []string{"social", "x", "--help"},
			expected: []string{"show", "post", "delete", "update"},
		},
		{
			name:     "social fb help",
			args:     []string{"social", "fb", "--help"},
			expected: []string{"show", "post", "delete", "update"},
		},
		{
			name:     "social ig help",
			args:     []string{"social", "ig", "--help"},
			expected: []string{"show", "post", "delete"},
		},
		{
			name:     "social campaign help",
			args:     []string{"social", "campaign", "--help"},
			expected: []string{"list", "post", "retract"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(mstackBin, tc.args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\n%s", err, out)
			}

			output := string(out)
			for _, exp := range tc.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("Expected '%s' in output, got: %s", exp, output)
				}
			}
		})
	}
}

func TestSecretsSubcommandHelp(t *testing.T) {
	ensureMstackBuilt(t)

	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "secrets help",
			args:     []string{"secrets", "--help"},
			expected: []string{"push", "pull", "list", "diff"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(mstackBin, tc.args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\n%s", err, out)
			}

			output := string(out)
			for _, exp := range tc.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("Expected '%s' in output, got: %s", exp, output)
				}
			}
		})
	}
}

func TestSocialXPostDryRun(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "social", "x", "post", "Test message", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "dry run") {
		t.Errorf("Expected 'dry run' in output, got: %s", output)
	}
}

func TestSocialFBPostDryRun(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "social", "fb", "post", "Test message", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "dry run") {
		t.Errorf("Expected 'dry run' in output, got: %s", output)
	}
}

func TestSocialIGPostRequiresImage(t *testing.T) {
	ensureMstackBuilt(t)

	// Post without image should fail
	cmd := exec.Command(mstackBin, "social", "ig", "post", "Test caption", "--dry-run")
	out, err := cmd.CombinedOutput()

	// Should fail because --image is required
	if err == nil {
		t.Error("Expected error when posting to Instagram without image")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "image") {
		t.Errorf("Expected 'image' in error message, got: %s", output)
	}
}

func TestSocialCampaignPostDryRun(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "social", "campaign", "post", "test-uuid-001", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "dry run") {
		t.Errorf("Expected 'dry run' in output, got: %s", output)
	}
}
