#!/usr/bin/env bash
# Build and test the TigerSwap Java SDK.
#
# The SDK has no third-party dependencies: it uses the JDK's own HTTP client and
# a small in-tree JSON codec, so only a JDK is required. Warnings are errors so
# that an unused or unchecked construct cannot land unnoticed.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$here"

out="build/classes"
rm -rf "$out"
mkdir -p "$out"

mapfile -t sources < <(find src/main/java src/test/java -name '*.java' | sort)
if [ "${#sources[@]}" -eq 0 ]; then
  echo "no Java sources found" >&2
  exit 1
fi

javac -Xlint:all -Werror -encoding UTF-8 -d "$out" "${sources[@]}"
java -cp "$out" com.tigerswap.TigerSwapClientTest
