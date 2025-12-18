#!/bin/bash

# Export ModelScan results to markdown format
# This script validates all providers and exports results

set -e

# Set the HOME directory if not already set
export HOME=${HOME:-/home/nexora}

echo "🔍 Running ModelScan validation..."

# Run validation for all providers and export to both formats
./modelscan --provider=all --format=all --output=./ --verbose

echo ""
echo "✓ Validation complete!"
echo "✓ Results saved to:"
echo "  - providers.db (SQLite database)"
echo "  - PROVIDERS.md (Markdown report)"
