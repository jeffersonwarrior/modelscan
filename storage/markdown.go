package storage

import (
	"fmt"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/jeffersonwarrior/modelscan/providers"
)

// ExportToMarkdown creates a Markdown report of all providers and their models
func ExportToMarkdown(outputPath string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Note: The parent directory should be created by the caller
	// Don't try to mkdir the outputPath which could be a file path

	// Get all provider names
	rows, err := db.Query("SELECT DISTINCT name FROM providers ORDER BY name")
	if err != nil {
		return fmt.Errorf("failed to query providers: %w", err)
	}
	defer rows.Close()

	var providerNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to scan provider name: %w", err)
		}
		providerNames = append(providerNames, name)
	}

	// Create markdown file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create markdown file: %w", err)
	}
	defer file.Close()

	// Execute markdown template
	tmpl := template.Must(template.New("report").Parse(markdownTemplate))
	data := struct {
		Providers   []string
		GeneratedAt time.Time
	}{
		Providers:   providerNames,
		GeneratedAt: time.Now(),
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Append provider details
	for _, providerName := range providerNames {
		if err := appendProviderDetails(file, providerName); err != nil {
			log.Printf("Error appending details for %s: %v", providerName, err)
		}
	}

	return nil
}

func appendProviderDetails(file *os.File, providerName string) error {
	fmt.Fprintf(file, "\n## %s\n\n", providerName)

	// Get models
	models, err := GetProviderModels(providerName)
	if err != nil {
		fmt.Fprintf(file, "Error fetching models: %v\n\n", err)
		return err
	}

	// Group models by category
	modelByCategory := make(map[string][]providers.Model)
	for _, model := range models {
		if len(model.Categories) == 0 {
			model.Categories = []string{"general"}
		}
		for _, cat := range model.Categories {
			modelByCategory[cat] = append(modelByCategory[cat], model)
		}
	}

	// Print models by category
	for category, categoryModels := range modelByCategory {
		fmt.Fprintf(file, "### %s Models\n\n", category)
		fmt.Fprintf(file, "| Name | ID | Context | Input/Output Cost | Features |\n")
		fmt.Fprintf(file, "|------|----|---------|-------------------|----------|\n")

		for _, model := range categoryModels {
			features := ""
			if model.SupportsImages {
				features += "🖼️ "
			}
			if model.SupportsTools {
				features += "🔧 "
			}
			if model.CanReason {
				features += "🧠 "
			}
			if model.CanStream {
				features += "📡 "
			}
			if features == "" {
				features = "N/A"
			}

			fmt.Fprintf(file, "| %s | %s | %d | $%.3f/$%.3f | %s |\n",
				model.Name,
				model.ID,
				model.ContextWindow,
				model.CostPer1MIn,
				model.CostPer1MOut,
				features,
			)
		}
		fmt.Fprintf(file, "\n")
	}

	// Get endpoint status
	endpoints, err := GetProviderEndpoints(providerName)
	if err != nil {
		fmt.Fprintf(file, "Error fetching endpoints: %v\n\n", err)
		return err
	}

	fmt.Fprintf(file, "### Endpoint Status\n\n")
	fmt.Fprintf(file, "| Endpoint | Method | Status | Latency |\n")
	fmt.Fprintf(file, "|----------|--------|--------|----------|\n")

	for _, endpoint := range endpoints {
		latency := "N/A"
		if endpoint.Latency > 0 {
			latency = fmt.Sprintf("%v", endpoint.Latency.Round(time.Millisecond))
		}

		status := "❌ " + endpoint.Error
		if endpoint.Status == providers.StatusWorking {
			status = "✅ Working"
		}

		fmt.Fprintf(file, "| %s | %s | %s | %s |\n",
			endpoint.Path,
			endpoint.Method,
			status,
			latency,
		)
	}
	fmt.Fprintf(file, "\n---\n\n")

	return nil
}

const markdownTemplate = `# AI Provider Validation Report

Generated on: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}

This document contains a comprehensive overview of all validated AI providers and their available models.

## Summary

- Total Providers: {{len .Providers}}
- Report generated automatically by modelscan

## Table of Contents
`

// Append storeEndpointResults from markdown.go since it's duplicated
