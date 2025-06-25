#!/bin/bash
# Comprehensive documentation generation using Sigil

set -e

# Configuration
PROJECT_NAME="${PROJECT_NAME:-$(basename $(pwd))}"
OUTPUT_DIR="${OUTPUT_DIR:-docs}"
INCLUDE_PRIVATE="${INCLUDE_PRIVATE:-false}"

echo "📚 Generating comprehensive documentation for $PROJECT_NAME..."
echo

# Ensure we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "⚠️  Warning: Not in a git repository"
fi

# Create output directory structure
mkdir -p "$OUTPUT_DIR"/{api,guides,architecture,examples}

# Step 1: Generate API documentation
echo "🔌 Generating API documentation..."
if [ "$INCLUDE_PRIVATE" = "true" ]; then
    PRIVATE_FLAG="--include-private"
else
    PRIVATE_FLAG=""
fi

# Find source files by language
if [ -f "go.mod" ]; then
    echo "  Detected Go project"
    find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | \
        sigil doc --format markdown $PRIVATE_FLAG -o "$OUTPUT_DIR/api/"
elif [ -f "package.json" ]; then
    echo "  Detected JavaScript/TypeScript project"
    find . -name "*.js" -o -name "*.ts" -not -path "./node_modules/*" -not -path "./.git/*" | \
        sigil doc --format markdown $PRIVATE_FLAG -o "$OUTPUT_DIR/api/"
elif [ -f "requirements.txt" ] || [ -f "setup.py" ]; then
    echo "  Detected Python project"
    find . -name "*.py" -not -path "./.git/*" -not -path "./venv/*" | \
        sigil doc --format markdown $PRIVATE_FLAG -o "$OUTPUT_DIR/api/"
else
    echo "  Using generic documentation"
    sigil doc . --recursive --format markdown $PRIVATE_FLAG -o "$OUTPUT_DIR/api/"
fi

# Step 2: Generate architecture documentation
echo "🏗️ Generating architecture documentation..."
sigil explain . \
    --query "Describe the overall architecture of this project, including key components, their relationships, and design decisions" \
    --detailed \
    -o "$OUTPUT_DIR/architecture/overview.md"

sigil explain . \
    --query "Create a detailed component diagram description and list all major modules with their responsibilities" \
    -o "$OUTPUT_DIR/architecture/components.md"

# Step 3: Generate user guides
echo "📖 Generating user guides..."

# Getting started guide
sigil ask "Based on this codebase, write a comprehensive Getting Started guide for new users" \
    -o "$OUTPUT_DIR/guides/getting-started.md"

# Configuration guide
sigil ask "Create a detailed configuration guide explaining all configuration options and best practices" \
    -o "$OUTPUT_DIR/guides/configuration.md"

# Best practices guide
sigil ask "Write a best practices guide for using and contributing to this project" \
    -o "$OUTPUT_DIR/guides/best-practices.md"

# Step 4: Generate examples
echo "💡 Generating examples..."
sigil ask "Create 5 practical examples showing common use cases for this project" \
    -o "$OUTPUT_DIR/examples/common-use-cases.md"

# Step 5: Generate README sections
echo "📄 Generating README content..."
sigil summarize . \
    --brief \
    --focus "key features and capabilities" \
    -o "$OUTPUT_DIR/readme-features.md"

# Step 6: Generate API reference index
echo "📑 Creating API reference index..."
cat > "$OUTPUT_DIR/api/index.md" << EOF
# API Reference

This directory contains the complete API documentation for $PROJECT_NAME.

## Contents

EOF

# Add file listings
find "$OUTPUT_DIR/api" -name "*.md" -not -name "index.md" | sort | while read -r file; do
    basename=$(basename "$file" .md)
    echo "- [${basename}](${basename}.md)" >> "$OUTPUT_DIR/api/index.md"
done

# Step 7: Generate main documentation index
echo "🏠 Creating documentation index..."
cat > "$OUTPUT_DIR/index.md" << EOF
# $PROJECT_NAME Documentation

Welcome to the $PROJECT_NAME documentation!

## Documentation Structure

- [API Reference](api/index.md) - Complete API documentation
- [Architecture](architecture/overview.md) - System architecture and design
- [Guides](guides/getting-started.md) - User and developer guides
- [Examples](examples/common-use-cases.md) - Practical examples

## Quick Links

- [Getting Started](guides/getting-started.md)
- [Configuration](guides/configuration.md)
- [Best Practices](guides/best-practices.md)
- [Components](architecture/components.md)

Generated on: $(date)
EOF

# Step 8: Generate change log from git history
if git rev-parse --git-dir > /dev/null 2>&1; then
    echo "📜 Generating changelog..."
    sigil ask "Based on the git history, create a CHANGELOG.md with version history and notable changes" \
        -o "$OUTPUT_DIR/CHANGELOG.md"
fi

# Summary
echo
echo "✅ Documentation generation complete!"
echo "📁 Documentation saved to: $OUTPUT_DIR/"
echo
echo "📊 Generated files:"
find "$OUTPUT_DIR" -type f -name "*.md" | wc -l | xargs echo "  - Markdown files:"
echo
echo "Next steps:"
echo "  1. Review generated documentation in $OUTPUT_DIR/"
echo "  2. Customize and edit as needed"
echo "  3. Set up a documentation site (e.g., MkDocs, Docusaurus)"
echo "  4. Add to your CI/CD pipeline for automatic updates"