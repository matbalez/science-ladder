import { test, expect } from "@playwright/test";

// Explicit local UI fixtures; these tests never create a hosted candidate.
test.beforeEach(async ({ page }) => {
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: {
        user: null,
        quotas: { remaining: 0, activeLimit: 0 },
        capabilities: { creation: false, submission: false, review: false },
        configuration: {
          githubAuth: true,
          scientificReview: true,
          officialRunner: true,
        },
      },
    }),
  );
  await page.route("**/v1/prompts/challenge-scout/v1", (r) =>
    r.fulfill({
      json: {
        version: "1.1.0",
        prompt:
          "TEST ONLY: research {{FIELD_OR_TOPIC}}. Sources: {{SEED_PAPERS_OR_BLANK}}",
      },
    }),
  );
});

test("creation shows one path and keeps YAML requirements beside import", async ({
  page,
}) => {
  await page.goto("/create");
  await page
    .getByLabel("Field or topic", { exact: true })
    .fill("Signal design");
  await expect(page.getByLabel("Candidate YAML")).toHaveCount(0);
  await page.getByRole("button", { name: "Import a candidate" }).click();
  await expect(page.getByLabel("Candidate YAML")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Research a challenge" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "YAML format & example" }),
  ).toHaveAttribute("href", "/docs/candidate");
  await page.getByRole("button", { name: "Find a challenge" }).click();
  await expect(page.getByLabel("Field or topic", { exact: true })).toHaveValue(
    "Signal design",
  );
  await page.goto("/docs/candidate");
  await expect(
    page.getByRole("link", { name: "Candidate schema", exact: true }),
  ).toHaveAttribute("href", /challenge-candidate-v1.schema.json$/);
  await expect(
    page.getByRole("link", { name: "Manifest schema", exact: true }),
  ).toHaveAttribute("href", /challenge-manifest-v1.schema.json$/);
  await expect(
    page.getByRole("link", { name: "Download example" }),
  ).toHaveCount(2);
  await page
    .getByRole("link", { name: "Import a candidate", exact: true })
    .last()
    .click();
  await expect(page.getByLabel("Candidate YAML")).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
});

test("manifest pasted into candidate import gets a useful error before any request", async ({
  page,
}) => {
  let calls = 0;
  await page.route("**/v1/candidates/validate", (r) => {
    calls++;
    return r.fulfill({ json: { valid: false, findings: [] } });
  });
  await page.goto("/create?path=import");
  await page
    .getByLabel("Candidate YAML")
    .fill("apiVersion: science-ladder/v1\nkind: ChallengeManifest\n");
  await page.getByRole("button", { name: "Validate candidate" }).click();
  await expect(page.locator("main").getByRole("alert")).toContainText(
    "Import science-ladder-candidate.yaml here",
  );
  expect(calls).toBe(0);
  await page.getByLabel("Candidate YAML").fill("kind: ChallengeCandidate\n");
  await expect(page.locator("main").getByRole("alert")).toHaveCount(0);
});
