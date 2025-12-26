package main

import (
	"fmt"

	"github.com/jeffersonwarrior/modelscan/sdk/router"
)

func main() {
	// Demonstrate the router strategies
	fmt.Println("ModelScan Unified Router")

	fmt.Println("Router strategies available:")
	fmt.Println("  - cheapest: Minimize cost")
	fmt.Println("  - fastest: Minimize latency")
	fmt.Println("  - balanced: Balance cost and latency")
	fmt.Println("  - round_robin: Cycle through providers")
	fmt.Println("  - fallback: Try primary, fallback on failure")

	// Create routers with different strategies
	strategies := []router.RoutingStrategy{
		router.StrategyCheapest,
		router.StrategyFastest,
		router.StrategyBalanced,
		router.StrategyRoundRobin,
		router.StrategyFallback,
	}

	fmt.Println("\nCreating routers with different strategies:")
	for _, strategy := range strategies {
		r := router.NewRouter(strategy)
		fmt.Printf("  ✓ Router created with %s strategy\n", strategy)
		_ = r // use the router
	}

	fmt.Println("\nRouter provides intelligent provider selection based on:")
	fmt.Println("  • Provider health and availability")
	fmt.Println("  • Rate limit capacity")
	fmt.Println("  • Cost and latency metrics")
	fmt.Println("  • Model capabilities")
}
