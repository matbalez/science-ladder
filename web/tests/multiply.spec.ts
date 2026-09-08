import { test, expect } from "@playwright/test";
import { MULTIPLY_SOURCE } from "../lib/multiply-reference";
const challenge = {
  id: "browser-fixture",
  slug: "one-less-multiply",
  title: "One Less Multiply",
  summary: "Browser test fixture",
  status: "published",
  reviewStatus: "automated_pass",
  intakeStatus: "open",
  economicMode: "none",
  versionId: "test-version",
  repository: "matbalez/science-ladder-one-less-multiply",
  sourceCommit: MULTIPLY_SOURCE,
  metric: {
    name: "bilinear-products",
    direction: "minimize",
    quantum: "1",
    baselineTicks: "23",
  },
  milestones: [],
  badges: [],
  reviews: [],
  submissions: [],
  education: { frontier: "Test frontier.", significance: "Test significance." },
  manifest: {
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
  test(`Matrix reference and participation at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.route("**/v1/me", (r) =>
      r.fulfill({ json: { user: null, capabilities: {}, configuration: {} } }),
    );
    await page.route("**/v1/challenges/one-less-multiply", (r) =>
      r.fulfill({ json: challenge }),
    );
    await page.goto("/challenges/one-less-multiply");
    const region = page.getByRole("region", {
      name: "Matrix multiplication explorer",
    });
    await expect(region).toBeVisible();
    await region.getByRole("slider", { name: "Reference product" }).fill("22");
    await expect(
      region.getByText("Product 23 of 23", { exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `/tmp/science-ladder-multiply-${width}.png`,
      fullPage: true,
    });
    await page
      .getByRole("button", { name: "Participate", exact: true })
      .click();
    const prompt = await page.getByLabel(/agent instructions/i).inputValue();
    expect(prompt).toContain(MULTIPLY_SOURCE);
    expect(prompt).toContain("python3 local.py");
    expect(prompt).toContain("729 exact identities");
    expect(prompt).toContain("sl submit --api");
  });
