import { test, expect } from "@playwright/test";

test("review email controls, retry acknowledgement, and email deep link", async ({
  page,
}) => {
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: {
        user: { login: "test-editor" },
        capabilities: { review: true },
        configuration: {},
        quotas: {},
      },
    }),
  );
  await page.route("**/v1/editor/queue", (r) =>
    r.fulfill({
      json: {
        flags: [],
        reviews: [],
        candidates: [{ id: "test-candidate", status: "human_review_required" }],
        notifications: {
          configured: true,
          counts: [{ status: "uncertain", count: 1 }],
          recent: [
            {
              id: "test-email",
              status: "uncertain",
              isTest: true,
              createdAt: "2026-09-08T10:00:00Z",
              lastError: "Check SendGrid activity.",
            },
          ],
        },
      },
    }),
  );
  let testSends = 0,
    retries = 0;
  await page.route("**/v1/editor/notifications/test", (r) => {
    testSends++;
    return r.fulfill({
      status: 202,
      json: { id: "new-test", status: "pending" },
    });
  });
  await page.route("**/v1/editor/notifications/test-email/retry", (r) => {
    expect(r.request().postDataJSON()).toEqual({
      confirmPossibleDuplicate: true,
    });
    retries++;
    return r.fulfill({ json: { status: "pending" } });
  });
  await page.goto("/review?version=test-version#decision-form");
  await expect(page.getByLabel("Challenge version ID")).toHaveValue(
    "test-version",
  );
  const panel = page.getByRole("region", {
    name: "Review email notifications",
  });
  await expect(panel).toContainText(
    "New requests for human review are emailed automatically",
  );
  await panel.getByText("Email activity", { exact: false }).click();
  await expect(
    panel.getByRole("button", { name: "Retry email" }),
  ).toBeDisabled();
  await panel.getByRole("checkbox").check();
  await panel.getByRole("button", { name: "Retry email" }).click();
  await expect.poll(() => retries).toBe(1);
  await panel.getByRole("button", { name: "Send test email" }).click();
  await expect.poll(() => testSends).toBe(1);
  await expect(panel.getByRole("status")).toContainText("Email queued");
  await expect(page.locator("#candidate-test-candidate")).toHaveCount(1);
});

test("missing email configuration is visible without allowing sends", async ({
  page,
}) => {
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: {
        user: { login: "test-editor" },
        capabilities: { review: true },
        configuration: {},
        quotas: {},
      },
    }),
  );
  await page.route("**/v1/editor/queue", (r) =>
    r.fulfill({
      json: {
        flags: [],
        reviews: [],
        candidates: [],
        notifications: { configured: false, counts: [], recent: [] },
      },
    }),
  );
  await page.goto("/review");
  await expect(
    page.getByText("Email is not configured.", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Send test email" }),
  ).toBeDisabled();
});
