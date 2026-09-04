import { test, expect } from "@playwright/test";
// Explicit test fixtures; these are never served by the production application.
const challenge = {
  id: "test-only",
  slug: "participation-test",
  title: "TEST ONLY: discrete construction",
  summary: "An explicit browser-test fixture.",
  domain: "Test mathematics",
  status: "draft",
  reviewStatus: "pending",
  intakeStatus: "closed",
  economicMode: "none",
  verificationPolicy: "platform",
  versionId: "test-version-123",
  repository: "test-owner/test-challenge",
  sourceCommit: "a".repeat(40),
  createdAt: "2026-09-04T10:00:00Z",
  deadline: "2027-09-04T23:59:59Z",
  metric: {
    name: "Test energy",
    direction: "minimize",
    units: "test units",
    quantum: "1",
    baselineTicks: "9",
  },
  milestones: [{ id: "first", label: "Test target", thresholdTicks: "8" }],
  badges: [],
  reviews: [],
  submissions: [],
  manifest: {
    scientificQuestion: "Can this test-only construction improve?",
    impact: "Test-only evidence.",
    limitations: ["Not a real challenge."],
    evidence: [
      {
        url: "https://example.org/test",
        title: "Test publication date",
        publicationDate: "2024-09-11",
        evidence: "Synthetic test citation.",
      },
      {
        url: "https://example.org/second",
        title: "Test access date",
        accessedAt: "2026-09-04",
        evidence: "Another synthetic test citation.",
      },
    ],
    submission: {
      allowedPaths: ["matrix.csv"],
      allowedExtensions: [".csv"],
      maxFiles: 1,
      maxBytes: 999,
      license: "MIT",
    },
    fixtures: [{ name: "baseline", path: "fixtures/reference-data" }],
    validator: { profile: "artifact-checker-v1" },
  },
};
test.use({
  timezoneId: "America/Vancouver",
  permissions: ["clipboard-read", "clipboard-write"],
});

test("anonymous pending challenge has complete accessible, copyable participation instructions", async ({
  page,
}) => {
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: {
        user: null,
        quotas: { remaining: 0, activeLimit: 0 },
        capabilities: { creation: false, submission: false, review: false },
        configuration: { githubAuth: true },
      },
    }),
  );
  await page.route("**/v1/challenges/participation-test", (r) =>
    r.fulfill({ json: challenge }),
  );
  await page.goto("/challenges/participation-test");
  await expect(page.getByText("Sep 11, 2024", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Source accessed Sep 4, 2026", { exact: true }),
  ).toBeVisible();
  const opener = page.getByRole("button", { name: "Participate", exact: true });
  await expect(opener).toBeEnabled();
  await expect(
    page.locator(".challenge-header-actions .primary:visible"),
  ).toHaveCount(1);
  await opener.click();
  const dialog = page.getByRole("dialog", {
    name: "Participate in this challenge",
  });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByRole("heading", { name: "Participate in this challenge" }),
  ).toBeFocused();
  const text = dialog.getByLabel("COMPLETE AGENT INSTRUCTIONS");
  const value = await text.inputValue();
  expect(value).toContain("test-version-123");
  expect(value).toContain("git checkout --detach '" + "a".repeat(40) + "'");
  expect(value).toContain("--artifact 'fixtures/reference-data'");
  expect(value).toContain("matrix.csv");
  expect(value).not.toContain("512 ASCII");
  expect(value).toContain("sl auth login");
  expect(value).toContain("--version 'test-version-123'");
  expect(value).toContain("intake: closed");
  await dialog
    .getByRole("button", { name: "Copy instructions for my agent" })
    .click();
  await expect(dialog.getByRole("status")).toContainText("Copied.");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(value);
  for (let i = 0; i < 8; i++) {
    await page.keyboard.press("Tab");
    expect(
      await dialog.evaluate((el) => el.contains(document.activeElement)),
    ).toBeTruthy();
  }
  await page.screenshot({ path: "test-results/participate-desktop.png" });
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(opener).toBeFocused();
});

test("standalone explorer keeps science central and puts tools below the visualization", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/showcase/quiet-echoes/index.html");
  await expect(
    page.getByRole("heading", { name: "Quiet Echoes." }),
  ).toBeVisible();
  await expect(page.locator("#pulse")).toBeVisible();
  await expect(page.locator("#energy")).toHaveText("17,996");
  const resources = page.locator("details.resources");
  await expect(resources).not.toHaveAttribute("open");
  await expect(
    page.getByText("Load sequence", { exact: true }),
  ).not.toBeVisible();
  expect(
    await resources.evaluate(
      (el) =>
        el.getBoundingClientRect().top >
        document.getElementById("spectrum")!.getBoundingClientRect().bottom,
    ),
  ).toBeTruthy();
  const opener = page.getByRole("button", { name: "Participate", exact: true });
  await opener.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const value = await dialog
    .getByLabel("COMPLETE AGENT INSTRUCTIONS")
    .inputValue();
  expect(value).toContain("56ddbf39-2b67-4172-9a9d-e3c78e44c7cf");
  expect(value).toContain("f42f527e97563b1c068a1835732c6da44f21223f");
  expect(value).toContain("--artifact 'fixtures/baseline'");
  expect(value).toContain("exactly 512 ASCII");
  expect(value).toContain("--license 'CC-BY-4.0'");
  await dialog
    .getByRole("button", { name: "Copy instructions for my agent" })
    .click();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(value);
  for (let i = 0; i < 8; i++) {
    await page.keyboard.press("Tab");
    expect(
      await dialog.evaluate((el) => el.contains(document.activeElement)),
    ).toBeTruthy();
  }
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({ path: "test-results/participate-mobile.png" });
  await page.keyboard.press("Escape");
  await expect(opener).toBeFocused();
  await resources.locator("summary").click();
  await page.locator("#preset").selectOption("rudin");
  await expect(page.locator("#energy")).toHaveText("43,776");
  await expect(page.locator("#source-name")).toHaveText(
    "Rudin–Shapiro comparison",
  );
  const downloadPromise = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "Download selected sequence" })
    .click();
  expect((await downloadPromise).suggestedFilename()).toBe("sequence.txt");
  await page.screenshot({
    path: "test-results/explorer-mobile.png",
    fullPage: true,
  });
});
