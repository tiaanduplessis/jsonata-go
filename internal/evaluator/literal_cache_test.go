package evaluator

import (
	"sync"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestCachedLiteralsAreSafeToReuseConcurrently(t *testing.T) {
	node, parseErr := syntax.Parse(`$contains("the quick brown fox", /quick|fox/) and $sum([1, 2, 3]) = 6`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	const workers = 16
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < 100; iteration++ {
				result, err := eval(node, state{runtime: newEvalRuntime(Options{})})
				if err != nil || result != true {
					t.Errorf("eval result=%v err=%v, want true", result, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
