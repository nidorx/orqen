#!/usr/bin/env bash
# code_stats.sh — Count lines of code by extension in a directory
# Usage: ./code_stats.sh <dir>

DIR="${1:-.}"

# Convert Windows path (e.g. D:\path) to Unix (/d/path) when running under Git Bash/MSYS
case "${DIR}" in
    [A-Za-z]:\\*|[A-Za-z]:/*) DIR="$(cygpath -u "${DIR}" 2>/dev/null || echo "${DIR}")" ;;
esac

echo "=== Code Stats ==="
echo "Directory: ${DIR}"
echo ""

for ext in go ts tsx js py rs toml yaml yml md sql sh; do
  count=$(find "${DIR}" -maxdepth 10 -name "*.${ext}" -type f 2>/dev/null | wc -l)
  [ "${count}" -gt 0 ] && printf "  .%-6s %6d lines\n" "${ext}" "${count}"
done

total=$(find "${DIR}" -maxdepth 10 -type f 2>/dev/null | wc -l)
echo "  ------  ------"
printf "  total  %6d lines\n" "${total}"
