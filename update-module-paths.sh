#!/bin/bash

# Script to update all module paths from nexora to jeffersonwarrior/modelscan
# Run this before pushing to GitHub

echo "🔄 Updating module paths to github.com/jeffersonwarrior/modelscan..."

# Update main go.mod
if [ -f "go.mod" ]; then
    echo "  ✓ Updating main go.mod"
    sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' go.mod
fi

# Update all SDK go.mod files
for dir in sdk/*/; do
    if [ -f "${dir}go.mod" ]; then
        echo "  ✓ Updating ${dir}go.mod"
        sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' "${dir}go.mod"
    fi
done

# Update unified SDK go.mod
if [ -f "sdk/go.mod" ]; then
    echo "  ✓ Updating sdk/go.mod"
    sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' sdk/go.mod
fi

# Update sdk.go imports
if [ -f "sdk/sdk.go" ]; then
    echo "  ✓ Updating sdk/sdk.go"
    sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' sdk/sdk.go
fi

# Update all example go.mod files
for dir in examples/*/; do
    if [ -f "${dir}go.mod" ]; then
        echo "  ✓ Updating ${dir}go.mod"
        sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' "${dir}go.mod"
    fi
done

# Update main.go imports
if [ -f "main.go" ]; then
    echo "  ✓ Updating main.go"
    sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' main.go
fi

# Update provider files
for file in providers/*.go; do
    if [ -f "$file" ]; then
        echo "  ✓ Updating $file"
        sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' "$file"
    fi
done

# Update config files
for file in config/*.go; do
    if [ -f "$file" ]; then
        echo "  ✓ Updating $file"
        sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' "$file"
    fi
done

# Update storage files
for file in storage/*.go; do
    if [ -f "$file" ]; then
        echo "  ✓ Updating $file"
        sed -i 's|github.com/jeffersonwarrior/modelscan|github.com/jeffersonwarrior/modelscan|g' "$file"
    fi
done

echo ""
echo "✅ All module paths updated!"
echo ""
echo "Next steps:"
echo "1. Run: make test"
echo "2. Run: git remote set-url origin https://github.com/jeffersonwarrior/modelscan.git"
echo "3. Run: git add -A"
echo "4. Run: git commit -m 'Release v1.0.0'"
echo "5. Run: git tag v1.0.0"
echo "6. Run: git push origin main --tags"
