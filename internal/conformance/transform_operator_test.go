package conformance_test

import (
	"fmt"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func TestFirstFifteenTransformsAgainstPinnedSuite(t *testing.T) {
	manifest := conformance.Manifest{"transforms": {}}
	for index := 0; index < 15; index++ {
		manifest["transforms"][fmt.Sprintf("case%03d", index)] = struct{}{}
	}
	report := conformance.RunWithOptions(loadPinnedSuite(t), realSuiteCompiler{}, manifest, fullSuiteOptions(t))
	assertExactGate(t, report, 15)
}
