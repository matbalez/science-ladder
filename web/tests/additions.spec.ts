import { test, expect } from "@playwright/test";
import { ADDITIONS_SOURCE } from "../lib/additions-reference";

// Frozen public challenge data; intercepted locally only to exercise UI before publication.
const version = "4094d7b3-c704-47d2-9eea-4ff792f7ff5d";
for (const width of [1280, 390]) {
  test(`addition challenge science and participation at ${width}px`, async ({
    page,
    request,
  }) => {
    if (process.env.LIVE_RELEASE_CHECK !== "1") {
      const fixture = {
        domain: "Mathematics & physics",
        status: "published",
        reviewStatus: "automated_pass",
        intakeStatus: "open",
        badges: [],
        reviews: [],
        deadline: "2027-09-08T00:00:00Z",
        economicMode: "none",
        verificationPolicy: "platform",
        createdAt: "2026-09-09T00:00:00Z",
        education: {
          frontier: "Test fixture for the 55-addition reference.",
          significance: "Test fixture: exact addition complexity.",
        },
        metric: {},
        manifest: {
          scientificQuestion: "Reduce additions in exact 3×3 multiplication.",
          impact: "Exact arithmetic complexity.",
          evidence: [],
          limitations: [],
          hardGates: [],
          submission: { allowedPaths: ["solver.py"], license: "MIT" },
        },
      };
      await page.route("**/v1/challenges/fewer-additions", (r) =>
        r.fulfill({
          json: {
            ...fixture,
            id: "test-additions",
            slug: "fewer-additions",
            title: "Fewer Additions",
            repository: "matbalez/science-ladder-fewer-additions",
            sourceCommit: ADDITIONS_SOURCE,
            versionId: version,
            summary:
              "Multiply 3×3 matrices with fewer than 55 additions, keeping 23 products.",
            metric: {
              ...fixture.metric,
              name: "additions",
              direction: "minimize",
              units: "additions",
              quantum: "1",
              baselineTicks: "55",
            },
            submissions: [],
            milestones: [],
            publicFrontier: null,
          },
        }),
      );
    }
    if (process.env.LIVE_RELEASE_CHECK === "1") {
      const res = await request.get("/v1/challenges/fewer-additions");
      expect(res.ok()).toBe(true);
      const c = await res.json();
      expect(c.status).toBe("published");
      expect(c.reviewStatus).toBe("automated_pass");
      expect(c.intakeStatus).toBe("open");
      expect(c.sourceCommit).toBe(ADDITIONS_SOURCE);
      expect(c.metric.baselineTicks).toBe("55");
      expect(c.education.significance).toContain("78 to 77");
      expect(c.lockDigest).toMatch(/^sha256:[a-f0-9]{64}$/);
    }
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/challenges/fewer-additions");
    await expect(
      page.getByRole("heading", { name: "Fewer Additions", exact: true }),
    ).toBeVisible();
    const visual = page.getByRole("figure", {
      name: "Addition sharing diagram",
    });
    await expect(visual).toBeVisible();
    if (width === 390)
      await expect(
        visual.getByText("13 + 14 + 28 = 55 additions", { exact: true }),
      ).toBeVisible();
    else await expect(visual.getByRole("img")).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page
      .getByRole("button", { name: "Participate", exact: true })
      .click();
    const prompt = await page.getByLabel(/agent instructions/i).inputValue();
    expect(prompt).toContain(ADDITIONS_SOURCE);
    expect(prompt).toContain("sl clone 'fewer-additions'");
    expect(prompt).toContain("sl run --baseline");
    expect(prompt).toContain("55 additions (13+14+28)");
    expect(prompt).toContain("may change the decomposition");
    expect(prompt).toContain("sl submit --model");
    await page.keyboard.press("Escape");
    await visual.screenshot({ path: `/tmp/additions-${width}.png` });
  });
}
