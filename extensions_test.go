package jsonata

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	bluesjtypes "github.com/blues/jsonata-go/jtypes"
	"github.com/tiaanduplessis/jsonata-go/jtypes"
)

type testExtensionVariant reflect.Value

type compatibilityArgHandler func([]reflect.Value) bool

func (testExtensionVariant) ValidTypes() []reflect.Type {
	return []reflect.Type{reflect.TypeOf(""), reflect.TypeOf(float64(0))}
}

type testLegacyCallable struct{}

func (testLegacyCallable) Name() string    { return "legacyDouble" }
func (testLegacyCallable) ParamCount() int { return 1 }
func (testLegacyCallable) Call(args []reflect.Value) (reflect.Value, error) {
	if len(args) != 1 {
		return reflect.Value{}, errors.New("wrong argument count")
	}
	return reflect.ValueOf(args[0].Float() * 2), nil
}

type testPanicCallable struct{}

func (testPanicCallable) Name() string    { return "legacyPanic" }
func (testPanicCallable) ParamCount() int { return 0 }
func (testPanicCallable) Call([]reflect.Value) (reflect.Value, error) {
	panic("legacy boom")
}

var (
	_ func(map[string]Extension) error            = RegisterExts
	_ func(map[string]interface{}) error          = RegisterVars
	_ func(*Expr, map[string]Extension) error     = (*Expr).RegisterExts
	_ func(*Expr, map[string]interface{}) error   = (*Expr).RegisterVars
	_ func(*Engine, map[string]Extension) error   = (*Engine).RegisterExts
	_ func(*Engine, map[string]interface{}) error = (*Engine).RegisterVars
	_ jtypes.ArgHandler                           = Extension{}.UndefinedHandler
	_ jtypes.ArgHandler                           = Extension{}.EvalContextHandler
)

func TestExtensionCompatibilitySurface(t *testing.T) {
	expr := MustCompile(`[
        $repeat(3),
        $optional(),
        $optional(7),
        $variant("x"),
        $variant(2),
        $variadic(1, 2, 3),
		$kind("x"),
		$apply(function($value) {$value * 2}, 4),
		$legacy(5)
    ]`)
	err := expr.RegisterExts(map[string]Extension{
		"repeat": {
			Func: func(input string, count int) string { return strings.Repeat(input, count) },
			EvalContextHandler: func(args []reflect.Value) bool {
				return len(args) < 2
			},
		},
		"optional": {Func: func(input jtypes.OptionalInt) any {
			return map[string]any{"set": input.IsSet(), "value": input.Int}
		}},
		"variant": {Func: func(input testExtensionVariant) string {
			value := reflect.Value(input)
			return value.Type().String()
		}},
		"variadic": {Func: func(values ...int) int {
			total := 0
			for _, value := range values {
				total += value
			}
			return total
		}},
		"kind": {Func: func(input reflect.Value) string { return input.Kind().String() }},
		"apply": {Func: func(fn jtypes.Callable, input float64) (any, error) {
			result, err := fn.Call([]reflect.Value{reflect.ValueOf(input)})
			if err != nil || !result.IsValid() {
				return nil, err
			}
			return result.Interface(), nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := expr.RegisterVars(map[string]interface{}{"legacy": testLegacyCallable{}}); err != nil {
		t.Fatal(err)
	}

	got, err := expr.Eval("x")
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		"xxx",
		map[string]any{"set": false, "value": 0},
		map[string]any{"set": true, "value": 7},
		"string",
		"float64",
		6,
		"string",
		float64(8),
		float64(10),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Eval() = %#v, want %#v", got, want)
	}

	hof := MustCompile(`$map([1, 2], $withIndex)`)
	if err := hof.RegisterExts(map[string]Extension{"withIndex": {
		Func: func(value float64, index int) float64 { return value + float64(index) },
	}}); err != nil {
		t.Fatal(err)
	}
	if got, err := hof.Eval(nil); err != nil || !reflect.DeepEqual(got, []any{float64(1), float64(3)}) {
		t.Fatalf("extension HOF callback = %#v, %v", got, err)
	}
}

func TestExtensionHandlerSourceCompatibility(t *testing.T) {
	var bluesUndefined bluesjtypes.ArgHandler = func(args []reflect.Value) bool {
		return len(args) == 1 && !args[0].IsValid()
	}
	var localUndefined compatibilityArgHandler = compatibilityArgHandler(bluesUndefined)
	ordinaryUndefined := func(args []reflect.Value) bool {
		return len(args) == 1 && !args[0].IsValid()
	}

	for _, test := range []struct {
		name    string
		handler func([]reflect.Value) bool
	}{
		{name: "blues named type", handler: bluesUndefined},
		{name: "structurally equivalent named type", handler: localUndefined},
		{name: "ordinary function", handler: ordinaryUndefined},
	} {
		t.Run("undefined/"+test.name, func(t *testing.T) {
			expr := MustCompile(`$compatUndefined(missing)`)
			if err := expr.RegisterExts(map[string]Extension{"compatUndefined": {
				Func:             func(any) string { return "called" },
				UndefinedHandler: test.handler,
			}}); err != nil {
				t.Fatal(err)
			}
			if _, err := expr.Eval(nil); !errors.Is(err, ErrUndefined) {
				t.Fatalf("Eval() error = %v, want ErrUndefined", err)
			}
		})
	}

	var bluesContext bluesjtypes.ArgHandler = func(args []reflect.Value) bool {
		return len(args) < 2
	}
	var localContext compatibilityArgHandler = compatibilityArgHandler(bluesContext)
	ordinaryContext := func(args []reflect.Value) bool {
		return len(args) < 2
	}
	for _, test := range []struct {
		name    string
		handler func([]reflect.Value) bool
	}{
		{name: "blues named type", handler: bluesContext},
		{name: "structurally equivalent named type", handler: localContext},
		{name: "ordinary function", handler: ordinaryContext},
	} {
		t.Run("context/"+test.name, func(t *testing.T) {
			expr := MustCompile(`$compatContext(3)`)
			if err := expr.RegisterExts(map[string]Extension{"compatContext": {
				Func:               func(input string, count int) string { return strings.Repeat(input, count) },
				EvalContextHandler: test.handler,
			}}); err != nil {
				t.Fatal(err)
			}
			if got, err := expr.Eval("x"); err != nil || got != "xxx" {
				t.Fatalf("Eval() = %#v, %v; want xxx", got, err)
			}
		})
	}
}

func TestExtensionUndefinedErrorsAndPanicContainment(t *testing.T) {
	expr := MustCompile(`$undefined(missing)`)
	if err := expr.RegisterExts(map[string]Extension{
		"undefined": {
			Func: func(any) string { return "called" },
			UndefinedHandler: func(args []reflect.Value) bool {
				return len(args) == 1 && !args[0].IsValid()
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); !errors.Is(err, ErrUndefined) {
		t.Fatalf("Eval() error = %v, want ErrUndefined", err)
	}

	expr = MustCompile(`$returnsUndefined()`)
	if err := expr.RegisterExts(map[string]Extension{"returnsUndefined": {
		Func: func() (string, error) { return "", jtypes.ErrUndefined },
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); !errors.Is(err, ErrUndefined) {
		t.Fatalf("Eval() error = %v, want ErrUndefined", err)
	}

	expr = MustCompile(`$invalidValue()`)
	if err := expr.RegisterExts(map[string]Extension{"invalidValue": {
		Func: func() reflect.Value { return reflect.Value{} },
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); !errors.Is(err, ErrUndefined) {
		t.Fatalf("Eval() error = %v, want ErrUndefined", err)
	}

	expr = MustCompile(`$panic()`)
	if err := expr.RegisterExts(map[string]Extension{"panic": {
		Func: func() string { panic("boom") },
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); err == nil || !strings.Contains(err.Error(), `extension "panic" panicked: boom`) {
		t.Fatalf("Eval() error = %v, want contained panic", err)
	}

	expr = MustCompile(`$legacyPanic()`)
	if err := expr.RegisterVars(map[string]interface{}{"legacyPanic": testPanicCallable{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); err == nil || !strings.Contains(err.Error(), `extension callable "legacyPanic" panicked: legacy boom`) {
		t.Fatalf("Eval() error = %v, want contained callable panic", err)
	}
}

func TestExtensionValidationAndTypedErrors(t *testing.T) {
	tests := []struct {
		name string
		ext  Extension
		want string
	}{
		{"notFunc", Extension{Func: 1}, "func must be a Go function"},
		{"noResults", Extension{Func: func() {}}, "func must return either 1 or 2 values"},
		{"badError", Extension{Func: func() (int, int) { return 0, 0 }}, "func must return an error as its second value"},
		{"optionalOrder", Extension{Func: func(jtypes.OptionalInt, int) int { return 0 }}, "a non-optional parameter cannot follow an optional parameter"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := MustCompile(`1`).RegisterExts(map[string]Extension{"test": test.ext})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("RegisterExts() error = %v, want %q", err, test.want)
			}
		})
	}
	if err := MustCompile(`1`).RegisterVars(map[string]interface{}{"bad-name": 1}); err == nil || err.Error() != "bad-name is not a valid name" {
		t.Fatalf("RegisterVars() error = %v", err)
	}

	expr := MustCompile(`$arity(1)`)
	if err := expr.RegisterExts(map[string]Extension{"arity": {Func: func(int, int) int { return 0 }}}); err != nil {
		t.Fatal(err)
	}
	_, err := expr.Eval(nil)
	var countError *ArgCountError
	if !errors.As(err, &countError) || *countError != (ArgCountError{Func: "arity", Expected: 2, Received: 1}) {
		t.Fatalf("Eval() error = %#v, want ArgCountError", err)
	}

	expr = MustCompile(`$typed("x")`)
	if err := expr.RegisterExts(map[string]Extension{"typed": {Func: func(int) int { return 0 }}}); err != nil {
		t.Fatal(err)
	}
	_, err = expr.Eval(nil)
	var typeError *ArgTypeError
	if !errors.As(err, &typeError) || *typeError != (ArgTypeError{Func: "typed", Which: 1}) {
		t.Fatalf("Eval() error = %#v, want ArgTypeError", err)
	}
}

func TestRegistrationSnapshotsAndPrecedence(t *testing.T) {
	preservePackageRegistry(t)
	const name = "phase6SnapshotValue"
	const globalOnly = "phase6GlobalOnly"
	if err := RegisterVars(map[string]interface{}{name: 1}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterVars(map[string]interface{}{globalOnly: 9}); err != nil {
		t.Fatal(err)
	}
	old := MustCompile(`$` + name)
	if err := RegisterVars(map[string]interface{}{name: 2}); err != nil {
		t.Fatal(err)
	}
	newer := MustCompile(`$` + name)
	assertEvalValue(t, old, 1)
	assertEvalValue(t, newer, 2)
	if err := old.RegisterVars(map[string]interface{}{name: 3}); err != nil {
		t.Fatal(err)
	}
	assertEvalValue(t, old, 3)
	got, err := old.EvalWithOptions(nil, EvalOptions{Bindings: map[string]any{name: 4}})
	if err != nil || got != 4 {
		t.Fatalf("per-call override = %#v, %v", got, err)
	}

	engine := NewEngine()
	isolated, err := engine.Compile(`$` + globalOnly)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := isolated.Eval(nil); !errors.Is(err, ErrUndefined) {
		t.Fatalf("isolated Engine inherited package registration: %v", err)
	}
	if err := engine.RegisterVars(map[string]interface{}{name: 5}); err != nil {
		t.Fatal(err)
	}
	engineOld, err := engine.Compile(`$` + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.RegisterVars(map[string]interface{}{name: 6}); err != nil {
		t.Fatal(err)
	}
	engineNew, err := engine.Compile(`$` + name)
	if err != nil {
		t.Fatal(err)
	}
	assertEvalValue(t, engineOld, 5)
	assertEvalValue(t, engineNew, 6)
}

func TestPackageRegistrationAndCompileAreConcurrent(t *testing.T) {
	preservePackageRegistry(t)
	const name = "phase6PackageConcurrent"
	if err := RegisterVars(map[string]interface{}{name: 0}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for iteration := 0; iteration < 50; iteration++ {
				if worker%2 == 0 {
					if err := RegisterVars(map[string]interface{}{name: iteration}); err != nil {
						t.Errorf("RegisterVars() error = %v", err)
						return
					}
					continue
				}
				expression, err := Compile(`$` + name)
				if err != nil {
					t.Errorf("Compile() error = %v", err)
					return
				}
				if _, err := expression.Eval(nil); err != nil {
					t.Errorf("Eval() error = %v", err)
					return
				}
			}
		}(worker)
	}
	group.Wait()
}

func TestContextExtensionCancellationAndConcurrentRegistration(t *testing.T) {
	expr := MustCompile(`$phase6Context()`)
	if err := expr.RegisterExts(map[string]Extension{"phase6Context": {
		Func: func(ctx context.Context) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
				value, _ := ctx.Value(testContextKey{}).(string)
				return value, nil
			}
		},
	}}); err != nil {
		t.Fatal(err)
	}
	active := context.WithValue(context.Background(), testContextKey{}, "injected")
	if got, err := expr.EvalContext(active, nil); err != nil || got != "injected" {
		t.Fatalf("EvalContext() = %#v, %v; want injected context", got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := expr.EvalContext(canceled, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("EvalContext() error = %v, want context.Canceled", err)
	}

	concurrent := MustCompile(`$phase6Concurrent`)
	if err := concurrent.RegisterVars(map[string]interface{}{"phase6Concurrent": 0}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				if worker%2 == 0 {
					if err := concurrent.RegisterVars(map[string]interface{}{"phase6Concurrent": iteration}); err != nil {
						t.Errorf("RegisterVars() error = %v", err)
						return
					}
					continue
				}
				if _, err := concurrent.Eval(nil); err != nil {
					t.Errorf("Eval() error = %v", err)
					return
				}
			}
		}(worker)
	}
	group.Wait()

	engine := NewEngine()
	if err := engine.RegisterVars(map[string]interface{}{"phase6EngineConcurrent": 0}); err != nil {
		t.Fatal(err)
	}
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for iteration := 0; iteration < 50; iteration++ {
				if worker%2 == 0 {
					if err := engine.RegisterVars(map[string]interface{}{"phase6EngineConcurrent": iteration}); err != nil {
						t.Errorf("Engine.RegisterVars() error = %v", err)
						return
					}
					continue
				}
				if _, err := engine.Eval(context.Background(), `$phase6EngineConcurrent`, nil); err != nil {
					t.Errorf("Engine.Eval() error = %v", err)
					return
				}
			}
		}(worker)
	}
	group.Wait()
}

func TestContextExtensionObservesEvaluationTimeout(t *testing.T) {
	expr := MustCompile(`$phase6WaitForTimeout()`)
	if err := expr.RegisterExts(map[string]Extension{"phase6WaitForTimeout": {
		Func: func(ctx context.Context) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err := expr.EvalWithOptions(nil, EvalOptions{Timeout: 20 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("EvalWithOptions() error = %v, want deadline exceeded", err)
	}
	var jsonataError *Error
	if !errors.As(err, &jsonataError) || jsonataError.Code != "D1012" {
		t.Fatalf("EvalWithOptions() error = %#v, want D1012", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("context-aware timeout returned after %s", elapsed)
	}
}

func TestRegisteredAndPerCallCallableBindingsMatch(t *testing.T) {
	registered := MustCompile(`[$fn(3), $nested.fn(4)]`)
	bindings := map[string]any{
		"fn":     testLegacyCallable{},
		"nested": map[string]any{"fn": testLegacyCallable{}},
	}
	if err := registered.RegisterVars(bindings); err != nil {
		t.Fatal(err)
	}
	want := []any{float64(6), float64(8)}
	if got, err := registered.Eval(nil); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("registered callable bindings = %#v, %v; want %#v", got, err, want)
	}

	perCall := MustCompile(`[$fn(3), $nested.fn(4)]`)
	if got, err := perCall.EvalWithOptions(nil, EvalOptions{Bindings: bindings}); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("per-call callable bindings = %#v, %v; want %#v", got, err, want)
	}
}

func TestLegacyExtensionTimeoutDoesNotDetach(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	expr := MustCompile(`$phase6Blocking()`)
	if err := expr.RegisterExts(map[string]Extension{"phase6Blocking": {
		Func: func() string {
			close(started)
			<-release
			return "finished"
		},
	}}); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := expr.EvalWithOptions(nil, EvalOptions{Timeout: time.Millisecond})
		result <- err
	}()
	<-started
	select {
	case err := <-result:
		t.Fatalf("legacy extension detached or returned before release: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("EvalWithOptions() error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("legacy extension did not return after release")
	}
}

type testContextKey struct{}

func assertEvalValue(t *testing.T, expr *Expr, want any) {
	t.Helper()
	got, err := expr.Eval(nil)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Eval() = %#v, %v; want %v", got, err, want)
	}
}

func preservePackageRegistry(t *testing.T) {
	t.Helper()
	packageRegistry.RLock()
	original := packageRegistry.values
	packageRegistry.RUnlock()
	t.Cleanup(func() {
		packageRegistry.Lock()
		packageRegistry.values = original
		packageRegistry.Unlock()
	})
}
