package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRAGTokenizer(t *testing.T) {
	tokens := tokenize("parseHTTPRequest camelCaseFunc getUser_byID123")
	if len(tokens) == 0 {
		t.Fatal("expected non-empty tokens")
	}

	hasParse := false
	for _, tok := range tokens {
		if tok == "parse" {
			hasParse = true
			break
		}
	}
	if !hasParse {
		t.Errorf("expected 'parse' token, got %v", tokens)
	}
}

func TestRAGTFIDF(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("func hello world hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("func world goodbye"), 0644)

	idx := NewRAGIndex()
	err := idx.IndexWorkspace(dir, []string{".go"})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.documents) != 2 {
		t.Errorf("expected 2 docs, got %d", len(idx.documents))
	}

	// "hello" should have higher IDF (appears in only 1 doc) than "world" (appears in 2 docs)
	if idx.idf["hello"] <= idx.idf["world"] {
		t.Errorf("expected hello (1 doc) IDF > world (2 docs) IDF: %f vs %f",
			idx.idf["hello"], idx.idf["world"])
	}
}

func TestRAGCosineSimilarity(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.go"), []byte("package main; func main() { fmt.Println(\"hello\") }"), 0644)
	os.WriteFile(filepath.Join(dir, "y.go"), []byte("package test; func TestX() { t.Run(\"test\") }"), 0644)

	idx := NewRAGIndex()
	err := idx.IndexWorkspace(dir, []string{".go"})
	if err != nil {
		t.Fatal(err)
	}

	results := idx.Search("hello world", 5)
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Path != "x.go" {
		t.Errorf("expected x.go top, got %s", results[0].Path)
	}
}

func TestRAGSearchRanking(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "relevant.go"), []byte(
		"// This file handles user authentication and session management\nfunc login() {}\nfunc logout() {}"), 0644)
	os.WriteFile(filepath.Join(dir, "unrelated.go"), []byte(
		"// Math utilities\nfunc add(a, b int) int { return a + b }\nfunc multiply(a, b int) int { return a * b }"), 0644)

	idx := NewRAGIndex()
	err := idx.IndexWorkspace(dir, []string{".go"})
	if err != nil {
		t.Fatal(err)
	}

	results := idx.Search("user login auth session", 5)
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].Path != "relevant.go" {
		t.Errorf("expected relevant.go first, got %s (score: %f)", results[0].Path, results[0].Score)
	}
}

func TestRAGEmptyQuery(t *testing.T) {
	idx := NewRAGIndex()
	results := idx.Search("", 5)
	if len(results) != 0 {
		t.Error("expected no results for empty query")
	}
}

func TestRAGEmptyIndex(t *testing.T) {
	dir := t.TempDir()
	idx := NewRAGIndex()
	err := idx.IndexWorkspace(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := idx.Search("anything", 5)
	if len(results) != 0 {
		t.Error("expected no results for empty index")
	}
}