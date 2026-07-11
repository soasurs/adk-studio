package demo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soasurs/adk/tool"
)

func TestReadFileReadsWithinRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello from read_file"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readFile(t.Context(), root, readFileInput{Path: "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "README.md" {
		t.Fatalf("path = %q, want README.md", got.Path)
	}
	if got.Content != "hello from read_file" {
		t.Fatalf("content = %q", got.Content)
	}
	if got.Truncated {
		t.Fatal("truncated = true, want false")
	}
}

func TestReadFileRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()

	if _, err := readFile(t.Context(), root, readFileInput{Path: "../secret.txt"}); err == nil {
		t.Fatal("expected error for path outside root")
	}
}

func TestReadFileToolReturnsHandledFailures(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	readTool, err := NewReadFileTool(root)
	if err != nil {
		t.Fatal(err)
	}

	for name, arguments := range map[string]string{
		"empty":      `{"path":""}`,
		"absolute":   `{"path":"/tmp/secret"}`,
		"outside":    `{"path":"../secret"}`,
		"directory":  `{"path":"dir"}`,
		"not_exists": `{"path":"missing.txt"}`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := readTool.Run(t.Context(), tool.Call{ID: "call-1", Name: "read_file", Arguments: []byte(arguments)})
			if err != nil {
				t.Fatalf("handled failure returned error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("result IsError = false, want true: %#v", result)
			}
		})
	}
}

func TestReadFileToolCancellationIsTerminal(t *testing.T) {
	readTool, err := NewReadFileTool(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := readTool.Run(ctx, tool.Call{ID: "call-1", Name: "read_file", Arguments: []byte(`{"path":"README.md"}`)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.Content != "" || len(result.StructuredContent) != 0 || result.IsError {
		t.Fatalf("result = %#v, want zero result", result)
	}
}

func TestReadFileToolRootInitializationFailureIsTerminal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	readTool, err := NewReadFileTool(root)
	if err != nil {
		t.Fatal(err)
	}

	result, err := readTool.Run(t.Context(), tool.Call{ID: "call-1", Name: "read_file", Arguments: []byte(`{"path":"README.md"}`)})
	if err == nil {
		t.Fatal("error = nil, want terminal root initialization error")
	}
	if result.Content != "" || len(result.StructuredContent) != 0 || result.IsError {
		t.Fatalf("result = %#v, want zero result", result)
	}
}

func TestReadFileTruncatesContent(t *testing.T) {
	root := t.TempDir()
	content := strings.Repeat("a", 32)
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readFile(t.Context(), root, readFileInput{Path: "large.txt", MaxBytes: 12})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated {
		t.Fatal("truncated = false, want true")
	}
	if got.ReturnedBytes != 12 {
		t.Fatalf("returned bytes = %d, want 12", got.ReturnedBytes)
	}
	if got.Content != strings.Repeat("a", 12) {
		t.Fatalf("content = %q", got.Content)
	}
}
