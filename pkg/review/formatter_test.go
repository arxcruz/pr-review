package review

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arxcruz/pr-review/pkg/gitprovider"
)

func TestPrintPullRequestsTable(t *testing.T) {
	prs := []*gitprovider.PullRequest{
		{
			ProjectID:    "ci-framework",
			Number:       123,
			Title:        "Support custom tempest roles",
			Author:       "arxcruz",
			Labels:       []string{"enhancement", "ci"},
			SourceBranch: "feat/tempest",
			TargetBranch: "main",
			URL:          "https://github.com/openstack-k8s-operators/ci-framework/pull/123",
		},
	}

	var buf bytes.Buffer
	PrintPullRequestsTable(&buf, prs)
	output := buf.String()

	if !strings.Contains(output, "ci-framework") {
		t.Errorf("expected output to contain project ID, got: %s", output)
	}
	if !strings.Contains(output, "123") {
		t.Errorf("expected output to contain PR number, got: %s", output)
	}
	if !strings.Contains(output, "arxcruz") {
		t.Errorf("expected output to contain author, got: %s", output)
	}
}
