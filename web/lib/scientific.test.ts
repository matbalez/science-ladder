import test from "node:test";
import assert from "node:assert/strict";
import { dateLabel, formatTicks, plotRatio, safeWebUrl } from "./scientific.ts";
import { readBinaryPulse, pulseStatistics } from "./signals.ts";

test("binary pulse preview preserves exact artifact grammar", () => {
  assert.equal(readBinaryPulse("+".repeat(512) + "\n")?.length, 512);
  for (const value of [
    "+".repeat(512),
    "+".repeat(512) + "\r\n",
    "+".repeat(511) + "−\n",
    "+".repeat(513) + "\n",
  ])
    assert.equal(readBinaryPulse(value), undefined);
});

test("aperiodic signal statistics match hand-computed and closed-form cases", () => {
  assert.deepEqual(pulseStatistics([1, 1, -1, -1]).correlations, [1, -2, -1]);
  assert.equal(pulseStatistics([1, 1, -1, -1]).energy, 6);
  const constant = pulseStatistics(Array(512).fill(1));
  assert.equal(constant.energy, (511 * 512 * 1023) / 6);
  assert.equal(constant.peak, 511);
});
test("integer score display remains exact beyond JavaScript safe integers", () => {
  assert.equal(
    formatTicks("900719925474099312345", "0.000001"),
    "900,719,925,474,099.312345",
  );
  assert.equal(formatTicks("-100001", "0.001"), "-100.001");
  assert.equal(formatTicks("4", "0.25"), "1");
});
test("plot coordinates normalize huge nearby integers before converting", () => {
  assert.equal(
    plotRatio("9007199254740993", 9007199254740992n, 9007199254740994n),
    0.5,
  );
});
test("manifest links cannot execute javascript or data URLs", () => {
  assert.equal(safeWebUrl("javascript:alert(1)"), undefined);
  assert.equal(safeWebUrl("data:text/html,<script>"), undefined);
  assert.equal(
    safeWebUrl("https://arxiv.org/abs/1234.56789"),
    "https://arxiv.org/abs/1234.56789",
  );
});

test("date-only citations keep their day west and east of UTC", () => {
  const original = process.env.TZ;
  try {
    for (const zone of [
      "America/Vancouver",
      "Pacific/Honolulu",
      "Pacific/Kiritimati",
    ]) {
      process.env.TZ = zone;
      assert.equal(dateLabel("2024-09-11"), "Sep 11, 2024");
      assert.equal(dateLabel("2026-09-04"), "Sep 4, 2026");
    }
    process.env.TZ = "America/Vancouver";
    assert.equal(dateLabel("2026-09-04T00:00:00Z"), "Sep 3, 2026");
  } finally {
    if (original === undefined) delete process.env.TZ;
    else process.env.TZ = original;
  }
});

test("solver bootstrap binds exact metadata and preserves generic artifact paths", async () => {
  const { solverInstructions, CLI_SOURCE } = await import("./solver-prompt.ts");
  const challenge = {
    slug: "test-only",
    title: "Test-only construction",
    versionId: "test-version",
    repository: "test-owner/test-repo",
    sourceCommit: "a".repeat(40),
    status: "draft",
    reviewStatus: "pending",
    intakeStatus: "closed",
    summary: "Synthetic test fixture",
    metric: {
      name: "Test energy",
      direction: "minimize",
      quantum: "1",
      baselineTicks: "9",
    },
    milestones: [{ label: "Test tier", thresholdTicks: "8" }],
    manifest: {
      fixtures: [{ name: "baseline", path: "fixtures/seed's data" }],
      submission: { allowedPaths: ["matrix.csv"], license: "MIT" },
    },
  } as unknown as import("./types.ts").Challenge;
  const prompt = solverInstructions(challenge);
  assert.ok(prompt.includes(`cmd/sl@${CLI_SOURCE}`));
  assert.ok(prompt.includes("git checkout --detach '" + "a".repeat(40) + "'"));
  assert.ok(prompt.includes("--version 'test-version'"));
  assert.ok(prompt.includes("Test tier: 8"));
  assert.ok(prompt.includes("matrix.csv"));
  assert.ok(prompt.includes("404 before publication"));
  const published = solverInstructions({
    ...challenge,
    status: "published",
    reviewStatus: "human_approved",
    intakeStatus: "open",
  });
  assert.ok(!published.includes("404 before publication"));
  assert.ok(!published.includes("awaiting review or publication"));
  assert.ok(
    published.includes("Inspect the frozen version and current intake"),
  );

  assert.ok(!prompt.includes("512 ASCII"));
  assert.ok(prompt.includes("'fixtures/seed'\\''s data'"));
  assert.ok(
    prompt.includes(
      "Use the native setup, baseline and public-test commands documented by this exact challenge",
    ),
  );
  assert.ok(
    prompt.indexOf("sl challenge test") >
      prompt.indexOf("OPTIONAL EXACT-RUNTIME CONTAINER CHECK"),
  );
  assert.ok(
    prompt.includes("sl auth login --api 'https://science-ladder.fly.dev'"),
  );
  const untrusted = solverInstructions({
    ...challenge,
    sourceCommit: "$(touch /tmp/no)",
    manifest: { fixtures: [{ name: "baseline", path: "../escape" }] },
  });
  assert.ok(!untrusted.includes("git checkout"));
  assert.ok(!untrusted.includes("--artifact '../escape'"));
});

test("native Quiet Echoes setup is exact-source scoped and defers CLI/container setup", async () => {
  const { solverInstructions, challengeSetupCommands } =
    await import("./solver-prompt.ts");
  const c = {
    slug: "quiet-echoes-labs512",
    title: "Test-only metadata",
    versionId: "test-version",
    repository: "matbalez/science-ladder-quiet-echoes",
    sourceCommit: "f42f527e97563b1c068a1835732c6da44f21223f",
    status: "published",
    summary: "Test",
    metric: {
      name: "Energy",
      direction: "minimize",
      quantum: "1",
      baselineTicks: "17996",
    },
    milestones: [],
    manifest: {
      fixtures: [{ name: "baseline", path: "fixtures/baseline" }],
      submission: { allowedPaths: ["sequence.txt"], license: "CC-BY-4.0" },
    },
  } as unknown as import("./types.ts").Challenge;
  const prompt = solverInstructions(c);
  assert.ok(prompt.includes("Python 3.13 or newer on macOS or Linux"));
  assert.equal(
    (prompt.match(/python3 tools\/reproduce.py --check/g) || []).length,
    2,
  );
  assert.equal(
    (prompt.match(/python3 -m unittest discover -s tests -v/g) || []).length,
    2,
  );
  assert.ok(
    prompt.includes(
      'SL_BASELINE_RUN="$(mktemp -d "$PWD/.local/baseline.XXXXXX")"',
    ),
  );
  assert.ok(
    prompt.includes(
      'SL_CANDIDATE_RUN="$(mktemp -d "$PWD/.local/candidate.XXXXXX")"',
    ),
  );
  assert.ok(
    prompt.includes(
      'python3 checker.py --submission ../candidate-artifact --suite suite --output "$SL_CANDIDATE_RUN/result.json"',
    ),
  );
  assert.ok(
    prompt.indexOf("go install") >
      prompt.indexOf("Before final submission, repeat the full native checks"),
  );
  assert.ok(
    prompt.indexOf("sl challenge test") >
      prompt.indexOf("OPTIONAL EXACT-RUNTIME CONTAINER CHECK"),
  );
  assert.ok(!prompt.includes("Docker running for local checks"));
  const snippet = challengeSetupCommands(c);
  assert.ok(snippet.includes("python3 tools/reproduce.py --check"));
  assert.ok(snippet.includes("python3 -m unittest discover -s tests -v"));
  assert.ok(!snippet.includes("sl challenge test"));
  for (const other of [
    { ...c, repository: "other/repo" },
    { ...c, sourceCommit: "b".repeat(40) },
  ]) {
    assert.ok(
      !solverInstructions(other).includes("python3 checker.py --submission"),
    );
    assert.ok(
      !challengeSetupCommands(other).includes("python3 tools/reproduce.py"),
    );
  }
});
