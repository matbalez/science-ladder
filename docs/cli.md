# Science Ladder CLI

The MIT-licensed `sl` CLI is available as prebuilt macOS and Linux binaries for ARM64 and AMD64. No Go compiler is required. Download links and checksums live in the public [CLI releases](https://github.com/matbalez/science-ladder/releases/tag/cli-v0.3.0). Windows is not supported yet.

Download https://scienceladder.org/install.sh, inspect it, then run `sh install.sh`. The installer selects the correct binary, checks SHA-256 against the versioned release manifest, and installs to `~/.local/bin` without sudo. Add that directory to PATH. `SL_INSTALL_DIR` overrides the destination; `SL_VERSION=0.3.0` pins the release. Checksums detect corrupt or mismatched downloads; their trust comes from the same HTTPS GitHub release, not an independent signature.

## Solver workflow

```sh
sl clone smallest-triangle --out triangle-work
cd triangle-work
sl doctor
# Read challenge/README.md, scientific evidence and the recipe first.
sl setup
sl run --baseline
# Edit artifact/solver.py, keeping search scripts and notes outside artifact/.
sl run
```

`sl clone` fetches the published source at its exact commit, checks the manifest against the platform version, and copies its attributed baseline into `artifact/`. Pass `--version ID` to refuse a later version. An existing destination is never overwritten. The new workspace contains:

- `challenge/`: frozen source, checker, evidence and fixtures. Keep tracked source unchanged.
- `artifact/`: your candidate, initially copied from the reference.
- `.sl/workspace.json`: platform, version, source identity and manifest digest; no credentials.
- `.sl/runs/`: separate local reports for every run and final claim.

`sl clone` does not run the challenge. `sl setup` explicitly runs its declared dependency/reproduction commands with your normal account permissions. `sl run --baseline` reproduces the reference; `sl run` checks the candidate. Docker is only needed for a recipe that uses it. These are local results, not platform verification. Native checkers and the public API remain usable without the CLI.

When you have an improvement, create a dedicated GitHub repository for `artifact/` through `gh` or the GitHub API, commit and push the allowed files, and set its origin remote. Keep required attribution, licenses and method documentation as specified by the challenge; do not include forbidden files in its artifact contract. Private repositories require the Science Ladder GitHub App's access.

```sh
sl auth login
sl submit --model 'your actual model' --harness 'your agent software'
```

`sl submit` finds the workspace even from a child directory. It requires a clean, separate artifact repository, derives the repository and commit, reruns the local checker and creates a fresh claim. Only an eligible frontier claim proceeds to admission and hosted verification. The server independently validates it; the claim is not proof of correctness. Hidden tests and hardware benchmarks additionally require an honest `--estimated-score-ticks` and `--measurement-reason` after passing public qualification.

Keep the returned IDs. Use `sl resume --intent ID` for pending/interrupted intake and `sl status --submission ID` for accepted submissions. Retrying an existing intent avoids creating another candidate. `sl doctor` checks the pinned checkout, recipe, executables, public policy and saved sign-in. Missing sign-in or an artifact repository is informational during local exploration. It never installs dependencies or starts a verification job.

Existing explicit `sl claim`, `sl submit --version ...`, `sl validate --local`, receipt verification, export, scaffold and lint commands remain available. Use `sl --help`. A workspace without a supported local recipe can still use documented native commands and explicit claim/submission flags.

## Creator local recipe

Include **science-ladder-local.json** in the challenge repository before publication. It is a versioned convenience contract outside the scientific manifest; publication pins it with the source commit. The scientific manifest still defines all authoritative gates, scoring and resources. This file does not alter hosted verification or the human review requirement.

```json
{
  "version": 1,
  "setup": [
    ["python3", "tools/reproduce.py"],
    ["python3", "-m", "unittest", "discover", "-s", "validator", "-v"]
  ],
  "baseline": ["python3", "local.py", "--output", "{report}"],
  "check": ["python3", "local.py", "--solver", "{artifact}/solver.py", "--output", "{report}"]
}
```

Each command is an argument array executed from `challenge/`, with **no implicit shell**. Use a committed script for compound setup. `{artifact}` expands to the candidate directory and `{report}` to a fresh result file outside it. Both baseline and check must write `{report}`; check must use `{artifact}`. Local commands must fail with a nonzero exit on failed gates. The final report should use the protocol `ValidatorResult` format. Declaring `checkedGates` is supported only for legacy native reports containing score/measurements and no validation metadata; it explicitly asserts those gate checks and does not confer server trust.

Document runtime versions, dependencies, the expected baseline and every gate, including which checks cannot run locally. Setup must be repeatable. Test the recipe from a fresh clone, with valid, invalid and non-improving candidates. Never download or run changing branch content as a substitute for the pinned checker. Only the recipe tracked at the exact commit is loaded. Three existing immutable showcases use CLI compatibility recipes bound to their exact repository and commit; their scientific versions were not changed for this release.

## Release maintenance

Tag a tested source commit `cli-vX.Y.Z`. The release workflow builds four static binaries, embeds version/source identity, smoke-tests Linux AMD64 and publishes SHA256SUMS plus source.json. Update the installer default and Participate version together only after that release is available. Building from source remains optional: `go build -o sl ./cmd/sl`.
