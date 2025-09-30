#!/bin/bash
# Test coverage script for mcp-compose
# Generates coverage reports and fails if total coverage is below threshold

set -e

COVERAGE_THRESHOLD=80
OUTPUT_DIR="coverage"
HTML_REPORT="${OUTPUT_DIR}/coverage.html"
COVERAGE_PROFILE="${OUTPUT_DIR}/coverage.out"

echo "==> Creating coverage output directory..."
mkdir -p "${OUTPUT_DIR}"

echo "==> Running tests with coverage..."
go test ./internal/... -coverprofile="${COVERAGE_PROFILE}" -covermode=atomic

echo "==> Generating HTML coverage report..."
go tool cover -html="${COVERAGE_PROFILE}" -o="${HTML_REPORT}"

echo "==> Calculating total coverage..."
TOTAL_COVERAGE=$(go tool cover -func="${COVERAGE_PROFILE}" | grep total | awk '{print $3}' | sed 's/%//')

echo "==> Coverage Results:"
echo "    Total coverage: ${TOTAL_COVERAGE}%"
echo "    Threshold: ${COVERAGE_THRESHOLD}%"
echo "    HTML report: ${HTML_REPORT}"

echo ""
echo "==> Per-package coverage:"
go tool cover -func="${COVERAGE_PROFILE}" | grep -v "total:" | grep "%" | awk '{printf "    %-50s %s\n", $1":"$2, $3}'

if (( $(echo "${TOTAL_COVERAGE} < ${COVERAGE_THRESHOLD}" | bc -l) )); then
    echo ""
    echo "ERROR: Total coverage ${TOTAL_COVERAGE}% is below threshold ${COVERAGE_THRESHOLD}%"
    exit 1
fi

echo ""
echo "SUCCESS: Coverage ${TOTAL_COVERAGE}% meets threshold ${COVERAGE_THRESHOLD}%"
exit 0