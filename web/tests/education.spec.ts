import { test, expect } from "@playwright/test";

test("challenge explains research scope and offers agent participation with readable density contours", async ({
  page,
}) => {
  test.skip(
    process.env.LIVE_RELEASE_CHECK !== "1",
    "Explicit read-only published challenge check",
  );
  await page.goto("/challenges/load-paths");
  await expect(
    page.getByRole("heading", { name: "Load Paths", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Where the research stands" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "What progress would mean" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Submit a solution" }),
  ).toHaveCount(0);
  await expect(page.locator("[data-density-contour]")).toHaveCount(5);
  const explorer = page.getByRole("region", {
    name: "Load Paths reference explorer",
  });
  await explorer.screenshot({ path: "/tmp/load-paths-contours-desktop.png" });
  await page.getByLabel("Density display").selectOption("mesh");
  await expect(page.locator("[data-density-contour]")).toHaveCount(0);
  await expect(explorer.locator("polygon")).toHaveCount(36 * 16);
  await page.getByLabel("Density display").selectOption("contours");
  for (const name of ["tall", "slender", "wide"]) {
    await page.getByRole("button", { name, exact: true }).click();
    await expect(page.locator("[data-density-contour]")).toHaveCount(5);
    await page.getByLabel("Load", { exact: true }).selectOption("2");
    await page
      .getByRole("slider", { name: "Illustrative displacement" })
      .fill("70");
    expect(
      await page.locator("[data-density-contour]").first().getAttribute("d"),
    ).not.toMatch(/NaN|Infinity/);
  }
  await page
    .getByRole("slider", { name: "Illustrative displacement" })
    .fill("0");
  await page.setViewportSize({ width: 390, height: 844 });
  await explorer.screenshot({ path: "/tmp/load-paths-contours-mobile.png" });
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  await page.getByRole("button", { name: "Participate", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
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
