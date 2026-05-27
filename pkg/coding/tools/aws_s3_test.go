package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestAWSS3Tool_NoCredentials(t *testing.T) {
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	tool := tools.AWSS3Tool()
	params := map[string]any{"action": "list_buckets"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestAWSS3Tool_ListBuckets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0"?><ListAllMyBucketsResult><Buckets><Bucket><Name>test-bucket</Name></Bucket></Buckets></ListAllMyBucketsResult>`))
	}))
	defer srv.Close()

	os.Setenv("AWS_ACCESS_KEY_ID", "AKID123")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "secret123")
	os.Setenv("AWS_REGION", "us-east-1")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	defer os.Unsetenv("AWS_REGION")

	tool := tools.AWSS3Tool()
	params := map[string]any{"action": "list_buckets"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestAWSS3Tool_UnknownAction(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "AKID123")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "secret123")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	tool := tools.AWSS3Tool()
	params := map[string]any{"action": "bogus"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}