package lsp

import (
	"context"
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

func TestRequestDefinitionRawRequiresInitializedClient(t *testing.T) {
	c := &Client{}
	_, err := c.RequestDefinitionRaw(context.Background(), "file:///tmp/main.go", protocol.Position{})
	if err == nil {
		t.Fatal("RequestDefinitionRaw() error = nil, want initialized error")
	}
}

func TestRequestImplementationRequiresInitializedClient(t *testing.T) {
	c := &Client{}
	_, err := c.RequestImplementation(context.Background(), "file:///tmp/main.go", protocol.Position{})
	if err == nil {
		t.Fatal("RequestImplementation() error = nil, want initialized error")
	}
}

func TestMakeClientCapabilitiesIncludesImplementation(t *testing.T) {
	c := &Client{}
	caps := c.makeClientCapabilities(false)
	textDocument, ok := caps["textDocument"].(map[string]any)
	if !ok {
		t.Fatalf("textDocument capabilities type = %T, want map[string]any", caps["textDocument"])
	}
	impl, ok := textDocument["implementation"].(map[string]any)
	if !ok {
		t.Fatalf("implementation capability type = %T, want map[string]any", textDocument["implementation"])
	}
	if impl["dynamicRegistration"] != true {
		t.Fatalf("implementation dynamicRegistration = %#v, want true", impl["dynamicRegistration"])
	}
	if impl["linkSupport"] != true {
		t.Fatalf("implementation linkSupport = %#v, want true", impl["linkSupport"])
	}
}
