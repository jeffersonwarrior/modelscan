#!/bin/bash

# ModelScan AI SDK Quickstart Setup
# One command to get the AI SDK working like the TypeScript AI SDK

set -e

echo "🚀 Setting up ModelScan AI SDK for Go..."
echo "========================================"

# Check if we're in the right directory
if [ ! -f "go.mod" ] && [ ! -f "../go.mod" ]; then
    echo "❌ Error: Please run this script from the modelscan root directory"
    echo "   or any subdirectory"
    exit 1
fi

# Ensure we have Go
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed. Please install Go 1.21 or newer"
    exit 1
fi

echo "✅ Go detected: $(go version)"

# Check for API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo ""
    echo "⚠️  No OPENAI_API_KEY found in environment"
    echo "   Please set your API key:"
    echo "   export OPENAI_API_KEY='sk-your-api-key'"
    echo ""
    read -p "Would you like to continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Setup the AI SDK module
echo ""
echo "📦 Setting up AI SDK modules..."

# Initialize or update the AI SDK module
cd sdk/ai 2>/dev/null || {
    echo "Creating AI SDK directory..."
    mkdir -p sdk/ai
    cd sdk/ai
}

# Download dependencies
echo "📥 Downloading dependencies..."
go mod tidy

echo "✅ AI SDK module ready"

# Setup the quickstart example
cd ../../examples/quickstart 2>/dev/null || {
    echo "Creating quickstart directory..."
    mkdir -p examples/quickstart
    cd examples/quickstart
}

echo "📥 Downloading quickstart dependencies..."
go mod tidy

echo "✅ Quickstart example ready"

# Try to build the quickstart
echo ""
echo "🔨 Testing build..."
if go build -o quickstart . 2>/dev/null; then
    echo "✅ Build successful!"
    
    # If we have an API key, test it
    if [ ! -z "$OPENAI_API_KEY" ]; then
        echo ""
        echo "🧪 Running quickstart test..."
        echo "   (This will make a small API call)"
        read -p "Continue with API test? (Y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
            if ./quickstart 2>/dev/null; then
                echo ""
                echo "🎉 SUCCESS! Your ModelScan AI SDK is working!"
            else
                echo ""
                echo "⚠️  API test failed, but the SDK is properly built."
                echo "   Check your API key and network connection."
            fi
        else
            echo ""
            echo "⏭️  Skipping API test. SDK is ready to use!"
        fi
    else
        echo ""
        echo "✅ SDK is ready! Set your API key to test:"
        echo "   export OPENAI_API_KEY='your-key'"
        echo "   ./quickstart"
    fi
else
    echo "❌ Build failed. Please check the error messages above."
    exit 1
fi

echo ""
echo "📚 Next Steps:"
echo "============"
echo "1. Try the examples: cd examples/quickstart && go run ."
echo "2. Read the documentation: cat sdk/ai/README.md"
echo "3. Explore more examples in examples/"
echo "4. Check provider-specific docs in providers/"
echo ""
echo "🔗 Want to use different providers?"
echo "   ai.NewAnthropic(\"sk-ant-key\")"
echo "   ai.NewGoogle(\"google-key\")"
echo "   ai.NewXAI(\"xai-key\")"
echo "   ...and 20+ more!"
echo ""
echo "✨ Happy coding with the ModelScan AI SDK! ✨"