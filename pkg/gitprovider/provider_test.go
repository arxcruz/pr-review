package gitprovider

import (
	"testing"
)

func TestPullRequestMatching(t *testing.T) {
	pr := &PullRequest{
		ID:     1,
		Number: 42,
		Title:  "Feature: Add user auth",
		Author: "alice",
		Labels: []string{"backend", "security", "needs-review"},
	}

	// Test Author matching
	if !pr.Matches(FilterOptions{Authors: []string{"alice"}}) {
		t.Errorf("expected match for author alice")
	}
	if !pr.Matches(FilterOptions{Authors: []string{"ALICE"}}) {
		t.Errorf("expected case-insensitive match for author ALICE")
	}
	if pr.Matches(FilterOptions{Authors: []string{"bob"}}) {
		t.Errorf("did not expect match for author bob")
	}

	// Test Label matching
	if !pr.Matches(FilterOptions{Labels: []string{"backend"}}) {
		t.Errorf("expected match for label backend")
	}
	if !pr.Matches(FilterOptions{Labels: []string{"backend", "security"}}) {
		t.Errorf("expected match for labels backend and security")
	}
	if pr.Matches(FilterOptions{Labels: []string{"backend", "frontend"}}) {
		t.Errorf("did not expect match when frontend label is missing")
	}

	// Test Author + Label matching combined
	if !pr.Matches(FilterOptions{Authors: []string{"alice"}, Labels: []string{"security"}}) {
		t.Errorf("expected match for alice with security label")
	}
	if pr.Matches(FilterOptions{Authors: []string{"bob"}, Labels: []string{"security"}}) {
		t.Errorf("did not expect match for bob with security label")
	}
}
