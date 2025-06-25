#!/bin/bash
# Intelligent codebase refactoring using Sigil

set -e

# Configuration
LANGUAGE="${LANGUAGE:-go}"
TARGET_DIR="${1:-.}"
REFACTOR_TYPE="${2:-improve}"

echo "🔄 Starting intelligent refactoring..."
echo "📁 Target directory: $TARGET_DIR"
echo "🔧 Refactor type: $REFACTOR_TYPE"
echo

# Ensure we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not in a git repository"
    exit 1
fi

# Create a new branch for refactoring
BRANCH_NAME="refactor/ai-${REFACTOR_TYPE}-$(date +%Y%m%d_%H%M%S)"
echo "🌿 Creating branch: $BRANCH_NAME"
git checkout -b "$BRANCH_NAME"

# Step 1: Analyze current architecture
echo "🔍 Analyzing current codebase..."
sigil explain "$TARGET_DIR" \
    --query "What is the current architecture and what improvements would you suggest?" \
    --detailed \
    -o .sigil/analysis/current-architecture.md

# Step 2: Create refactoring plan
echo "📋 Creating refactoring plan..."
sigil ask "Based on the codebase in $TARGET_DIR, create a detailed refactoring plan to $REFACTOR_TYPE the code. Focus on maintainability, testability, and performance." \
    -o .sigil/analysis/refactoring-plan.md

# Step 3: Apply refactoring based on type
case "$REFACTOR_TYPE" in
    "improve")
        echo "✨ Improving code quality..."
        find "$TARGET_DIR" -name "*.$LANGUAGE" -type f | while read -r file; do
            echo "  Processing: $file"
            sigil edit "$file" \
                -d "Improve code quality: add error handling, improve variable names, add comments where needed, optimize performance" \
                --validate
        done
        ;;
    
    "modularize")
        echo "📦 Modularizing codebase..."
        sigil edit "$TARGET_DIR" \
            -d "Refactor to improve modularity: extract interfaces, separate concerns, reduce coupling between components" \
            --validate \
            --recursive
        ;;
    
    "patterns")
        echo "🏗️ Applying design patterns..."
        sigil edit "$TARGET_DIR" \
            -d "Apply appropriate design patterns: use dependency injection, implement factory patterns where suitable, add builders for complex objects" \
            --validate \
            --recursive
        ;;
    
    "testable")
        echo "🧪 Making code more testable..."
        find "$TARGET_DIR" -name "*.$LANGUAGE" -type f | while read -r file; do
            echo "  Processing: $file"
            sigil edit "$file" \
                -d "Refactor for testability: extract dependencies, use interfaces, avoid global state, make functions pure where possible" \
                --validate
        done
        ;;
    
    *)
        echo "❌ Unknown refactor type: $REFACTOR_TYPE"
        echo "Valid types: improve, modularize, patterns, testable"
        exit 1
        ;;
esac

# Step 4: Generate updated documentation
echo "📚 Updating documentation..."
sigil doc "$TARGET_DIR" \
    --recursive \
    --update-existing \
    -o docs/

# Step 5: Review changes
echo "🔍 Reviewing refactored code..."
sigil review "$TARGET_DIR" \
    --check-performance \
    --check-style \
    -o .sigil/analysis/post-refactor-review.md

# Step 6: Run tests (if available)
if [ -f "go.mod" ]; then
    echo "🧪 Running tests..."
    go test ./... || echo "⚠️  Some tests failed - please review"
elif [ -f "package.json" ]; then
    echo "🧪 Running tests..."
    npm test || echo "⚠️  Some tests failed - please review"
fi

# Step 7: Generate summary
echo "📊 Generating refactoring summary..."
sigil summarize "$TARGET_DIR" \
    --focus "changes and improvements" \
    -o .sigil/analysis/refactoring-summary.md

# Show results
echo
echo "✅ Refactoring complete!"
echo
echo "📈 Summary of changes:"
git diff --stat
echo
echo "📁 Analysis reports saved to: .sigil/analysis/"
echo
echo "Next steps:"
echo "  1. Review the changes with: git diff"
echo "  2. Check the analysis reports in .sigil/analysis/"
echo "  3. Run your test suite to ensure everything works"
echo "  4. Commit when satisfied: git commit -am 'Refactor: $REFACTOR_TYPE using AI assistance'"
echo "  5. Create a pull request for review"