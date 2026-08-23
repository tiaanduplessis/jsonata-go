package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	blues "github.com/blues/jsonata-go"
	gnata "github.com/recolabs/gnata"
	jsonata "github.com/tiaanduplessis/jsonata-go"
)

const (
	WorkspaceImplementation = "jsonata-go"
	BluesImplementation     = "blues"
	GnataImplementation     = "gnata"
)

// Implementations returns the pinned comparison adapters in stable report
// order.
func Implementations() []Runtime {
	return []Runtime{workspaceRuntime(), bluesRuntime(), gnataRuntime()}
}

func workspaceRuntime() Runtime {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{
		"double": {Func: func(value float64) float64 { return value * 2 }},
	}); err != nil {
		panic(fmt.Sprintf("configure workspace benchmark runtime: %v", err))
	}
	return Runtime{
		ID: WorkspaceImplementation,
		CompileCase: func(c Case) (Compiled, error) {
			expr, err := engine.Compile(c.Expression)
			if err != nil {
				return nil, err
			}
			return workspaceExpression{expr: expr}, nil
		},
	}
}

type workspaceExpression struct{ expr *jsonata.Expr }

func (e workspaceExpression) Eval(input any) (any, error) { return e.expr.Eval(input) }
func (e workspaceExpression) EvalBytes(input []byte) (any, error) {
	return e.expr.EvalBytes(input)
}

func bluesRuntime() Runtime {
	if err := blues.RegisterExts(map[string]blues.Extension{
		"double": {Func: func(value float64) float64 { return value * 2 }},
	}); err != nil {
		panic(fmt.Sprintf("configure blues benchmark runtime: %v", err))
	}
	return Runtime{
		ID: BluesImplementation,
		CompileCase: func(c Case) (Compiled, error) {
			expr, err := blues.Compile(c.Expression)
			if err != nil {
				return nil, err
			}
			return bluesExpression{expr: expr}, nil
		},
		Unsupported: func(_ Case, mode Mode) string {
			if mode == ModeParallel {
				return "v1.5.4 does not document compiled expressions as safe for concurrent evaluation"
			}
			return ""
		},
	}
}

type bluesExpression struct{ expr *blues.Expr }

func (e bluesExpression) Eval(input any) (any, error) { return e.expr.Eval(input) }
func (e bluesExpression) EvalBytes(input []byte) (any, error) {
	return e.expr.EvalBytes(input)
}

func gnataRuntime() Runtime {
	customEnvironment := gnata.NewCustomEnv(map[string]gnata.CustomFunc{
		"double": func(args []any, _ any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("double expects one argument")
			}
			value, err := numericValue(args[0])
			if err != nil {
				return nil, err
			}
			return value * 2, nil
		},
	})
	return Runtime{
		ID: GnataImplementation,
		CompileCase: func(c Case) (Compiled, error) {
			expr, err := gnata.Compile(c.Expression)
			if err != nil {
				return nil, err
			}
			compiled := gnataExpression{expr: expr}
			if c.CustomFunction != "" {
				compiled.eval = func(input any) (any, error) {
					value, evalErr := expr.EvalWithCustomFuncs(context.Background(), input, customEnvironment)
					return gnata.NormalizeValue(value), evalErr
				}
			}
			return compiled, nil
		},
		Unsupported: func(c Case, mode Mode) string {
			if c.CustomFunction != "" && mode == ModeBytes {
				return "v0.2.3 has no raw-input evaluation API that accepts a custom-function environment"
			}
			return ""
		},
	}
}

type gnataExpression struct {
	expr *gnata.Expression
	eval func(any) (any, error)
}

func (e gnataExpression) Eval(input any) (any, error) {
	if e.eval != nil {
		return e.eval(input)
	}
	value, err := e.expr.Eval(context.Background(), input)
	return gnata.NormalizeValue(value), err
}

func (e gnataExpression) EvalBytes(input []byte) (any, error) {
	value, err := e.expr.EvalBytes(context.Background(), json.RawMessage(input))
	return gnata.NormalizeValue(value), err
}
func numericValue(value any) (float64, error) {
	switch number := value.(type) {
	case float64:
		return number, nil
	case float32:
		return float64(number), nil
	case int:
		return float64(number), nil
	case int64:
		return float64(number), nil
	case json.Number:
		return number.Float64()
	case string:
		return strconv.ParseFloat(number, 64)
	default:
		return 0, fmt.Errorf("double expects a number, got %T", value)
	}
}
