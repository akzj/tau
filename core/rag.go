package core

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// RAGIndex builds a TF-IDF index over workspace files for semantic code search.
type RAGIndex struct {
	documents map[string]string             // filePath → content
	tf        map[string]map[string]float64 // term → doc → TF
	idf       map[string]float64            // term → IDF
	docNorms  map[string]float64            // doc → vector norm
}

// NewRAGIndex creates an empty index.
func NewRAGIndex() *RAGIndex {
	return &RAGIndex{
		documents: make(map[string]string),
		tf:        make(map[string]map[string]float64),
		idf:       make(map[string]float64),
		docNorms:  make(map[string]float64),
	}
}

// SearchResult holds a single search result.
type SearchResult struct {
	Path    string  `json:"path"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// IndexWorkspace scans workspace files, tokenizes them, and builds TF-IDF vectors.
// extensions: file extensions to include (e.g., [".go", ".md"]). nil = all text files.
func (idx *RAGIndex) IndexWorkspace(root string, extensions []string) error {
	idx.documents = make(map[string]string)
	idx.tf = make(map[string]map[string]float64)
	idx.idf = make(map[string]float64)
	idx.docNorms = make(map[string]float64)

	df := make(map[string]int) // document frequency

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		if len(extensions) > 0 {
			ext := filepath.Ext(d.Name())
			found := false
			for _, e := range extensions {
				if ext == e {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		content, err := readFileContent(path)
		if err != nil {
			return nil
		}
		if len(content) == 0 {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		idx.documents[rel] = content

		tokens := tokenize(content)
		seen := make(map[string]bool)
		idx.tf[rel] = make(map[string]float64)
		for _, t := range tokens {
			idx.tf[rel][t]++
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Compute IDF: idf(t) = log((N+1)/(df+1)) + 1 (smooth)
	n := float64(len(idx.documents))
	for term, d := range df {
		idx.idf[term] = math.Log((n+1)/(float64(d)+1)) + 1
	}

	// Compute doc norms for cosine similarity
	for doc, terms := range idx.tf {
		sumSq := 0.0
		for term, tf := range terms {
			w := tf * idx.idf[term]
			sumSq += w * w
		}
		idx.docNorms[doc] = math.Sqrt(sumSq)
	}
	return nil
}

// Search returns top-k documents matching the query using cosine similarity.
func (idx *RAGIndex) Search(query string, topK int) []SearchResult {
	if topK <= 0 {
		topK = 10
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	// Build query vector (raw term frequency)
	queryVec := make(map[string]float64)
	for _, t := range queryTokens {
		queryVec[t]++
	}

	// Normalize query
	queryNorm := 0.0
	for _, v := range queryVec {
		queryNorm += v * v
	}
	queryNorm = math.Sqrt(queryNorm)
	if queryNorm == 0 {
		return nil
	}

	// Score each document
	type scored struct {
		path    string
		score   float64
		snippet string
	}
	var results []scored

	for doc := range idx.documents {
		dot := 0.0
		for term, qv := range queryVec {
			if tv, ok := idx.tf[doc][term]; ok {
				dot += (qv / queryNorm) * (tv * idx.idf[term])
			}
		}
		if idx.docNorms[doc] > 0 {
			dot /= idx.docNorms[doc]
		}
		if dot > 0 {
			snippet := generateSnippet(idx.documents[doc], queryTokens[0], 80)
			results = append(results, scored{doc, dot, snippet})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	var out []SearchResult
	for i := 0; i < topK; i++ {
		out = append(out, SearchResult{
			Path:    results[i].path,
			Score:   results[i].score,
			Snippet: results[i].snippet,
		})
	}
	return out
}

// --- Tokenizer ---

var wordSplitter = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
var camelSplitter = regexp.MustCompile(`[a-z][A-Z]|[A-Z][A-Z][a-z]|[0-9][a-zA-Z]|[a-zA-Z][0-9]`)

// stopWords contains 150 common English + code stop words.
var stopWords = map[string]bool{
	"the": true, "is": true, "at": true, "which": true, "on": true, "a": true, "an": true, "and": true, "or": true, "not": true,
	"but": true, "in": true, "to": true, "for": true, "of": true, "with": true, "from": true, "by": true, "as": true, "be": true,
	"are": true, "was": true, "were": true, "been": true, "being": true, "have": true, "has": true, "had": true, "do": true,
	"does": true, "did": true, "will": true, "would": true, "could": true, "should": true, "may": true, "might": true, "can": true,
	"shall": true, "it": true, "its": true, "he": true, "she": true, "they": true, "we": true, "you": true, "i": true, "me": true,
	"my": true, "your": true, "his": true, "her": true, "our": true, "their": true, "this": true, "that": true, "these": true,
	"those": true, "all": true, "each": true, "every": true, "both": true, "few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true, "no": true, "nor": true, "only": true, "own": true, "same": true, "so": true, "than": true, "too": true,
	"very": true, "just": true, "about": true, "above": true, "after": true, "again": true, "against": true, "between": true,
	"into": true, "through": true, "during": true, "before": true, "under": true, "while": true, "then": true, "also": true,
	"if": true, "else": true, "when": true, "where": true, "how": true, "what": true, "who": true, "why": true,
	"func": true, "type": true, "var": true, "const": true, "package": true, "import": true, "return": true, "nil": true,
	"true": true, "false": true, "string": true, "int": true, "error": true, "bool": true, "byte": true, "make": true, "new": true,
	"len": true, "cap": true, "append": true, "copy": true, "close": true, "delete": true, "panic": true, "recover": true,
	"defer": true, "go": true, "select": true, "case": true, "default": true, "switch": true, "range": true, "break": true,
	"continue": true, "fallthrough": true, "goto": true, "interface": true, "struct": true, "map": true, "chan": true,
}

func tokenize(text string) []string {
	// CamelCase split on original casing: "camelCase" → "camel case"
	parts := camelSplitter.ReplaceAllStringFunc(text, func(m string) string {
		return m[:1] + " " + m[1:]
	})
	// Split on non-alphanumeric
	words := wordSplitter.Split(parts, -1)

	var tokens []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		w = strings.ToLower(w)
		if len(w) < 2 || len(w) > 30 {
			continue
		}
		if stopWords[w] {
			continue
		}
		tokens = append(tokens, w)
	}
	return tokens
}

func readFileContent(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 1024*1024 { // skip >1MB
		return "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, " "), nil
}

func generateSnippet(content, query string, maxLen int) string {
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)
	i := strings.Index(contentLower, queryLower)
	if i < 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}
	start := i - 20
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet += "..."
	}
	return snippet
}

// ensure unicode is used (imported for future use)

// --- VectorRAGPipeline ---

// VectorRAGPipeline combines embedding + vector store for semantic document retrieval.
type VectorRAGPipeline struct {
	embedder Embedder
	store    VectorStore
	topK     int
}

// NewVectorRAGPipeline creates a vector-based RAG pipeline.
// topK controls the number of documents retrieved per query.
func NewVectorRAGPipeline(embedder Embedder, store VectorStore, topK int) *VectorRAGPipeline {
	if topK <= 0 {
		topK = 5
	}
	return &VectorRAGPipeline{embedder: embedder, store: store, topK: topK}
}

// Index indexes a set of document chunks by computing embeddings and inserting
// them into the vector store. Returns the number of successfully indexed chunks.
func (p *VectorRAGPipeline) Index(ctx context.Context, chunks []DocumentChunk) (int, error) {
	count := 0
	for _, chunk := range chunks {
		emb, err := p.embedder.Embed(ctx, chunk.Content)
		if err != nil {
			continue
		}
		err = p.store.Insert(VectorEntry{
			ID:        chunk.ID,
			Text:      truncateStr(chunk.Content, 200),
			Chunk:     chunk.Path,
			Embedding: emb,
		})
		if err == nil {
			count++
		}
	}
	return count, nil
}

// Search finds the top-K most relevant documents for a query.
func (p *VectorRAGPipeline) Search(ctx context.Context, query string) ([]VectorEntry, error) {
	emb, err := p.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return p.store.Search(emb, p.topK)
}

// GenerateContext builds a Markdown context string from search results
// suitable for injection into an agent's system prompt.
func (p *VectorRAGPipeline) GenerateContext(results []VectorEntry) string {
	if len(results) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Retrieved Documents\n\n")
	for i, r := range results {
		fmt.Fprintf(&b, "### %d. %s\n", i+1, r.Chunk)
		b.WriteString(r.Text)
		b.WriteString("\n\n")
	}
	return b.String()
}

// Stats returns RAG pipeline statistics.
func (p *VectorRAGPipeline) Stats() map[string]int {
	return map[string]int{
		"documents": p.store.Count(),
		"top_k":     p.topK,
	}
}

// Embedder returns the pipeline's embedder (for external diagnostic use).
func (p *VectorRAGPipeline) Embedder() Embedder {
	return p.embedder
}
var _ = unicode.ToLower