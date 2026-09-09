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

test("prebuilt workspace instructions retain the frozen identity and scientific constraints", async () => {
  const { solverInstructions, CLI_VERSION } =
    await import("./solver-prompt.ts");
  const c = {
    slug: "test-only",
    title: "Test",
    versionId: "test-version",
    repository: "test/repo",
    sourceCommit: "a".repeat(40),
    status: "draft",
    intakeStatus: "closed",
    summary: "Test question",
    metric: {
      name: "area",
      direction: "maximize",
      quantum: "0.01",
      baselineTicks: "9",
    },
    milestones: [{ label: "Target", thresholdTicks: "10" }],
    manifest: { submission: { allowedPaths: ["matrix.csv"], license: "MIT" } },
  } as unknown as import("./types.ts").Challenge;
  const p = solverInstructions(c);
  for (const expected of [
    "SL_VERSION=" + CLI_VERSION,
    "install.sh",
    "sl clone 'test-only' --version 'test-version'",
    c.sourceCommit,
    "sl setup",
    "sl run --baseline",
    "sl submit",
    "sl auth login",
    "matrix.csv",
    "Target: 10",
    "404 before publication",
    "intake: closed",
    "Do not upload baseline or non-improving attempts",
  ])
    assert.ok(p.includes(expected), expected);
  assert.ok(!p.includes("go install"));
  assert.ok(p.indexOf("sl run --baseline") < p.indexOf("sl submit"));
  assert.ok(
    !solverInstructions({ ...c, sourceCommit: "invalid" }).includes(
      "sl clone 'test-only'",
    ),
  );
  assert.ok(
    !solverInstructions({ ...c, status: "published" }).includes(
      "404 before publication",
    ),
  );
});
test("legacy scientific guidance is bound to the exact immutable source", async () => {
  const { solverInstructions } = await import("./solver-prompt.ts");
  const c = {
    slug: "quiet-echoes-labs512",
    title: "Test",
    versionId: "test",
    repository: "matbalez/science-ladder-quiet-echoes",
    sourceCommit: "f42f527e97563b1c068a1835732c6da44f21223f",
    status: "published",
    metric: {
      name: "Energy",
      direction: "minimize",
      quantum: "1",
      baselineTicks: "17996",
    },
    milestones: [],
    manifest: {},
  } as unknown as import("./types.ts").Challenge;
  assert.ok(solverInstructions(c).includes("exactly 512 ASCII"));
  assert.ok(
    !solverInstructions({ ...c, sourceCommit: "b".repeat(40) }).includes(
      "exactly 512 ASCII",
    ),
  );
});
