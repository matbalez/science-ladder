import { test, expect } from "@playwright/test";
test("Load Paths is withdrawn and Quiet Echoes remains published with its original frontier reference", async ({
  page,
  request,
}) => {
  test.skip(
    process.env.LIVE_RELEASE_CHECK !== "1",
    "Explicit read-only production smoke check",
  );
  const load = await request.get("/v1/challenges/load-paths");
  expect(load.status()).toBe(410);
  const listing = await (await request.get("/v1/challenges")).json();
  expect(
    listing.challenges.some((c: { slug: string }) => c.slug === "load-paths"),
  ).toBe(false);
  expect(
    (await request.get("/showcase/load-paths/reference.json")).status(),
  ).toBe(404);
  await page.goto("/challenges/load-paths");
  await expect(
    page.getByRole("heading", { name: "Challenge removed", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Participate", exact: true }),
  ).toHaveCount(0);
  const quiet = await request.get("/v1/challenges/quiet-echoes-labs512");
  expect(quiet.ok()).toBe(true);
  const old = await quiet.json();
  expect(old.status).toBe("published");
  expect(old.metric.baselineTicks).toBe("17996");
  expect(old.sourceCommit).toBe("f42f527e97563b1c068a1835732c6da44f21223f");
  expect(old.lockDigest).toBe(
    "sha256:ae2103aca32a90c6bb166745cbd6aa2fcfbc3381fe01a1a27f2190afb7bfbbd4",
  );
  expect(old.submissions.length).toBeGreaterThanOrEqual(3);
  await page.goto("/challenges/quiet-echoes-labs512");
  await expect(
    page.getByRole("button", { name: "Participate", exact: true }),
  ).toBeVisible();
});
