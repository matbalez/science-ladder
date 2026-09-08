#!/bin/sh
set -eu
export PATH=/workspace/lean-4.33.1-linux/bin:$PATH
export LEAN_PATH=/workspace/fixtures
cd /workspace/fixtures
for fixture in Reference Valid Changed Sorry Axiom; do
 /workspace/lean-4.33.1-linux/bin/lean -o "$fixture.olean" "$fixture.lean"
 /workspace/lean4export/.lake/build/bin/lean4export "$fixture" -- target Nat.add Nat.sub Nat.mul Nat.pow Nat.gcd Nat.div Nat.mod Nat.beq Nat.ble Nat.land Nat.lor Nat.xor Nat.shiftLeft Nat.shiftRight String.ofList Char.ofNat List eagerReduce > "$fixture.ndjson"
done
checker=/workspace/comparator/.lake/build/bin/sl-lean-check
"$checker" policy.json Reference.ndjson Valid.ndjson
for fixture in Changed Sorry Axiom; do
 if "$checker" policy.json Reference.ndjson "$fixture.ndjson"; then echo "FAIL: accepted $fixture"; exit 1; fi
done
printf '%s\n' '{"not":"a certificate"}' > Malformed.ndjson
if "$checker" policy.json Reference.ndjson Malformed.ndjson; then echo 'FAIL: malformed accepted'; exit 1; fi
printf '%s\n' 'All five serialized Lean certificate cases passed.'
