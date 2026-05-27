package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DocumentChunk represents a chunk of a loaded document.
type DocumentChunk struct {
	ID        string
	Path      string
	Content   string
	Language  string
}

// DocumentLoader loads and chunks documents from a directory tree.
type DocumentLoader struct {
	ChunkSize  int
	MaxChunks  int
	Extensions []string
}

// NewDocumentLoader creates a document loader with sensible defaults.
func NewDocumentLoader() *DocumentLoader {
	return &DocumentLoader{
		ChunkSize:  1000,
		MaxChunks:  100000,
		Extensions: []string{".txt", ".md", ".go", ".py", ".js", ".ts", ".yaml", ".json", ".html", ".css"},
	}
}

// Load scans a directory recursively and returns chunked documents.
func (dl *DocumentLoader) Load(root string) ([]DocumentChunk, error) {
	var chunks []DocumentChunk
	idCounter := 0

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		allowed := false
		for _, e := range dl.Extensions {
			if strings.EqualFold(ext, e) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil
		}
		if info.Size() > 1024*1024 { // skip >1MB
			return nil
		}

		fileChunks, err := dl.chunkFile(path, &idCounter)
		if err != nil {
			return nil // skip unreadable files
		}
		chunks = append(chunks, fileChunks...)
		if len(chunks) >= dl.MaxChunks {
			return filepath.SkipAll
		}
		return nil
	})
	return chunks, err
}

// chunkFile reads a file and splits it into fixed-size chunks.
func (dl *DocumentLoader) chunkFile(path string, counter *int) ([]DocumentChunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if len(content) == 0 {
		return nil, nil
	}

	// Use rune count for accurate character-based chunking
	runes := []rune(content)
	if len(runes) == 0 {
		return nil, nil
	}

	ext := filepath.Ext(path)
	lang := strings.TrimPrefix(ext, ".")

	var chunks []DocumentChunk
	for i := 0; i < len(runes); i += dl.ChunkSize {
		end := i + dl.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunkContent := string(runes[i:end])
		if len(strings.TrimSpace(chunkContent)) == 0 {
			continue
		}

		*counter++
		id := fmt.Sprintf("%s#%d", path, *counter)

		chunks = append(chunks, DocumentChunk{
			ID:       id,
			Path:     path,
			Content:  chunkContent,
			Language: lang,
		})
	}
	return chunks, nil
}
