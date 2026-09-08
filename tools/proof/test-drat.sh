#!/bin/sh
# MIT. Run only with the pinned first-party fixture bundle and reviewed checker.
set -eu
checker="$1"
fixtures="$2"
"$checker" "$fixtures/unsat.cnf" "$fixtures/valid.drat"
if "$checker" "$fixtures/sat.cnf" "$fixtures/valid.drat"; then
 echo 'FAIL: certificate for a different formula was accepted'; exit 1
fi
if "$checker" "$fixtures/unsat.cnf" "$fixtures/invalid.drat"; then
 echo 'FAIL: invalid certificate was accepted'; exit 1
fi
printf '%s\n' 'All three DRAT replay cases passed.'
