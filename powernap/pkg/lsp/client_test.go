package lsp

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/keakon/x/powernap/pkg/lsp/protocol"
)

func TestPositionToByteOffset(t *testing.T) {
	tests := []struct {
		name      string
		lineText  string
		utf16Char uint32
		expected  int
	}{
		{
			name:      "ASCII only",
			lineText:  "hello world",
			utf16Char: 6,
			expected:  6,
		},
		{
			name:      "CJK characters (3 bytes each in UTF-8, 1 UTF-16 unit)",
			lineText:  "你好world",
			utf16Char: 2,
			expected:  6,
		},
		{
			name:      "CJK - position after CJK",
			lineText:  "var x = \"你好world\"",
			utf16Char: 11,
			expected:  15,
		},
		{
			name:      "Emoji (4 bytes in UTF-8, 2 UTF-16 units)",
			lineText:  "👋hello",
			utf16Char: 2,
			expected:  4,
		},
		{
			name:      "Multiple emoji",
			lineText:  "👋👋world",
			utf16Char: 4,
			expected:  8,
		},
		{
			name:      "Mixed content",
			lineText:  "Hello👋你好",
			utf16Char: 8,
			expected:  12,
		},
		{
			name:      "Position 0",
			lineText:  "hello",
			utf16Char: 0,
			expected:  0,
		},
		{
			name:      "Position beyond end",
			lineText:  "hi",
			utf16Char: 100,
			expected:  2,
		},
		{
			name:      "Empty string",
			lineText:  "",
			utf16Char: 0,
			expected:  0,
		},
		{
			name:      "Surrogate pair at start",
			lineText:  "𐐷hello",
			utf16Char: 2,
			expected:  4,
		},
		{
			name:      "Surrogate pair middle UTF-16 unit",
			lineText:  "𐐷hello",
			utf16Char: 1,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PositionToByteOffset(tt.lineText, tt.utf16Char)
			if result != tt.expected {
				t.Errorf("PositionToByteOffset(%q, %d) = %d, want %d",
					tt.lineText, tt.utf16Char, result, tt.expected)
			}
		})
	}
}

func TestClosedStderrPipeErrorIsSilentDuringClose(t *testing.T) {
	closing := make(chan struct{})
	if isClosedStderrPipeError(errors.New("read |0: file already closed"), closing) {
		t.Fatal("file already closed before Close should not be treated as normal shutdown")
	}
	if isClosedStderrPipeError(os.ErrClosed, closing) {
		t.Fatal("os.ErrClosed before Close should not be treated as normal shutdown")
	}
	close(closing)
	if !isClosedStderrPipeError(errors.New("read |0: file already closed"), closing) {
		t.Fatal("file already closed during Close should be treated as normal shutdown")
	}
	if !isClosedStderrPipeError(os.ErrClosed, closing) {
		t.Fatal("os.ErrClosed should be treated as normal shutdown")
	}
	if isClosedStderrPipeError(errors.New("permission denied"), closing) {
		t.Fatal("unrelated stderr read errors should still be reported")
	}
}

func TestParseOffsetEncoding(t *testing.T) {
	tests := []struct {
		name             string
		positionEncoding *protocol.PositionEncodingKind
		offsetEncoding   string
		expected         OffsetEncoding
	}{
		{
			name:           "defaults to UTF16 when no encoding provided",
			offsetEncoding: "",
			expected:       UTF16,
		},
		{
			name:           "uses legacy offsetEncoding",
			offsetEncoding: "utf-8",
			expected:       UTF8,
		},
		{
			name: "uses positionEncoding when provided",
			positionEncoding: func() *protocol.PositionEncodingKind {
				v := protocol.UTF8
				return &v
			}(),
			offsetEncoding: "utf-16",
			expected:       UTF8,
		},
		{
			name: "falls back to legacy offsetEncoding when positionEncoding unknown",
			positionEncoding: func() *protocol.PositionEncodingKind {
				v := protocol.PositionEncodingKind("unknown")
				return &v
			}(),
			offsetEncoding: "utf-32",
			expected:       UTF32,
		},
		{
			name: "defaults to UTF16 when both encodings unknown",
			positionEncoding: func() *protocol.PositionEncodingKind {
				v := protocol.PositionEncodingKind("unknown")
				return &v
			}(),
			offsetEncoding: "also-unknown",
			expected:       UTF16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOffsetEncoding(tt.positionEncoding, tt.offsetEncoding)
			if result != tt.expected {
				t.Errorf("parseOffsetEncoding(%v, %q) = %v, want %v", tt.positionEncoding, tt.offsetEncoding, result, tt.expected)
			}
		})
	}
}

func TestProcessCloser_ConcurrentClose(t *testing.T) {
	config := ClientConfig{
		Command: "cat",
		RootURI: "file:///tmp",
	}

	stream, err := startServerProcess(t.Context(), config)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = stream.Close()
		}()
	}
	wg.Wait()
}

func TestProcessCloser_CloseAfterContextCancel(t *testing.T) {
	config := ClientConfig{
		Command: "cat",
		RootURI: "file:///tmp",
	}

	ctx, cancel := context.WithCancel(t.Context())
	stream, err := startServerProcess(ctx, config)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	cancel()
	_ = stream.Close()
}

func TestProcessCloser_ConcurrentCancelAndClose(t *testing.T) {
	config := ClientConfig{
		Command: "cat",
		RootURI: "file:///tmp",
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	stream, err := startServerProcess(ctx, config)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cancel()
	}()
	go func() {
		defer wg.Done()
		_ = stream.Close()
	}()
	wg.Wait()
}

func TestSaveOptionsDecodesServerSaveCapability(t *testing.T) {
	tests := []struct {
		name        string
		sync        any
		wantInclude bool
		wantOK      bool
	}{
		{name: "no capability", sync: nil},
		{name: "sync kind only", sync: float64(2)},
		{name: "open close only", sync: map[string]any{"openClose": true, "change": float64(2)}},
		{name: "save enabled", sync: map[string]any{"save": true}, wantOK: true},
		{name: "save disabled", sync: map[string]any{"save": false}},
		{name: "save with text", sync: map[string]any{"save": map[string]any{"includeText": true}}, wantOK: true, wantInclude: true},
		{name: "save without text", sync: map[string]any{"save": map[string]any{"includeText": false}}, wantOK: true},
		{name: "save with unknown shape", sync: map[string]any{"save": map[string]any{"unexpected": 1}}, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{capabilities: protocol.ServerCapabilities{TextDocumentSync: tt.sync}}
			options, ok := c.SaveOptions()
			if ok != tt.wantOK {
				t.Fatalf("SaveOptions() ok = %v, want %v", ok, tt.wantOK)
			}
			if options.IncludeText != tt.wantInclude {
				t.Fatalf("SaveOptions() includeText = %v, want %v", options.IncludeText, tt.wantInclude)
			}
		})
	}
}

func TestMakeClientCapabilitiesDeclaresOnlyImplementedFeatures(t *testing.T) {
	caps := (&Client{}).makeClientCapabilities(false)

	textDocument, ok := caps["textDocument"].(map[string]any)
	if !ok {
		t.Fatalf("textDocument capabilities missing: %#v", caps)
	}
	synchronization, ok := textDocument["synchronization"].(map[string]any)
	if !ok {
		t.Fatalf("textDocument.synchronization capabilities missing: %#v", textDocument)
	}
	if synchronization["openClose"] != true || synchronization["didSave"] != true {
		t.Fatalf("openClose/didSave should be declared, got %#v", synchronization)
	}
	for _, key := range []string{"willSave", "willSaveWaitUntil"} {
		if _, ok := synchronization[key]; ok {
			t.Fatalf("%s has no sending path and must not be declared", key)
		}
	}

	workspace, ok := caps["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("workspace capabilities missing: %#v", caps)
	}
	// These invite server-initiated requests (workspace/applyEdit,
	// workspace/workspaceFolders, workspace/will*Files) that the client does
	// not answer, so they must stay undeclared.
	for _, key := range []string{"applyEdit", "workspaceEdit", "workspaceFolders", "fileOperations"} {
		if _, ok := workspace[key]; ok {
			t.Fatalf("workspace.%s must not be declared", key)
		}
	}

	// window/workDoneProgress, window/showMessageRequest and window/showDocument
	// are server-initiated requests without handlers; declaring the window
	// capabilities would invite them.
	if _, ok := caps["window"]; ok {
		t.Fatalf("window capabilities must not be declared without handlers: %#v", caps["window"])
	}

	general, ok := caps["general"].(map[string]any)
	if !ok {
		t.Fatalf("general capabilities missing: %#v", caps)
	}
	encodings, ok := general["positionEncodings"].([]string)
	if !ok || len(encodings) != 1 || encodings[0] != "utf-16" {
		t.Fatalf("positionEncodings = %#v, want [utf-16]", general["positionEncodings"])
	}
}
