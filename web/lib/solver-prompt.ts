import { MULTIPLY_SOURCE } from "./multiply-reference.ts";
import { TRIANGLE_SOURCE } from "./triangle-reference.ts";
import type { Challenge } from "./types.ts";
import { asList, asRecord, asText } from "./scientific.ts";

const quote = (value: string) => "'" + value.replace(/'/g, "'\\''") + "'";

/** The native path belongs to a verified immutable source, not its display slug. */
export function hasNativeQuietEchoesChecker(
  c: Pick<Challenge, "repository" | "sourceCommit">,
): boolean {
  return (
    c.repository === "matbalez/science-ladder-quiet-echoes" &&
    c.sourceCommit === "f42f527e97563b1c068a1835732c6da44f21223f"
  );
}
export const LOAD_PATHS_SOURCE = "f738b962986c192a6f6b986db6151f57737d2f28";
export function hasNativeLoadPathsChecker(
  c: Pick<Challenge, "repository" | "sourceCommit">,
): boolean {
  return (
    c.repository === "matbalez/science-ladder-load-paths" &&
    c.sourceCommit === LOAD_PATHS_SOURCE
  );
}
export function hasNativeTriangleChecker(
  c: Pick<Challenge, "repository" | "sourceCommit">,
): boolean {
  return (
    c.repository === "matbalez/science-ladder-smallest-triangle" &&
    c.sourceCommit === TRIANGLE_SOURCE
  );
}
export function hasNativeMultiplyChecker(
  c: Pick<Challenge, "repository" | "sourceCommit">,
): boolean {
  return (
    c.repository === "matbalez/science-ladder-one-less-multiply" &&
    c.sourceCommit === MULTIPLY_SOURCE
  );
}
const triangleSetup =
  "python3 tools/reproduce.py\npython3 -m unittest discover -s validator -v\npython3 local.py";
const loadPathsSetup =
  "python3 -m venv .venv\n. .venv/bin/activate\npython -m pip install -r requirements.txt\npython validator/test_science.py\npython local.py";
export function challengeSetupCommands(c: Challenge): string {
  if (
    !/^[a-f0-9]{40}$/.test(c.sourceCommit) ||
    !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(c.repository)
  )
    return "# Verify the exact repository and full source commit before cloning.";
  const checkout = `git clone --no-checkout ${quote(`https://github.com/${c.repository}.git`)} challenge\ncd challenge\ngit checkout --detach ${quote(c.sourceCommit)}`;
  return (
    checkout +
    (hasNativeQuietEchoesChecker(c)
      ? "\npython3 tools/reproduce.py --check\npython3 -m unittest discover -s tests -v"
      : hasNativeTriangleChecker(c) || hasNativeMultiplyChecker(c)
        ? "\n" + triangleSetup
        : hasNativeLoadPathsChecker(c)
          ? "\n" + loadPathsSetup
          : "\n# Read README.md and the manifest.\n# Follow the documented native baseline and test commands, if provided.")
  );
}

export const CLI_VERSION = "0.3.0";

/** Credential-free instructions; workspace commands resolve a frozen source, never a branch. */
export function solverInstructions(c: Challenge): string {
  const m = c.manifest || {};
  const contract = asRecord(m.submission);
  const valid =
    /^[a-f0-9]{40}$/.test(c.sourceCommit) &&
    /^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(c.repository);
  return `Work on Science Ladder challenge “${c.title}”.
Page: https://scienceladder.org/challenges/${encodeURIComponent(c.slug)}
Version: ${c.versionId}
Repository: ${c.repository}
Exact source commit: ${c.sourceCommit}
Status: ${c.status}; scientific review: ${c.reviewStatus || "pending"}; intake: ${c.intakeStatus || "not open"}.
${c.status === "published" ? "Inspect the frozen version and current intake before submission." : "This version may return 404 before publication. Do not silently substitute another version."}

INSTALL THE PREBUILT CLI
Download and inspect the installer, then run it. It selects macOS/Linux and ARM64/AMD64, verifies the release checksum and installs a binary. Go and Docker Desktop are not prerequisites.
curl -fsSL https://scienceladder.org/install.sh -o /tmp/science-ladder-install.sh
# Read /tmp/science-ladder-install.sh before running it.
SL_VERSION=${CLI_VERSION} sh /tmp/science-ladder-install.sh
export PATH="$HOME/.local/bin:$PATH"
sl version

PREPARE AND REPRODUCE
${
  valid
    ? `sl clone ${quote(c.slug)} --version ${quote(c.versionId)} --out challenge-work
cd challenge-work`
    : "Source identity is invalid. Stop and obtain the exact published source; do not guess a checkout."
}
Read challenge/README.md, AGENTS.md if present, science-ladder.yaml, the scientific brief, docs/submitting.md and frontier evidence before executing challenge code. The cloned checkout is pinned and must stay unchanged; artifact/ contains an attributed copy of its baseline. Clone itself does not execute the challenge.
sl doctor
sl setup
sl run --baseline
sl run

Commands run locally with your account permissions. The setup/baseline/check recipe comes from science-ladder-local.json in the pinned source, or an exact-source compatibility recipe for existing showcases. If no recipe is available, follow the repository's documented native checks and the explicit CLI commands described at https://scienceladder.org/docs/cli; do not invent a checker.
${hasNativeQuietEchoesChecker(c) ? "Quiet Echoes needs Python 3.13 or newer on macOS or Linux. Its sequence.txt artifact is exactly 512 ASCII '+' or '-' characters followed by one LF. Expect baseline energy 17996." : hasNativeTriangleChecker(c) ? "Smallest Triangle needs Python 3.10+ standard library only. Expect minimum area 0.024303979620992486, 364 triples, and 26 exact bottlenecks. Read docs/frontier.md. Submit solver.py emitting fourteen points in the documented exact rational/algebraic format." : hasNativeMultiplyChecker(c) ? "One Less Multiply needs Python 3.10+ standard library only. Expect 23 products and 729 exact tensor identities. Submit solver.py emitting exact rational U,V,W arrays. Floating-point residuals, finite-field-only or commutative-only formulas are not valid rational bilinear solutions." : "Use the native runtime specified by the challenge. Report unmet dependencies or a failing baseline before search."}

SCIENTIFIC TASK
Question: ${asText(m.scientificQuestion, c.summary)}
Objective: ${c.metric.name}; ${c.metric.direction}; quantum ${c.metric.quantum}; baseline ticks ${c.metric.baselineTicks}.
Milestones (exact ticks): ${c.milestones.map((t) => `${t.label}: ${t.thresholdTicks}`).join("; ") || "See the manifest"}.
Allowed artifact paths: ${JSON.stringify(contract.allowedPaths || [])}.
Required artifact license: ${asText(contract.license, "See manifest")}.
Read and obey every hard gate and resource/format limit in the manifest. Never alter the checker, metric, fixtures or reference to manufacture progress.

SEARCH LOCALLY
Edit artifact/ only; keep search code and notes elsewhere in this workspace. Run sl run for feedback. Inspect every hard gate and exact comparison, not just a rounded score. Reports remain in .sl/runs/; a valid reproduction is not an improvement. Record seeds, method, actual model/harness and compute budget; disclose baseline or frontier reuse.
Do not upload baseline or non-improving attempts. No separate agent skill is required. Scientific novelty requires evidence beyond a local score.

SUBMIT ONLY A FRONTIER-POTENTIAL RESULT
When ready, repeat sl setup and sl run. Create a dedicated GitHub repository you own for artifact/ using authenticated gh or the GitHub API. Initialize Git in artifact/, commit its allowed files and push normally; configure its origin remote. Never overwrite another repository or force-push. A private repository needs the Science Ladder GitHub App installed with access to it. Follow the artifact license and keep any required attribution notes outside the scored paths.
sl auth login
# Let the user complete their own browser authorization. Never ask for a token in chat.
sl submit --model 'ACTUAL_MODEL_IDENTIFIER' --harness 'ACTUAL_HARNESS'
Replace attribution placeholders truthfully; omit an unknown model rather than inventing one.

sl submit reads the workspace identity and artifact repository, reruns the final checker, binds its report to the exact files, and compares against the current frontier. Only a qualifying claim requests server admission. The claim is unverified until the hosted checker accepts it. The API independently checks the artifact, current frontier and admission limits. Hidden-suite or hardware challenges need honest --estimated-score-ticks and --measurement-reason with their public qualification checks; never claim local access to hidden tests.
If the API returns pending processing, retain the real intent ID and use sl resume --intent ID; use sl status --submission ID after acceptance. Do not create another candidate just to retry. Hosted submission requires an invited account, open intake and available verification capacity. Keep the work and explain the actual blocker if those are unavailable.

DELIVER
Report the method, reproducible source, artifact digest, exact local result, comparison with the frontier and any real hosted submission/receipt. Public frontier advances publish their artifacts under the challenge license. Other artifacts stay private unless explicitly published. State failures and limits candidly; never claim an official win from a local score. Keep credentials, private reasoning and hidden-suite content out of public notes.`;
}
