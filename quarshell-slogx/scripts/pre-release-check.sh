#!/usr/bin/env bash
# Pre-Release Validation Script for slogx
# Mirrors CI checks + additional local validations.
#
# Usage:
#   bash scripts/pre-release-check.sh          # full check before release
#   bash scripts/pre-release-check.sh --quick  # skip coverage and lint

set -e

QUICK=0
[[ "${1:-}" == "--quick" ]] && QUICK=1

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC}   $1"; }
log_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[FAIL]${NC} $1"; }

echo ""
echo "================================================"
echo "  slogx — Pre-Release Check"
echo "================================================"
echo ""

ERRORS=0
WARNINGS=0

# 1. Go version
log_info "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}')
REQUIRED="go1.27"
if [[ "$GO_VERSION" < "$REQUIRED" ]]; then
    log_error "Go $REQUIRED+ required, found $GO_VERSION"
    ERRORS=$((ERRORS + 1))
else
    log_success "Go version: $GO_VERSION"
fi
echo ""

# 2. Git status
log_info "Checking git status..."
if git diff-index --quiet HEAD -- 2>/dev/null; then
    log_success "Working directory is clean"
else
    log_warning "Uncommitted changes detected"
    git status --short
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 3. Formatting
log_info "Checking code formatting..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    log_error "Files need formatting:"
    echo "$UNFORMATTED"
    echo "  Run: go fmt ./..."
    ERRORS=$((ERRORS + 1))
else
    log_success "All files formatted"
fi
echo ""

# 4. go vet
log_info "Running go vet..."
if go vet ./... 2>&1; then
    log_success "go vet passed"
else
    log_error "go vet failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 5. Build
log_info "Building..."
if go build ./... 2>&1; then
    log_success "Build OK"
else
    log_error "Build failed"
    ERRORS=$((ERRORS + 1))
fi

log_info "Building examples..."
EX_FAIL=0
for d in examples/*/; do
    if ! go build -o /dev/null "./$d" 2>&1; then
        log_error "Example build failed: $d"
        EX_FAIL=1
    fi
done
if [ "$EX_FAIL" -eq 0 ]; then
    log_success "Examples build OK"
else
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 6. go.mod
log_info "Validating go.mod..."
go mod verify && log_success "go.mod verified" || { log_error "go.mod verify failed"; ERRORS=$((ERRORS + 1)); }

go mod tidy
MOD_FILES="go.mod"
[ -f go.sum ] && MOD_FILES="go.mod go.sum"
if git diff --quiet $MOD_FILES 2>/dev/null; then
    log_success "go.mod is tidy"
else
    log_warning "go.mod needs tidying (run 'go mod tidy')"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 7. Tests with race detector
log_info "Running tests..."
if go test -race -count=1 ./... 2>&1; then
    log_success "Tests passed"
else
    log_error "Tests failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

if [ "$QUICK" -eq 0 ]; then
    # 8. Coverage
    log_info "Checking test coverage..."
    COV_PROFILE=$(mktemp -t slogx-cov)
    go list ./... | grep -v '/examples/' | \
        xargs go test -count=1 -coverprofile="$COV_PROFILE" -covermode=atomic 2>/dev/null
    COVERAGE=$(go tool cover -func="$COV_PROFILE" | tail -1 | awk '{print $3}' | sed 's/%//')
    rm -f "$COV_PROFILE"
    if [ -n "$COVERAGE" ]; then
        echo "  overall: ${COVERAGE}%"
        if awk -v cov="$COVERAGE" 'BEGIN {exit !(cov >= 80.0)}'; then
            log_success "Coverage >= 80%"
        else
            log_warning "Coverage below 80% (${COVERAGE}%)"
            WARNINGS=$((WARNINGS + 1))
        fi
    else
        log_warning "Could not determine coverage"
        WARNINGS=$((WARNINGS + 1))
    fi
    echo ""

    # 9. golangci-lint
    log_info "Running golangci-lint..."
    if command -v golangci-lint &>/dev/null; then
        if golangci-lint run --timeout=5m ./... 2>&1; then
            log_success "golangci-lint passed"
        else
            log_error "golangci-lint found issues"
            ERRORS=$((ERRORS + 1))
        fi
    else
        log_warning "golangci-lint not installed — https://golangci-lint.run/welcome/install/"
        WARNINGS=$((WARNINGS + 1))
    fi
    echo ""
fi

# 10. Required docs
log_info "Checking required files..."
for f in README.md LICENSE CHANGELOG.md CONTRIBUTING.md; do
    if [ -f "$f" ]; then
        log_success "$f present"
    else
        log_error "$f missing"
        ERRORS=$((ERRORS + 1))
    fi
done
echo ""

# 11. TODO/FIXME
log_info "Checking for TODO/FIXME..."
TODO_COUNT=$(grep -r "TODO\|FIXME" --include="*.go" --exclude-dir=vendor . 2>/dev/null | wc -l | tr -d ' ')
if [ "$TODO_COUNT" -gt 0 ]; then
    log_warning "Found $TODO_COUNT TODO/FIXME comments"
    grep -r "TODO\|FIXME" --include="*.go" --exclude-dir=vendor . 2>/dev/null | head -5
    WARNINGS=$((WARNINGS + 1))
else
    log_success "No TODO/FIXME found"
fi
echo ""

# Summary
echo "========================================"
echo "  Summary"
echo "========================================"
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    log_success "All checks passed — ready for release."
    echo ""
    echo "  Next steps:"
    echo "    1. Update CHANGELOG.md and README.md"
    echo "    2. git commit -m \"chore: prepare vX.Y.Z release\""
    echo "    3. Wait for CI to pass on main"
    echo "    4. git tag -a vX.Y.Z -m \"Release vX.Y.Z\""
    echo "    5. git push origin vX.Y.Z"
    echo ""
    exit 0
elif [ $ERRORS -eq 0 ]; then
    log_warning "Passed with $WARNINGS warning(s) — review before release."
    exit 0
else
    log_error "Failed: $ERRORS error(s), $WARNINGS warning(s) — fix before release."
    exit 1
fi
