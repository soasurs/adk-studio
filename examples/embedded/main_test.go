package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
