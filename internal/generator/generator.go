package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Generator generates SDK code from discovery results
type Generator struct {
	outputDir string
	templates *Templates
}

// Config holds generator configuration
type Config struct {
	OutputDir string // Directory for generated files
}

// GenerateRequest contains information for SDK generation
type GenerateRequest struct {
	ProviderID   string
	ProviderName string
	BaseURL      string
	AuthMethod   string // bearer, api-key, oauth
	AuthHeader   string
	SDKType      string // openai-compatible, anthropic-compatible, custom
	Endpoints    []Endpoint
	Models       []Model
}

// Endpoint represents an API endpoint
type Endpoint struct {
	Path    string
	Method  string
	Purpose string
}

// Model represents a model
type Model struct {
	ID   string
	Name string
}

// GenerateResult contains generation results
type GenerateResult struct {
	FilePath string
	Provider string
	Success  bool
	Error    error
}

// NewGenerator creates a new SDK generator
func NewGenerator(cfg Config) (*Generator, error) {
	if cfg.OutputDir == "" {
		cfg.OutputDir = "generated"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	templates, err := LoadTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return &Generator{
		outputDir: cfg.OutputDir,
		templates: templates,
	}, nil
}

// Generate generates SDK code for a provider
func (g *Generator) Generate(req GenerateRequest) (*GenerateResult, error) {
	result := &GenerateResult{
		Provider: req.ProviderID,
	}

	// Select template based on SDK type
	var tmpl *template.Template
	switch req.SDKType {
	case "openai-compatible":
		tmpl = g.templates.OpenAICompatible
	case "anthropic-compatible":
		tmpl = g.templates.AnthropicCompatible
	case "custom":
		tmpl = g.templates.Custom
	default:
		tmpl = g.templates.OpenAICompatible // Default to OpenAI-compatible
	}

	// Generate code from template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, req); err != nil {
		result.Error = fmt.Errorf("template execution failed: %w", err)
		return result, result.Error
	}

	// Write to file
	filename := fmt.Sprintf("%s_generated.go", req.ProviderID)
	filePath := filepath.Join(g.outputDir, filename)

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		result.Error = fmt.Errorf("failed to write file: %w", err)
		return result, result.Error
	}

	result.FilePath = filePath
	result.Success = true

	return result, nil
}

// GenerateBatch generates SDKs for multiple providers in parallel
func (g *Generator) GenerateBatch(requests []GenerateRequest) []*GenerateResult {
	results := make([]*GenerateResult, len(requests))
	resultCh := make(chan struct {
		idx    int
		result *GenerateResult
	}, len(requests))

	// Generate in parallel
	for i, req := range requests {
		go func(idx int, r GenerateRequest) {
			result, _ := g.Generate(r) // Error is captured in result.Error
			resultCh <- struct {
				idx    int
				result *GenerateResult
			}{idx, result}
		}(i, req)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		res := <-resultCh
		results[res.idx] = res.result
	}

	return results
}

// Delete removes generated SDK file
func (g *Generator) Delete(providerID string) error {
	filename := fmt.Sprintf("%s_generated.go", providerID)
	filePath := filepath.Join(g.outputDir, filename)
	return os.Remove(filePath)
}

// List lists all generated SDK files
func (g *Generator) List() ([]string, error) {
	entries, err := os.ReadDir(g.outputDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}
