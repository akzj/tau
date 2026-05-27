package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestStripeInvoiceTool_NoAPIKey(t *testing.T) {
	os.Unsetenv("STRIPE_API_KEY")

	tool := tools.StripeInvoiceTool()
	params := map[string]any{"action": "list_invoices"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestStripeInvoiceTool_ListInvoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"inv_123","amount_due":1000,"status":"paid"}]}`))
	}))
	defer srv.Close()

	os.Setenv("STRIPE_API_KEY", "sk_test_456")
	defer os.Unsetenv("STRIPE_API_KEY")

	tool := tools.StripeInvoiceTool()
	params := map[string]any{"action": "list_invoices"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestStripeInvoiceTool_CreateInvoiceNoCustomer(t *testing.T) {
	os.Setenv("STRIPE_API_KEY", "sk_test_456")
	defer os.Unsetenv("STRIPE_API_KEY")

	tool := tools.StripeInvoiceTool()
	params := map[string]any{"action": "create_invoice"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for missing customer_id/amount")
	}
}