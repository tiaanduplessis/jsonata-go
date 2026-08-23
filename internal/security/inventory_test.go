package security_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const pinnedCommit = "6c7e95fdbf4405a1e741852a7cd8cd985b4305bb"

type securityInventory struct {
	SchemaVersion   int            `json:"schemaVersion"`
	ReferenceName   string         `json:"referenceName"`
	ReferenceCommit string         `json:"referenceCommit"`
	Risks           []securityRisk `json:"risks"`
}

type securityRisk struct {
	ID            string     `json:"id"`
	Category      string     `json:"category"`
	Source        string     `json:"source"`
	Applicability string     `json:"applicability"`
	Status        string     `json:"status"`
	Evidence      []evidence `json:"evidence"`
}

type evidence struct {
	File string `json:"file"`
	Test string `json:"test"`
}

func TestSecurityInventoryIsCompleteAndExecutable(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(repositoryRoot, "reports", "security-regressions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory securityInventory
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.SchemaVersion != 1 || inventory.ReferenceName != "jsonata-js-v2.2.2" || inventory.ReferenceCommit != pinnedCommit {
		t.Fatal("security inventory is not tied to the pinned reference")
	}
	required := map[string]bool{
		"GHSA-2943-5xfg-gq5f":            false,
		"GHSA-86vw-mfpg-wwv9":            false,
		"prototype-and-constructor-keys": false,
		"reserved-internal-flags":        false,
		"wildcard-function-boundary":     false,
		"evaluation-resource-guardrails": false,
		"append-sequence-guardrail":      false,
		"evaluation-local-root":          false,
		"hostile-cyclic-go-input":        false,
	}
	for _, risk := range inventory.Risks {
		if _, ok := required[risk.ID]; !ok {
			t.Fatalf("unreviewed security inventory entry %q", risk.ID)
		}
		if required[risk.ID] {
			t.Fatalf("duplicate security inventory entry %q", risk.ID)
		}
		required[risk.ID] = true
		if risk.Category == "" || risk.Source == "" || risk.Applicability != "applicable" || risk.Status != "covered" || len(risk.Evidence) == 0 {
			t.Fatalf("incomplete security inventory entry: %#v", risk)
		}
		for _, item := range risk.Evidence {
			source, readErr := os.ReadFile(filepath.Join(repositoryRoot, item.File))
			if readErr != nil {
				t.Fatalf("%s evidence: %v", risk.ID, readErr)
			}
			declaration := []byte("func " + item.Test + "(")
			if item.Test == "" || !bytes.Contains(source, declaration) {
				t.Fatalf("%s evidence test %q is missing from %s", risk.ID, item.Test, item.File)
			}
		}
	}
	for id, covered := range required {
		if !covered {
			t.Fatalf("security inventory is missing %q", id)
		}
	}
}
