package demo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soasurs/adk/tool"
)

const (
	defaultReadFileMaxBytes = 16 * 1024
	hardReadFileMaxBytes    = 64 * 1024
)

type readFileInput struct {
	Path     string `json:"path" jsonschema:"Workspace-relative file path to read. Absolute paths and paths outside the workspace are rejected."`
	MaxBytes int64  `json:"max_bytes,omitempty" jsonschema:"Optional maximum number of bytes to return. Defaults to 16384 and is capped at 65536."`
}

type readFileOutput struct {
	Path          string `json:"path"`
	TotalBytes    int64  `json:"total_bytes"`
	ReturnedBytes int64  `json:"returned_bytes"`
	Truncated     bool   `json:"truncated"`
	Content       string `json:"content"`
}

func NewReadFileTool(workspaceRoot string) (tool.Tool, error) {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve read_file root: %w", err)
	}

	return tool.NewFunc(tool.Definition{
		Name:        "read_file",
		Description: "Read a UTF-8 text file from the example process working directory. Absolute paths and paths outside that root are rejected.",
	}, func(ctx context.Context, input readFileInput) (readFileOutput, error) {
		return readFile(ctx, absRoot, input)
	})
}

func readFile(ctx context.Context, workspaceRoot string, input readFileInput) (readFileOutput, error) {
	select {
	case <-ctx.Done():
		return readFileOutput{}, ctx.Err()
	default:
	}

	requestedPath := strings.TrimSpace(input.Path)
	if requestedPath == "" {
		return readFileOutput{}, tool.NewFuncError("path is required")
	}
	if filepath.IsAbs(requestedPath) {
		return readFileOutput{}, tool.NewFuncError("absolute paths are not allowed")
	}

	cleanPath := filepath.Clean(requestedPath)
	if cleanPath == "." {
		return readFileOutput{}, tool.NewFuncError("path must point to a file")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return readFileOutput{}, tool.NewFuncError("paths outside the read_file root are not allowed")
	}

	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultReadFileMaxBytes
	}
	if maxBytes > hardReadFileMaxBytes {
		maxBytes = hardReadFileMaxBytes
	}

	root, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return readFileOutput{}, fmt.Errorf("open read_file root: %w", err)
	}
	defer root.Close()

	info, err := root.Stat(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readFileOutput{}, tool.NewFuncError(fmt.Sprintf("file %q does not exist", cleanPath))
		}
		return readFileOutput{}, fmt.Errorf("stat %q: %w", cleanPath, err)
	}
	if info.IsDir() {
		return readFileOutput{}, tool.NewFuncError(fmt.Sprintf("%q is a directory", cleanPath))
	}

	file, err := root.Open(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readFileOutput{}, tool.NewFuncError(fmt.Sprintf("file %q does not exist", cleanPath))
		}
		return readFileOutput{}, fmt.Errorf("open %q: %w", cleanPath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return readFileOutput{}, fmt.Errorf("read %q: %w", cleanPath, err)
	}

	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}

	return readFileOutput{
		Path:          filepath.ToSlash(cleanPath),
		TotalBytes:    info.Size(),
		ReturnedBytes: int64(len(data)),
		Truncated:     truncated,
		Content:       string(data),
	}, nil
}
