import { test, expect } from "@playwright/test";

// Synthetic fixtures only; no submissions or accounts are created on the server.
const me = {
  user: { id: "owner", login: "test-owner", role: "operator", invited: true },
  quotas: { remaining: 991, activeLimit: 3 },
  capabilities: { creation: true, submission: true, review: true },
  configuration: {
    githubAuth: true,
    officialRunner: true,
    scientificReview: true,
  },
};
const challenge = {
  id: "one",
  slug: "one",
  title: "Test geometry",
  domain: "Mathematics",
  status: "published",
  versionId: "new",
};
const activity = {
  challenges: [
    challenge,
    { ...challenge, versionId: "old", status: "superseded" },
  ],
  candidates: [],
  intents: [],
  submissionCount: 123,
  participation: [
    { ...challenge, open: true, submissionCount: 122, pendingCount: 0 },
    {
      id: "closed",
      slug: "closed",
      title: "Past challenge",
      status: "withdrawn",
      open: false,
      submissionCount: 1,
      pendingCount: 0,
    },
  ],
  submissionContexts: [
    {
      versionId: "new",
      title: challenge.title,
      slug: "one",
      quantum: "0.001",
      units: "square units",
    },
  ],
  submissions: [
    {
      id: "sub",
      versionId: "new",
      sequence: 2,
      scoreTicks: "25",
      status: "finalized",
      outcome: "valid",
      verificationStatus: "platform_verified",
      attribution: { model: "Test model" },
      createdAt: "2026-09-08T10:00:00Z",
    },
  ],
};

test("account emphasizes accurate activity, groups versions and keeps limits secondary", async ({
  page,
}) => {
  await page.route("**/v1/me", (r) => r.fulfill({ json: me }));
  await page.route("**/v1/dashboard", (r) => r.fulfill({ json: activity }));
  await page.goto("/account");
  const stats = page.getByRole("navigation", { name: "Your activity" });
  await expect(stats.locator("strong")).toHaveText(["1", "123", "1"]);
  await expect(page.locator("#created .account-challenge")).toHaveCount(1);
  await expect(page.locator("#submissions")).toContainText(
    "0.025 square units",
  );
  await expect(page.locator("#submissions")).toContainText("Latest 100 of 123");
  await expect(page.locator("#participation")).toContainText(
    "Local agent runs are not tracked",
  );
  await expect(
    page.locator("#participation a[href='/challenges/closed']"),
  ).toHaveCount(0);
  await expect(
    page.getByText("There is no lifetime submission limit.", { exact: false }),
  ).not.toBeVisible();
  await page.getByText("Account settings & limits", { exact: true }).click();
  await expect(
    page.getByText("There is no lifetime submission limit.", { exact: false }),
  ).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
});

test("uninvited accounts get access explanation without requesting a forbidden dashboard", async ({
  page,
}) => {
  let reads = 0;
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: {
        ...me,
        user: { ...me.user, role: "member", invited: false },
        quotas: { remaining: 0, activeLimit: 3 },
      },
    }),
  );
  await page.route("**/v1/dashboard", (r) => {
    reads++;
    return r.fulfill({ status: 403, json: {} });
  });
  await page.goto("/account");
  await expect(
    page.getByText("An operator must invite your GitHub account", {
      exact: false,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Your activity" }),
  ).toHaveCount(0);
  expect(reads).toBe(0);
});

test("activity failures are not presented as zero participation and CLI approval still works", async ({
  page,
}) => {
  await page.route("**/v1/me", (r) => r.fulfill({ json: me }));
  await page.route("**/v1/dashboard", (r) =>
    r.fulfill({
      status: 503,
      json: { error: { message: "Activity temporarily unavailable" } },
    }),
  );
  await page.route("**/v1/auth/cli-sessions/test-device/approve", (r) => {
    expect(r.request().postDataJSON()).toEqual({ userCode: "TEST-CODE" });
    return r.fulfill({ json: { approved: true } });
  });
  await page.goto("/account?cliSession=test-device&userCode=TEST-CODE");
  await expect(
    page.getByText("Activity temporarily unavailable"),
  ).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Your activity" }).locator("strong"),
  ).toHaveText(["—", "—", "—"]);
  await expect(page.getByText("No participation recorded yet.")).toHaveCount(0);
  await expect(page.getByLabel("CLI session ID")).toHaveValue("test-device");
  await page.getByRole("button", { name: "Approve my CLI session" }).click();
  await expect(
    page.getByText("CLI session approved.", { exact: false }),
  ).toBeVisible();
});
