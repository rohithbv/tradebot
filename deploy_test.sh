#!/usr/bin/env bash
# Tests for deploy.sh — validates argument parsing, error handling,
# and docker-compose.prod.yml correctness.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== deploy.sh tests ==="

# Test 1: No arguments shows usage and exits non-zero
echo "--- Argument validation ---"
if output=$(bash "${SCRIPT_DIR}/deploy.sh" 2>&1); then
    fail "should exit non-zero with no arguments"
else
    if echo "$output" | grep -q "Usage:"; then
        pass "no arguments prints usage"
    else
        fail "no arguments should print usage message"
    fi
fi

# Test 2: Invalid host format rejected
if output=$(bash "${SCRIPT_DIR}/deploy.sh" "badformat" 2>&1); then
    fail "should reject invalid host format"
else
    if echo "$output" | grep -q "Invalid remote host format"; then
        pass "invalid host format rejected"
    else
        fail "should show 'Invalid remote host format' error"
    fi
fi

# Test 3: Multiple arguments rejected
if output=$(bash "${SCRIPT_DIR}/deploy.sh" "user@host" "extra" 2>&1); then
    fail "should reject extra arguments"
else
    if echo "$output" | grep -q "Usage:"; then
        pass "extra arguments prints usage"
    else
        fail "extra arguments should print usage message"
    fi
fi

# Test 4: deploy.sh checks remote directory instead of creating it
# The script should use "test -d" to check existence, and only mention mkdir in echo hints
if grep -q "test -d" "${SCRIPT_DIR}/deploy.sh" && ! grep -v 'echo' "${SCRIPT_DIR}/deploy.sh" | grep -q 'ssh.*mkdir'; then
    pass "deploy.sh checks remote dir existence instead of creating it"
else
    fail "deploy.sh should check remote dir, not create it"
fi

# Test 5: Script is executable
echo "--- File checks ---"
if [ -x "${SCRIPT_DIR}/deploy.sh" ]; then
    pass "deploy.sh is executable"
else
    fail "deploy.sh should be executable"
fi

# Test 5: docker-compose.prod.yml exists
if [ -f "${SCRIPT_DIR}/docker-compose.prod.yml" ]; then
    pass "docker-compose.prod.yml exists"
else
    fail "docker-compose.prod.yml should exist"
fi

# Test 6: docker-compose.prod.yml uses image (not build)
echo "--- docker-compose.prod.yml validation ---"
if grep -q "image:" "${SCRIPT_DIR}/docker-compose.prod.yml"; then
    pass "prod compose uses image directive"
else
    fail "prod compose should use image directive"
fi

if grep -q "build:" "${SCRIPT_DIR}/docker-compose.prod.yml"; then
    fail "prod compose should NOT have build directive"
else
    pass "prod compose has no build directive"
fi

# Test 7: docker-compose.prod.yml has required services config
if grep -q "tradebot-data:/data" "${SCRIPT_DIR}/docker-compose.prod.yml"; then
    pass "prod compose mounts data volume"
else
    fail "prod compose should mount tradebot-data:/data"
fi

if grep -q "env_file" "${SCRIPT_DIR}/docker-compose.prod.yml"; then
    pass "prod compose references env_file"
else
    fail "prod compose should reference env_file"
fi

if grep -q "restart:" "${SCRIPT_DIR}/docker-compose.prod.yml"; then
    pass "prod compose has restart policy"
else
    fail "prod compose should have restart policy"
fi

# Test 8: docker-compose.prod.yml is valid YAML (if docker compose is available)
if command -v docker &>/dev/null; then
    if docker compose -f "${SCRIPT_DIR}/docker-compose.prod.yml" config --quiet 2>/dev/null; then
        pass "prod compose is valid (docker compose config)"
    else
        fail "prod compose failed docker compose config validation"
    fi
else
    echo "  SKIP: docker not available for compose validation"
fi

# Test 9: Makefile has deploy target
echo "--- Makefile checks ---"
if grep -q "^deploy:" "${SCRIPT_DIR}/Makefile"; then
    pass "Makefile has deploy target"
else
    fail "Makefile should have deploy target"
fi

if grep -q "REMOTE_HOST" "${SCRIPT_DIR}/Makefile"; then
    pass "Makefile deploy uses REMOTE_HOST variable"
else
    fail "Makefile deploy should use REMOTE_HOST variable"
fi

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
