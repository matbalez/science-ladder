import { test, expect } from "@playwright/test";
import { MULTIPLY_SOURCE } from "../lib/multiply-reference";
const challenge = {
  id: "browser-fixture",
  slug: "one-less-multiply",
  title: "One Less Multiply",
  summary: "Browser test fixture",
  status: "published",
  reviewStatus: "automated_pass",
  intakeStatus: "open",
  economicMode: "none",
  versionId: "e588cddb-c861-4879-8857-27fd478101cb",
  repository: "matbalez/science-ladder-one-less-multiply",
  sourceCommit: MULTIPLY_SOURCE,
  metric: {
    name: "bilinear-products",
    direction: "minimize",
    quantum: "1",
    baselineTicks: "23",
  },
  milestones: [],
  badges: [],
  reviews: [],
  submissions: [],
  education: { frontier: "Test frontier.", significance: "Test significance." },
  manifest: {
    fixtures: [{ name: "baseline", path: "submission" }],
    validator: { profile: "native-evaluator-v2" },
    submission: { allowedPaths: ["solver.py"], license: "MIT" },
    evaluation: {
      version: "science-ladder/evaluation/v1",
      mode: "program",
      measurements: [],
    },
  },
};

function stream(answer: string, done = true) {
  return (
    `data: ${JSON.stringify({ type: "delta", text: answer })}\n\n` +
    (done ? 'data: {"type":"done"}\n\n' : "")
  );
}
test.beforeEach(async ({ page }) => {
  await page.route("**/v1/me", (r) =>
    r.fulfill({ json: { user: null, capabilities: {}, configuration: {} } }),
  );
  await page.route("**/v1/challenges/one-less-multiply", (r) =>
    r.fulfill({ json: challenge }),
  );
});
for (const width of [1440, 390])
  test(`Learning guide uses current visualization and remembers follow-ups at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    const requests: any[] = [];
    await page.route("**/v1/challenges/one-less-multiply/ask", (r) => {
      requests.push(r.request().postDataJSON());
      return r.fulfill({
        contentType: "text/event-stream",
        body: stream(
          requests.length === 1
            ? "**Product 23** multiplies 1 by 6. [Source](https://example.org/paper)"
            : "All 23 contributions add up to the full matrix.",
        ),
      });
    });
    await page.goto("/challenges/one-less-multiply");
    await page.getByRole("slider", { name: "Reference product" }).fill("22");
    const guide = page.getByRole("region", {
      name: "Ask about this challenge",
    });
    await guide
      .getByRole("button", {
        name: "What am I seeing in the visualization?",
        exact: true,
      })
      .click();
    await expect(guide.getByText("Product 23", { exact: true })).toBeVisible();
    await expect(
      guide.getByRole("button", { name: "Stop answer" }),
    ).toHaveCount(0);
    expect(requests[0].versionId).toBe(challenge.versionId);
    expect(
      requests[0].view.visualizations["matrix multiplication"].selectedProduct,
    ).toBe(23);
    expect(
      requests[0].view.visualizations["matrix multiplication"].product,
    ).toBe(6);
    await guide
      .getByRole("textbox")
      .fill("How does that give the whole matrix?");
    await guide.getByRole("textbox").press("Enter");
    await expect(
      guide.getByText("All 23 contributions add up to the full matrix."),
    ).toBeVisible();
    expect(requests[1].messages.map((m: any) => m.role)).toEqual([
      "user",
      "assistant",
      "user",
    ]);
    expect(requests[1].messages[1].content).toContain("Product 23");
    await expect(
      guide.getByRole("button", { name: "Stop answer" }),
    ).toHaveCount(0);
    await page.reload();
    await expect(
      guide.getByText("All 23 contributions add up to the full matrix."),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await guide.screenshot({
      path: `/tmp/science-ladder-learning-${width}.png`,
    });
    await guide.getByRole("button", { name: "Clear conversation" }).click();
    await expect(guide.getByRole("log")).toHaveCount(0);
    await page.reload();
    await expect(guide.getByRole("log")).toHaveCount(0);
  });
test("Interrupted answers are labeled and excluded from subsequent history", async ({
  page,
}) => {
  const requests: any[] = [];
  await page.route("**/v1/challenges/one-less-multiply/ask", (r) => {
    requests.push(r.request().postDataJSON());
    return r.fulfill({
      contentType: "text/event-stream",
      body: stream(
        requests.length === 1
          ? "An unfinished explanation"
          : "A complete explanation.",
        requests.length !== 1,
      ),
    });
  });
  await page.goto("/challenges/one-less-multiply");
  const guide = page.getByRole("region", { name: "Ask about this challenge" });
  await guide
    .getByRole("button", { name: "Explain this challenge simply", exact: true })
    .click();
  await expect(guide.getByRole("alert")).toContainText("interrupted");
  await expect(
    guide.getByText("Partial answer", { exact: true }),
  ).toBeVisible();
  await expect(guide.getByRole("textbox")).toHaveValue(
    "Explain this challenge simply",
  );
  await guide
    .getByRole("button", { name: "Ask question", exact: true })
    .click();
  await expect(guide.getByText("A complete explanation.")).toBeVisible();
  expect(requests[1].messages).toHaveLength(1);
});
test("Public limit errors preserve the question for retry", async ({
  page,
}) => {
  await page.route("**/v1/challenges/one-less-multiply/ask", (r) =>
    r.fulfill({
      status: 429,
      json: { error: { message: "Please try again later." } },
    }),
  );
  await page.goto("/challenges/one-less-multiply");
  const guide = page.getByRole("region", { name: "Ask about this challenge" });
  await guide
    .getByRole("button", {
      name: "Why would an improvement matter?",
      exact: true,
    })
    .click();
  await expect(guide.getByRole("alert")).toContainText(
    "Please try again later.",
  );
  await expect(guide.getByRole("textbox")).toHaveValue(
    "Why would an improvement matter?",
  );
});
