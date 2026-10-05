#!/usr/bin/env bash
set -euo pipefail

tinygo version | grep -F "tinygo version 0.42.0 "
go version | grep -F "go1.27.1 "

for qualify_target in wasm wasm-unknown wasi wasip1 wasip2; do
	tinygo info "$qualify_target" | grep -E '^LLVM triple:[[:space:]]+wasm32-'
done

if tinygo targets | grep -Eq '^(wasm64|wasmgc)$'; then
	echo "TinyGo gained a Wasm64 or WasmGC target; replace this expected limitation with positive coverage." >&2
	exit 1
fi

tinygo build \
	-target=wasm-unknown -scheduler=none -gc=leaking -no-debug -p=2 \
	-o testdata/tinygo-wasm32-version.generated.wasm \
	./testdata/tinygo-wasm32-version

qualify_temp="$(mktemp -d)"
set +e
qualify_output="$(tinygo build \
	-target=wasm-unknown -scheduler=none -gc=leaking -no-debug -p=2 \
	-o "$qualify_temp/unsupported.wasm" \
	./testdata/tinygo-wasm32 2>&1)"
qualify_status=$?
set -e

if [[ $qualify_status -eq 0 ]]; then
	echo "TinyGo accepted multi-result wasm imports; replace the expected limitation with an executable round-trip guest." >&2
	exit 1
fi
if ! grep -Fq "too many return values" <<<"$qualify_output"; then
	echo "$qualify_output" >&2
	echo "TinyGo guest failed for an unexpected reason." >&2
	exit 1
fi

echo "TinyGo guest scope: Wasm32 single-result imports execute; codec multi-result imports, Wasm64, and WasmGC generation are unsupported by TinyGo 0.42.0."
