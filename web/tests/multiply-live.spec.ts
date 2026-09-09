import { test, expect } from "@playwright/test";

test("sunset matrix rank challenge points to its replacement", async ({
  page,
  request,
}) => {
  test.skip(
    process.env.LIVE_RELEASE_CHECK !== "1",
    "Explicit production check",
  );
  const response = await request.get("/v1/challenges/one-less-multiply");
  expect(response.status()).toBe(410);
  expect((await response.json()).error.code).toBe("challenge_withdrawn");
  await page.goto("/challenges/one-less-multiply");
  await expect(
    page.getByRole("link", { name: "Try Fewer Additions" }),
  ).toHaveAttribute("href", "/challenges/fewer-additions");
  await expect(
    page.getByRole("button", { name: "Participate", exact: true }),
  ).toHaveCount(0);
});
