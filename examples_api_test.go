package jsonata_test

import (
	"context"
	"errors"
	"fmt"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

func ExampleCompile() {
	expression, err := jsonata.Compile(`Account.Name`)
	if err != nil {
		panic(err)
	}
	value, err := expression.Eval(map[string]any{
		"Account": map[string]any{"Name": "Ada"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(value)
	// Output: Ada
}

func ExampleEvalBytes() {
	result, err := jsonata.EvalBytes(
		`$sum(items.price)`,
		[]byte(`{"items":[{"price":12},{"price":30}]}`),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(result))
	// Output: 42
}

func ExampleEngine() {
	first := jsonata.NewEngine()
	second := jsonata.NewEngine()
	if err := first.RegisterVars(map[string]interface{}{"tenant": "one"}); err != nil {
		panic(err)
	}

	firstValue, err := first.Eval(context.Background(), `$tenant`, nil)
	if err != nil {
		panic(err)
	}
	secondValue, secondErr := second.Eval(context.Background(), `$tenant`, nil)
	fmt.Println(firstValue)
	fmt.Println(secondValue == nil, errors.Is(secondErr, jsonata.ErrUndefined))
	// Output:
	// one
	// true true
}

func ExampleError() {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := jsonata.MustCompile(`1 + 1`).EvalContext(canceled, nil)

	var structured *jsonata.Error
	if errors.As(err, &structured) {
		fmt.Println(structured.Code, errors.Is(err, context.Canceled))
	}
	// Output: U1001 true
}

func ExampleErrUndefined() {
	_, err := jsonata.MustCompile(`missing`).Eval(nil)
	fmt.Println(errors.Is(err, jsonata.ErrUndefined))
	// Output: true
}
