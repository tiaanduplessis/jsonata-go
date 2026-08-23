package syntax

import "testing"

func TestParenthesizedArrayPathStepRetainsFlatteningProvenance(t *testing.T) {
	parenthesized, err := Parse(`[0..2].([0..2])`)
	if err != nil {
		t.Fatal(err)
	}
	parenthesizedPath, ok := parenthesized.(Binary)
	if !ok {
		t.Fatalf("parenthesized path = %T, want Binary", parenthesized)
	}
	parenthesizedArray, ok := parenthesizedPath.Right.(Array)
	if !ok || !parenthesizedArray.FlattenInPath {
		t.Fatalf("parenthesized right step = %#v, want flattening Array", parenthesizedPath.Right)
	}

	direct, err := Parse(`[0..2].[0..2]`)
	if err != nil {
		t.Fatal(err)
	}
	directPath, ok := direct.(Binary)
	if !ok {
		t.Fatalf("direct path = %T, want Binary", direct)
	}
	directArray, ok := directPath.Right.(Array)
	if !ok || directArray.FlattenInPath {
		t.Fatalf("direct right step = %#v, want constructor Array", directPath.Right)
	}
}
