package gitprovider

import (
	"encoding/json"
	"testing"
)

func TestStripGerritMagic(t *testing.T) {
	raw := []byte(")]}'\n[{\"id\": \"myproject~master~I12345\"}]")
	cleaned := stripGerritMagic(raw)
	expected := "[{\"id\": \"myproject~master~I12345\"}]"
	if string(cleaned) != expected {
		t.Errorf("expected %s, got %s", expected, string(cleaned))
	}
}

func TestParseGerritChangeJSON(t *testing.T) {
	rawGerritJSON := []byte(`)]}'
[
  {
    "id": "openstack%2Fnova~master~I1234567890",
    "project": "openstack/nova",
    "branch": "master",
    "_number": 890123,
    "subject": "Fix hypervisor live migration timeout",
    "status": "NEW",
    "created": "2026-08-30 10:00:00.000000000",
    "updated": "2026-08-31 08:30:00.000000000",
    "owner": {
      "name": "Alice Developer",
      "username": "alice",
      "email": "alice@example.com"
    },
    "labels": {
      "Code-Review": {
        "all": [{"value": 1}]
      }
    }
  }
]`)

	cleaned := stripGerritMagic(rawGerritJSON)
	var changes []gerritChange
	if err := json.Unmarshal(cleaned, &changes); err != nil {
		t.Fatalf("failed to unmarshal gerrit changes: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	c := changes[0]
	if c.ChangeNumber != 890123 {
		t.Errorf("expected change number 890123, got %d", c.ChangeNumber)
	}
	if c.Owner.Username != "alice" {
		t.Errorf("expected author alice, got %s", c.Owner.Username)
	}
	if c.Project != "openstack/nova" {
		t.Errorf("expected project openstack/nova, got %s", c.Project)
	}
}
