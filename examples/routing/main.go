package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jeffersonwarrior/modelscan/routing"
)

func main() {
	// Example 1: Direct Mode (default)
	fmt.Println("=== Direct Mode ===")
	directExample()

	// Example 2: Plano Proxy Mode
	fmt.Println("\n=== Plano Proxy Mode ===")
	proxyExample()

	// Example 3: Plano Embedded Mode
	fmt.Println("\n=== Plano Embedded Mode ===")
	embeddedExample()
}

// directExample demonstrates direct routing to SDK clients
func directExample() {
	// Create direct router with default config
	config := routing.DefaultConfig()
	router, err := routing.NewRouter(config)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}
	defer router.Close()

	// Register SDK clients (in real usage, these would be actual SDK clients)
	// directRouter := router.(*routing.DirectRouter)
	// directRouter.RegisterClient("openai", openaiClient)
	// directRouter.RegisterClient("anthropic", anthropicClient)

	fmt.Println("Direct router created successfully")
	fmt.Println("This mode routes directly to SDK clients with no proxy overhead")
}

// proxyExample demonstrates routing through an external Plano instance
func proxyExample() {
	// Create proxy router config
	config := routing.NewProxyConfigFromURL("http://localhost:12000")

	router, err := routing.NewRouter(config)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}
	defer router.Close()

	// Make a request
	ctx := context.Background()
	req := routing.Request{
		Model: "none", // Let Plano decide based on routing preferences
		Messages: []routing.Message{
			{
				Role:    "user",
				Content: "Write a function to calculate fibonacci numbers",
			},
		},
	}

	resp, err := router.Route(ctx, req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Model: %s\n", resp.Model)
	fmt.Printf("Provider: %s\n", resp.Provider)
	fmt.Printf("Content: %s\n", resp.Content)
	fmt.Printf("Latency: %v\n", resp.Latency)
}

// embeddedExample demonstrates managing an embedded Plano container
func embeddedExample() {
	// Check if plano_config.yaml exists
	if _, err := os.Stat("./plano_config.yaml"); os.IsNotExist(err) {
		fmt.Println("Skipping embedded example: plano_config.yaml not found")
		return
	}

	// Create embedded router config
	config := routing.NewEmbeddedConfigFromFile("./plano_config.yaml")

	// Add environment variables for API keys
	config.Embedded.Env = map[string]string{
		"OPENAI_API_KEY":    os.Getenv("OPENAI_API_KEY"),
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
		"DEEPSEEK_API_KEY":  os.Getenv("DEEPSEEK_API_KEY"),
		"GROQ_API_KEY":      os.Getenv("GROQ_API_KEY"),
	}

	router, err := routing.NewRouter(config)
	if err != nil {
		log.Printf("Failed to create router: %v", err)
		return
	}
	defer router.Close()

	embeddedRouter := router.(*routing.PlanoEmbeddedRouter)
	fmt.Printf("Embedded Plano started with container ID: %s\n", embeddedRouter.GetContainerID())

	// Make a request
	ctx := context.Background()
	req := routing.Request{
		Model: "none",
		Messages: []routing.Message{
			{
				Role:    "user",
				Content: "Explain quantum computing in simple terms",
			},
		},
		Temperature: 0.7,
		MaxTokens:   500,
	}

	resp, err := router.Route(ctx, req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Model: %s\n", resp.Model)
	fmt.Printf("Provider: %s\n", resp.Provider)
	fmt.Printf("Tokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
	fmt.Printf("Latency: %v\n", resp.Latency)
	fmt.Printf("Content: %.200s...\n", resp.Content)
}
