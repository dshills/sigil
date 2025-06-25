#!/bin/bash
# Comprehensive code review workflow using Sigil

set -e

echo "🔍 Starting comprehensive code review..."

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not in a git repository"
    exit 1
fi

# Get the files to review (default to changed files)
if [ $# -eq 0 ]; then
    FILES=$(git diff --name-only --diff-filter=ACMR)
    if [ -z "$FILES" ]; then
        echo "No changed files found. Checking staged files..."
        FILES=$(git diff --cached --name-only --diff-filter=ACMR)
    fi
else
    FILES="$@"
fi

if [ -z "$FILES" ]; then
    echo "❌ No files to review"
    exit 1
fi

echo "📋 Files to review:"
echo "$FILES" | sed 's/^/  - /'
echo

# Create output directory
OUTPUT_DIR=".sigil/reviews/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$OUTPUT_DIR"

# Step 1: Security review
echo "🔒 Running security review..."
sigil review $FILES \
    --check-security \
    --severity error \
    --format sarif \
    -o "$OUTPUT_DIR/security-review.sarif"

# Step 2: Performance review
echo "⚡ Running performance review..."
sigil review $FILES \
    --check-performance \
    --format markdown \
    -o "$OUTPUT_DIR/performance-review.md"

# Step 3: Code style review
echo "🎨 Running style review..."
sigil review $FILES \
    --check-style \
    --format markdown \
    -o "$OUTPUT_DIR/style-review.md"

# Step 4: Generate comprehensive report
echo "📊 Generating comprehensive report..."
sigil review $FILES \
    --check-security \
    --check-performance \
    --check-style \
    --format html \
    -o "$OUTPUT_DIR/comprehensive-review.html"

# Step 5: Auto-fix if requested
if [ "${AUTO_FIX:-false}" = "true" ]; then
    echo "🔧 Applying auto-fixes..."
    sigil review $FILES --auto-fix
    
    # Show what was changed
    echo "📝 Changes made:"
    git diff --stat
fi

# Summary
echo
echo "✅ Code review complete!"
echo "📁 Results saved to: $OUTPUT_DIR"
echo
echo "Next steps:"
echo "  1. Review the findings in $OUTPUT_DIR"
echo "  2. Address critical issues first"
echo "  3. Run 'AUTO_FIX=true $0' to apply automatic fixes"
echo "  4. Commit your changes when ready"