import { test, expect } from "@playwright/test";
test("a removed challenge has no solver instructions or illustration", async ({
  page,
}) => {
  await page.route("**/v1/me", (r) =>
    r.fulfill({ json: { user: null, capabilities: {}, configuration: {} } }),
  );
  await page.route("**/v1/challenges/removed-test", (r) =>
    r.fulfill({
      status: 410,
      json: {
        error: {
          code: "challenge_withdrawn",
          message: "This challenge was removed. Submissions are closed.",
        },
      },
    }),
  );
  await page.goto("/challenges/removed-test");
  await expect(
    page.getByRole("heading", { name: "Challenge removed" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Participate", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Load Paths reference explorer" }),
  ).toHaveCount(0);
});
