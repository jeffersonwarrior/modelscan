package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		OutputDir: tmpDir,
	}

	gen, err := NewGenerator(cfg)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	if gen.outputDir != tmpDir {
		t.Errorf("expected output dir %s, got %s", tmpDir, gen.outputDir)
	}

	// Check directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("output directory was not created")
	}
}

func TestGenerateOpenAICompatible(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := NewGenerator(Config{OutputDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	req := GenerateRequest{
		ProviderID:   "testprovider",
		ProviderName: "TestProvider",
		BaseURL:      "https://api.test.com",
		SDKType:      "openai-compatible",
		Models: []Model{
			{ID: "test-model-1", Name: "Test Model 1"},
			{ID: "test-model-2", Name: "Test Model 2"},
		},
	}

	result, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	if result.Provider != "testprovider" {
		t.Errorf("expected provider testprovider, got %s", result.Provider)
	}

	// Check file was created
	expectedPath := filepath.Join(tmpDir, "testprovider_generated.go")
	if result.FilePath != expectedPath {
		t.Errorf("expected file path %s, got %s", expectedPath, result.FilePath)
	}

	// Check file contents
	content, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	contentStr := string(content)

	// Verify key elements
	if !strings.Contains(contentStr, "TestProviderProvider") {
		t.Error("generated code missing provider struct")
	}
	if !strings.Contains(contentStr, "NewTestProviderProvider") {
		t.Error("generated code missing constructor")
	}
	if !strings.Contains(contentStr, "https://api.test.com") {
		t.Error("generated code missing base URL")
	}
	if !strings.Contains(contentStr, "test-model-1") {
		t.Error("generated code missing model ID")
	}
}

func TestGenerateAnthropicCompatible(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := NewGenerator(Config{OutputDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	req := GenerateRequest{
		ProviderID:   "anthropictest",
		ProviderName: "AnthropicTest",
		BaseURL:      "https://api.anthropic-test.com",
		SDKType:      "anthropic-compatible",
	}

	result, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	content, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "CreateMessage") {
		t.Error("generated code missing CreateMessage method (Anthropic-style)")
	}
}

func TestGenerateCustomREST(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := NewGenerator(Config{OutputDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	req := GenerateRequest{
		ProviderID:   "customtest",
		ProviderName: "CustomTest",
		BaseURL:      "https://api.custom-test.com",
		SDKType:      "custom",
		Endpoints: []Endpoint{
			{Path: "/v1/generate", Method: "POST", Purpose: "text generation"},
			{Path: "/v1/embed", Method: "POST", Purpose: "embeddings"},
		},
	}

	result, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	content, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "POST /v1/generate") {
		t.Error("generated code missing endpoint documentation")
	}
}

func TestGenerateBatch(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := NewGenerator(Config{OutputDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	requests := []GenerateRequest{
		{ProviderID: "provider1", ProviderName: "Provider1", BaseURL: "https://api1.com", SDKType: "openai-compatible"},
		{ProviderID: "provider2", ProviderName: "Provider2", BaseURL: "https://api2.com", SDKType: "openai-compatible"},
		{ProviderID: "provider3", ProviderName: "Provider3", BaseURL: "https://api3.com", SDKType: "openai-compatible"},
	}

	results := gen.GenerateBatch(requests)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Errorf("generation %d failed: %v", i, result.Error)
		}
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := NewGenerator(Config{OutputDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Generate a couple of files
	gen.Generate(GenerateRequest{ProviderID: "test1", ProviderName: "Test1", BaseURL: "https://api1.com", SDKType: "openai-compatible"})
	gen.Generate(GenerateRequest{ProviderID: "test2", ProviderName: "Test2", BaseURL: "https://api2.com", SDKType: "openai-compatible"})

	files, err := gen.List()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := NewGenerator(Config{OutputDir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	result, err := gen.Generate(GenerateRequest{ProviderID: "testdelete", ProviderName: "TestDelete", BaseURL: "https://api.com", SDKType: "openai-compatible"})
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}

	// Delete
	if err := gen.Delete("testdelete"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(result.FilePath); !os.IsNotExist(err) {
		t.Error("file was not deleted")
	}
}

func TestLoadTemplates(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("failed to load templates: %v", err)
	}

	if templates.OpenAICompatible == nil {
		t.Error("OpenAI template is nil")
	}
	if templates.AnthropicCompatible == nil {
		t.Error("Anthropic template is nil")
	}
	if templates.Custom == nil {
		t.Error("Custom template is nil")
	}
}
