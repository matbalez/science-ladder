import { test, expect } from "@playwright/test";
import { TRIANGLE_SOURCE } from "../lib/triangle-reference";
const challenge = {
  id: "browser-test-only",
  slug: "smallest-triangle",
  title: "Smallest Triangle",
  summary: "Browser test fixture",
  status: "published",
  reviewStatus: "automated_pass",
  intakeStatus: "open",
  economicMode: "none",
  versionId: "test-version",
  repository: "matbalez/science-ladder-smallest-triangle",
  sourceCommit: TRIANGLE_SOURCE,
  metric: {
    name: "minimum-area",
    direction: "maximize",
    quantum: "0.000000000000000001",
    baselineTicks: "24303979620992486",
  },
  milestones: [],
  badges: [],
  reviews: [],
  submissions: [],
  education: {
    frontier: "TEST ONLY: exact frontier construction.",
    significance: "TEST ONLY: raise the constructive lower bound.",
  },
  manifest: {
    scientificQuestion: "TEST ONLY: improve the smallest triangle",
    evidence: [],
    limitations: [],
    fixtures: [{ name: "baseline", path: "submission" }],
    validator: { profile: "native-evaluator-v2" },
    submission: { allowedPaths: ["solver.py"], license: "MIT" },
    evaluation: {
      version: "science-ladder/evaluation/v1",
      mode: "program",
      measurements: [],
    },
  },
};
for (const width of [1440, 390])
  test(`Triangle exploration and agent instructions at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.route("**/v1/me", (r) =>
      r.fulfill({ json: { user: null, capabilities: {}, configuration: {} } }),
    );
    await page.route("**/v1/challenges/smallest-triangle", (r) =>
      r.fulfill({ json: challenge }),
    );
    await page.goto("/challenges/smallest-triangle");
    const region = page.getByRole("region", {
      name: "Fourteen-point geometry explorer",
    });
    await expect(region).toBeVisible();
    await expect(
      region.getByText("0.024303979621", { exact: true }),
    ).toBeVisible();
    await expect(
      region.getByRole("slider", { name: "Bottleneck triangle" }),
    ).toHaveAttribute("max", "25");
    await region.getByRole("slider").fill("12");
    await expect(region.getByText(/13 \/ 26/)).toBeVisible();
    await region
      .getByRole("button", {
        name: "Point 1; use arrow keys to move",
        exact: true,
      })
      .press("ArrowUp");
    await expect(
      region.getByText("Your exploration", { exact: true }),
    ).toBeVisible();
    await region.getByRole("button", { name: "Reset reference" }).click();
    await expect(
      region.getByText("Best substantiated reference", { exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `/tmp/science-ladder-triangle-${width}.png`,
      fullPage: true,
    });
    await expect(
      page.getByRole("heading", { name: "Why this is worth solving" }),
    ).toBeVisible();
    await expect(
      page.getByText("Challenge record", { exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("link", { name: "Download verification record" }),
    ).toHaveCount(0);
    await page.getByRole("tab", { name: "Evaluation", exact: true }).click();
    await page.getByText("Verification record", { exact: true }).click();
    await expect(
      page.getByRole("link", { name: "Download verification record" }),
    ).toHaveAttribute("href", "/v1/exports/challenge-versions/test-version");
    await page
      .getByRole("button", { name: "Participate", exact: true })
      .click();
    const prompt = await page.getByLabel(/agent instructions/i).inputValue();
    expect(prompt).toContain(TRIANGLE_SOURCE);
    expect(prompt).toContain("python3 local.py");
    expect(prompt).toContain("python3 tools/reproduce.py");
    expect(prompt).toContain("exact frontier reference");
    await expect(
      page.getByRole("button", { name: "Submit a solution", exact: true }),
    ).toHaveCount(0);
  });
