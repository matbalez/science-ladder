import { test, expect } from "@playwright/test";
import { LOAD_PATHS_SOURCE } from "../lib/solver-prompt";
const manifest = {
  scientificQuestion: "TEST ONLY: stiffness with a fixed material budget",
  impact: "Browser test fixture",
  limitations: [],
  evidence: [],
  fixtures: [{ name: "baseline", path: "submission" }],
  validator: { profile: "native-evaluator-v2" },
  submission: { allowedPaths: ["solver.py"], license: "MIT" },
  evaluation: {
    version: "science-ladder/evaluation/v1",
    mode: "program",
    measurements: [],
  },
};
const challenge = {
  id: "browser-test-only",
  slug: "load-paths",
  title: "Load Paths",
  summary: "Browser test fixture",
  status: "published",
  reviewStatus: "automated_pass",
  intakeStatus: "open",
  economicMode: "none",
  versionId: "test-version",
  repository: "matbalez/science-ladder-load-paths",
  sourceCommit: LOAD_PATHS_SOURCE,
  metric: {
    name: "robust-stiffness",
    direction: "maximize",
    quantum: "0.000001",
    baselineTicks: "1000000",
  },
  milestones: [],
  badges: [],
  reviews: [],
  submissions: [],
  manifest,
};
for (const width of [1440, 390]) {
  test(`Load Paths explorer and native participation at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.route("**/v1/me", (r) =>
      r.fulfill({ json: { user: null, capabilities: {}, configuration: {} } }),
    );
    await page.route("**/v1/challenges/load-paths", (r) =>
      r.fulfill({ json: challenge }),
    );
    await page.goto("/challenges/load-paths");
    const explorer = page.getByRole("region", {
      name: "Load Paths reference explorer",
    });
    await expect(explorer).toBeVisible();
    await expect(explorer.locator("[data-density-contour]")).toHaveCount(5);
    await explorer.getByLabel("Density display").selectOption("mesh");
    await expect(explorer.locator("polygon")).toHaveCount(36 * 16);
    await explorer.getByRole("button", { name: "tall", exact: true }).click();
    await expect(explorer.locator("polygon")).toHaveCount(28 * 20);
    await explorer
      .getByRole("combobox", { name: "Load", exact: true })
      .selectOption("2");
    const range = explorer.getByRole("slider");
    await range.fill("75");
    await expect(explorer.getByRole("img")).toHaveAttribute(
      "aria-label",
      /Load 3; illustrative displacement shown/,
    );
    await page.screenshot({
      path: `/tmp/science-ladder-load-paths-${width}.png`,
      fullPage: true,
    });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page
      .getByRole("button", { name: "Participate", exact: true })
      .click();
    const prompt = await page.getByLabel(/agent instructions/i).inputValue();
    expect(prompt).toContain("python3 -m venv");
    expect(prompt).toContain(LOAD_PATHS_SOURCE);
    expect(prompt).toContain("solver.py");
    expect(prompt).toContain("Docker Desktop is not a prerequisite");
    expect(prompt).not.toContain("sl validate --local --manifest");
  });
}
