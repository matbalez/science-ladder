import { test, expect } from "@playwright/test";
const baseline = "24303979620992486";
const fixture = {
  id: "test",
  slug: "results-test",
  title: "Test triangle",
  summary: "Test-only geometry",
  domain: "Test mathematics",
  status: "published",
  reviewStatus: "human_approved",
  intakeStatus: "open",
  versionId: "test-version",
  repository: "test/fixture",
  sourceCommit: "a".repeat(40),
  deadline: "2027-09-08",
  badges: [],
  metric: {
    name: "minimum-area",
    direction: "maximize",
    units: "square units",
    quantum: "0.000000000000000001",
    baselineTicks: baseline,
  },
  milestones: [
    {
      id: "one",
      label: "Improve the frontier",
      thresholdTicks: "24303979620992487",
    },
    {
      id: "two",
      label: "Gain 0.00001 square units",
      thresholdTicks: "24313979620992486",
    },
    {
      id: "three",
      label: "Gain 0.0001 square units",
      thresholdTicks: "24403979620992486",
    },
  ],
  manifest: { submission: { allowedPaths: ["solver.py"], license: "MIT" } },
  submissions: [
    {
      id: "loser",
      sequence: 2,
      scoreTicks: "21267880883079842",
      outcome: "valid",
      verificationStatus: "platform_verified",
      status: "finalized",
      public: true,
      attribution: { model: "Test lower" },
      createdAt: "2026-09-08",
    },
    {
      id: "tie",
      sequence: 1,
      scoreTicks: baseline,
      outcome: "valid",
      verificationStatus: "platform_verified",
      status: "finalized",
      public: true,
      attribution: { model: "Test reference" },
      createdAt: "2026-09-08",
    },
    {
      id: "pending",
      sequence: 3,
      scoreTicks: "999999999999999999",
      outcome: "valid",
      verificationStatus: "",
      status: "pending",
      public: true,
      attribution: { model: "Test pending" },
      createdAt: "2026-09-08",
    },
  ],
};
async function mock(page: import("@playwright/test").Page, data: unknown) {
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: { user: null, capabilities: {}, configuration: {}, quotas: {} },
    }),
  );
  await page.route("**/v1/challenges/results-test", (r) =>
    r.fulfill({ json: data }),
  );
  await page.goto("/challenges/results-test");
}
for (const width of [1280, 390])
  test(`results visible without tabs and chart labels contained at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await mock(page, fixture);
    const results = page.locator(".challenge-results");
    await expect(
      results.getByRole("heading", { name: "Reference to beat" }),
    ).toBeVisible();
    await expect(
      results.getByRole("heading", { name: "Submissions (3)" }),
    ).toBeVisible();
    await expect(page.getByRole("tab", { name: "Submissions" })).toHaveCount(0);
    const rows = results.locator("tbody tr");
    await expect(rows.nth(0)).toContainText("Test reference");
    await expect(rows.nth(1)).toContainText("Test lower");
    await expect(rows.nth(2)).toContainText("Test pending");
    await results.locator(".progress-details > summary").click();
    const svg = results.locator(".frontier-chart > svg");
    await expect(svg).toBeVisible();
    await expect(svg.locator('circle[role="button"]')).toHaveCount(2);
    const contained = await svg.evaluate((el) => {
      const box = el.getBoundingClientRect();
      return Array.from(el.querySelectorAll("text")).every((t) => {
        const b = t.getBoundingClientRect();
        return b.left >= box.left - 1 && b.right <= box.right + 1;
      });
    });
    expect(contained).toBeTruthy();
    await expect(svg.locator("text").filter({ hasText: "Gain" })).toHaveCount(
      0,
    );
    await expect(
      page.getByRole("tab", { name: "Artifacts", exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByText("Milestone ladder", { exact: true }),
    ).toHaveCount(0);
    await page.getByRole("tab", { name: "Evaluation", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Version rules", exact: true }),
    ).toHaveCount(0);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(width);
    await page.screenshot({
      path: `test-results/challenge-results-${width}.png`,
      fullPage: true,
    });
  });
test("current public leader is named and highlighted without treating pending scores as wins", async ({
  page,
}) => {
  const winner = {
    ...fixture.submissions[0],
    id: "winner",
    sequence: 4,
    scoreTicks: "24323979620992486",
    attribution: { model: "Test winning model" },
  };
  await mock(page, {
    ...fixture,
    publicFrontier: { submissionId: "winner", scoreTicks: winner.scoreTicks },
    submissions: [...fixture.submissions, winner],
  });
  await expect(
    page.getByRole("heading", { name: "Current leader" }),
  ).toBeVisible();
  await expect(page.locator(".results-heading a")).toContainText(
    "Test winning model",
  );
  await expect(page.locator(".challenge-results tbody tr").first()).toHaveClass(
    "leader-row",
  );
});
