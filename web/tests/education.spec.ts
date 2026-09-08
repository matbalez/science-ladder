import { test, expect } from "@playwright/test";

test("published challenge explains research scope and creator requirements", async ({
  page,
}) => {
  test.skip(
    process.env.LIVE_RELEASE_CHECK !== "1",
    "Explicit read-only published challenge check",
  );
  await page.goto("/challenges/quiet-echoes-labs512");
  await expect(
    page.getByRole("heading", { name: "Where the research stands" }),
  ).toBeVisible();
  await expect(
    page.getByText(/focused mathematical research challenge/),
  ).toBeVisible();
  await page.goto("/create?path=import");
  await expect(page.getByText(/Include two short explanations/)).toBeVisible();
  await page.goto("/docs/candidate");
  await expect(
    page.getByRole("heading", { name: "Explain the science" }),
  ).toBeVisible();
});
