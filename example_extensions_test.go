package jsonata_test

import (
	"fmt"
	"strings"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

func ExampleExpr_RegisterExts() {
	expression := jsonata.MustCompile(`$uppercase("beneath the underdog")`)
	err := expression.RegisterExts(map[string]jsonata.Extension{
		"uppercase": {Func: strings.ToUpper},
	})
	if err != nil {
		panic(err)
	}
	result, err := expression.Eval(nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
	// Output: BENEATH THE UNDERDOG
}
