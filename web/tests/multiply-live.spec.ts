import { test, expect } from "@playwright/test";
import { MULTIPLY_SOURCE } from "../lib/multiply-reference";

test("published matrix challenge has a verified frontier and agent participation", async ({
  page,
  request,
}) => {
  test.skip(
    process.env.LIVE_RELEASE_CHECK !== "1",
    "Explicit production check",
  );
  const response = await request.get("/v1/challenges/one-less-multiply");
  expect(response.ok()).toBe(true);
  const challenge = await response.json();
  expect(challenge.status).toBe("published");
  expect(challenge.reviewStatus).toBe("automated_pass");
  expect(challenge.sourceCommit).toBe(MULTIPLY_SOURCE);
  expect(challenge.metric.baselineTicks).toBe("23");
  expect(challenge.verificationPolicy).toBe("platform");
  expect(challenge.education.significance).toMatch(/algebraic-complexity/i);
  expect(challenge.lockDigest).toMatch(/^sha256:[a-f0-9]{64}$/);
  await page.goto("/challenges/one-less-multiply");
  await expect(
    page.getByRole("region", { name: "Matrix multiplication explorer" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Why this is worth solving" }),
  ).toBeVisible();
  await expect(page.getByText("Challenge record", { exact: true })).toHaveCount(
    0,
  );
  await page.screenshot({path: "/tmp/science-ladder-multiply-live.png", fullPage: true});
  await page.getByRole("tab", { name: "Evaluation", exact: true }).click();
  await page.getByText("Verification record", { exact: true }).click();
  const link = page.getByRole("link", { name: "Download verification record" });
  const record = await request.get((await link.getAttribute("href"))!);
  expect(record.ok()).toBe(true);
  expect((await record.json()).kind).toBe("ChallengeExport");
  await page.getByRole("button", { name: "Participate", exact: true }).click();
  const prompt = await page.getByLabel(/agent instructions/i).inputValue();
  expect(prompt).toContain(MULTIPLY_SOURCE);
  expect(prompt).toContain("python3 local.py");
  expect(prompt).toContain("sl submit --api");
});
