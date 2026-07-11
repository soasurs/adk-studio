package main

import (
	"context"
	"errors"
	"testing"

	"github.com/soasurs/adk/tool"
)

func TestToolLabHandledFailure(t *testing.T) {
	tools, err := toolLabTools()
	if err != nil {
		t.Fatal(err)
	}

	result, err := tools[1].Run(t.Context(), tool.Call{
		ID:        "call-1",
		Name:      "inspect_order",
		Arguments: []byte(`{"customer_id":"cus-1","order_id":"unknown","diagnostic_token":"token"}`),
	})
	if err != nil {
		t.Fatalf("handled failure returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("result IsError = false, want true: %#v", result)
	}
}

func TestToolLabCancellationIsTerminal(t *testing.T) {
	tools, err := toolLabTools()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := tools[0].Run(ctx, tool.Call{
		ID:        "call-1",
		Name:      "lookup_customer",
		Arguments: []byte(`{"query":"Alex"}`),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.Content != "" || len(result.StructuredContent) != 0 || result.IsError {
		t.Fatalf("result = %#v, want zero result", result)
	}
}
