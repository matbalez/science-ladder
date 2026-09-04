import { test, expect, type Page } from "@playwright/test";
// Explicit test-only API fixtures. Production contains no fallback scientific data.
const session = {
  user: {
    id: "test-user",
    githubId: "123",
    login: "test-solver",
    role: "member",
    invited: true,
  },
  quotas: { remaining: 8, activeLimit: 2 },
  capabilities: { creation: true, submission: true, review: false },
  configuration: {
    githubAuth: true,
    scientificReview: true,
    officialRunner: true,
  },
};
const source = {
  url: "https://example.org/test-paper",
  title: "TEST ONLY primary source",
  identifier: "arXiv:TEST-ONLY",
  evidence: "An explicitly synthetic source used in this browser test.",
  location: "Section 4",
  accessedAt: "2026-09-04",
};
const manifest = {
  title: "TEST ONLY geometric construction",
  scientificQuestion: "Can a test-only construction improve?",
  impact: "This text is browser-test evidence, not a scientific claim.",
  evidence: [source],
  limitations: ["This fixture is not a scientific benchmark."],
  metric: {
    name: "Test energy",
    direction: "minimize",
    unit: "test units",
    quantum: "0.000001",
    baselineTicks: "900719925474099312345",
    minimumDeltaTicks: "10",
    toleranceTicks: "0",
  },
  hardGates: ["Every coordinate must be finite."],
  submission: {
    license: "MIT",
    allowedPaths: ["submission/"],
    maxBytes: 65536,
  },
  validator: { profile: "artifact-checker-v1" },
};
const challenge = {
  id: "test-challenge",
  slug: "test-only",
  title: manifest.title,
  summary: "A browser-test challenge fixture.",
  domain: "Test mathematics",
  status: "published",
  reviewStatus: "automated_pass",
  intakeStatus: "open",
  economicMode: "none",
  versionId: "test-version",
  repository: "test-owner/test-repo",
  sourceCommit: "a".repeat(40),
  createdAt: "2026-09-04T10:00:00Z",
  deadline: "2027-09-04T10:00:00Z",
  metric: { ...manifest.metric, units: "test units" },
  milestones: [
    {
      id: "tier-one",
      label: "First test threshold",
      thresholdTicks: "900719925474099312300",
    },
  ],
  badges: [],
  manifest,
  reviews: [],
  submissions: [],
};
async function base(page: Page) {
  await page.route("**/v1/me", (r) => r.fulfill({ json: session }));
}
test("public empty state has no invented challenges and mobile has no horizontal overflow", async ({
  page,
}) => {
  await base(page);
  await page.route("**/v1/challenges?*", (r) =>
    r.fulfill({ json: { challenges: [] } }),
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(page.getByText("The next frontier is waiting.")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Explore the Challenge Scout" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: "test-results/mobile-empty.png",
    fullPage: true,
  });
});
test("flat canonical manifest renders real evidence, gates, and exact score decimals", async ({
  page,
}) => {
  await base(page);
  await page.route("**/v1/challenges/test-only", (r) =>
    r.fulfill({ json: challenge }),
  );
  await page.goto("/challenges/test-only");
  await expect(
    page.getByRole("heading", { name: manifest.scientificQuestion }),
  ).toBeVisible();
  await expect(page.getByText(source.evidence)).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Read primary source" }),
  ).toHaveAttribute("href", source.url);
  await expect(
    page.getByRole("link", { name: "Explore the pulse and its echoes" }),
  ).toHaveCount(0);

  await expect(
    page.getByText("900,719,925,474,099.312345", { exact: false }).first(),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Evaluation contract" }).click();
  await expect(
    page.getByText("Every coordinate must be finite.", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("0.00001", { exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "Frontier & artifacts" }).click();
  await expect(
    page.getByText(
      "Verified submissions will trace the actual frontier here.",
      { exact: false },
    ),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/desktop-challenge.png",
    fullPage: true,
  });
});
test("candidate import validates YAML and preserves canonical prompt inputs", async ({
  page,
}) => {
  await base(page);
  await page.route("**/v1/prompts/challenge-scout/v1", (r) =>
    r.fulfill({
      json: {
        version: "1.0.0",
        prompt:
          "CANONICAL TEST PROMPT\nTopic: {{FIELD_OR_TOPIC}}\nQuestion: {{OPEN_QUESTION_OR_BLANK}}",
      },
    }),
  );
  await page.goto("/create");
  await page
    .getByLabel("Field or topic", { exact: true })
    .fill("Quantum geometry");
  await page.getByText("Preview the complete prompt").click();
  await expect(
    page.getByText("Topic: Quantum geometry", { exact: false }),
  ).toBeVisible();
  await page.getByLabel("Candidate YAML").fill("title: [");
  await page.getByRole("button", { name: "Validate candidate" }).click();
  await expect(page.locator("main").getByRole("alert")).toBeVisible();
});
test("submission independently fetches exact SHA before acceptance", async ({
  page,
}) => {
  await base(page);
  await page.route("**/v1/challenges/test-only", (r) =>
    r.fulfill({ json: challenge }),
  );
  let submitted: Record<string, unknown> = {};
  await page.route("**/v1/submission-intents", async (r) => {
    submitted = r.request().postDataJSON();
    expect(r.request().headers()["idempotency-key"]).toBeTruthy();
    await r.fulfill({
      status: 202,
      json: {
        id: "test-intent",
        versionId: "test-version",
        status: "ready",
        repository: "test-owner/solve",
        sourceCommit: "b".repeat(40),
        artifactDigest: "sha256:" + "1".repeat(64),
        findings: [],
        createdAt: "2026-09-04T11:00:00Z",
      },
    });
  });
  await page.route("**/v1/submission-intents/test-intent", (r) =>
    r.fulfill({
      json: {
        id: "test-intent",
        status: "ready",
        artifactDigest: "sha256:" + "1".repeat(64),
        findings: [],
      },
    }),
  );
  await page.route("**/v1/submission-intents/test-intent/accept", (r) =>
    r.fulfill({
      json: {
        submissionId: "test-submission",
        sequence: 1,
        receiptDigest: "sha256:" + "2".repeat(64),
        status: "accepted",
      },
    }),
  );
  await page.goto("/challenges/test-only");
  await page.getByRole("button", { name: "Submit a construction" }).click();
  await expect(
    page.getByText(
      "The best verified score is public even when its artifact remains private.",
      { exact: false },
    ),
  ).toBeVisible();
  await page
    .getByLabel("GitHub repository", { exact: true })
    .fill("test-owner/solve");
  await page
    .getByLabel("Exact pushed commit", { exact: false })
    .fill("b".repeat(40));
  await page
    .getByLabel("I agree that public-frontier artifacts", { exact: false })
    .check();
  await page
    .getByRole("button", { name: "Fetch & inspect exact commit" })
    .click();
  await page
    .getByRole("button", { name: "Accept & reserve validation" })
    .click();
  await expect(
    page.getByRole("link", { name: "Open submission receipt" }),
  ).toBeVisible();
  expect(submitted.ref).toBe("b".repeat(40));
  expect(submitted.license).toBe("MIT");
  expect(submitted).not.toHaveProperty("score");
});
test("completed preflight object remains readable and can lock the reviewed contract", async ({
  page,
}) => {
  await base(page);
  const preflight = {
    id: "test-preflight",
    versionId: "test-version",
    status: "queued",
    findings: [],
    reports: [],
  };
  const candidate = {
    id: "test-candidate",
    status: "ready",
    candidate: { title: "TEST ONLY candidate" },
    findings: [],
  };
  await page.addInitScript(
    (draft) =>
      sessionStorage.setItem(
        "science-ladder-creator-draft",
        JSON.stringify(draft),
      ),
    {
      candidate,
      preflight,
      creation: {
        id: "test-challenge",
        versionId: "test-version",
        slug: "test-only",
        status: "preflight_pending",
      },
      repository: "test-owner/test-repo",
      ref: "a".repeat(40),
    },
  );
  await page.route("**/v1/prompts/challenge-scout/v1", (r) =>
    r.fulfill({ json: { version: "1.0.0", prompt: "TEST ONLY prompt" } }),
  );
  await page.route("**/v1/candidates/test-candidate", (r) =>
    r.fulfill({ json: candidate }),
  );
  await page.route("**/v1/preflights/test-preflight", (r) =>
    r.fulfill({
      json: {
        ...preflight,
        status: "pass",
        reports: {
          passed: true,
          validatorDiskDigest: "sha256:" + "3".repeat(64),
          findings: ["TEST ONLY signed build evidence"],
        },
      },
    }),
  );
  await page.route("**/v1/challenge-versions/test-version/lock", (r) =>
    r.fulfill({
      json: { status: "locked", lockDigest: "sha256:" + "4".repeat(64) },
    }),
  );
  await page.goto("/create");
  await page
    .getByText("Report 1: checks and evidence", { exact: true })
    .click();
  await expect(
    page.getByText("TEST ONLY signed build evidence", { exact: false }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Lock the immutable contract" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Challenge contract locked" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Publish challenge" }),
  ).toBeEnabled();
});
test("API failure remains visible instead of showing invented empty data", async ({
  page,
}) => {
  await base(page);
  await page.route("**/v1/challenges?*", (r) =>
    r.fulfill({
      status: 503,
      json: {
        error: {
          code: "db_unavailable",
          message: "The test data service is temporarily unavailable.",
        },
      },
    }),
  );
  await page.goto("/");
  await expect(page.locator("main").getByRole("alert")).toContainText(
    "The test data service is temporarily unavailable.",
  );
  await expect(page.getByText("The next frontier is waiting.")).toHaveCount(0);
});

test("canonical artifact bundle previews real coordinate data without executing files", async ({
  page,
}) => {
  await base(page);
  const digest = "sha256:" + "8".repeat(64);
  const record = {
    id: "test-record",
    versionId: "test-version",
    sequence: 1,
    status: "finalized",
    outcome: "valid",
    scoreTicks: "900719925474099312290",
    artifactDigest: digest,
    public: true,
    attribution: { model: "TEST ONLY model" },
    createdAt: "2026-09-04T10:00:00Z",
    claims: [],
    runs: [],
  };
  await page.route("**/v1/challenges/test-only", (r) =>
    r.fulfill({
      json: {
        ...challenge,
        publicFrontier: {
          submissionId: record.id,
          scoreTicks: record.scoreTicks,
        },
        submissions: [record],
      },
    }),
  );
  await page.route("**/v1/artifacts/*", (r) =>
    r.fulfill({
      json: {
        tree: { kind: "ScienceLadderArtifactTree" },
        files: {
          "submission/points.json": Buffer.from(
            JSON.stringify({
              points: [
                [0, 0, 1],
                [1, 0, 0],
                [0, 1, 0],
              ],
            }),
          ).toString("base64"),
        },
      },
    }),
  );
  await page.goto("/challenges/test-only");
  await page.getByRole("tab", { name: "Frontier & artifacts" }).click();
  await expect(
    page.getByRole("img", {
      name: "submission/points.json: 3 artifact coordinates",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("slider", { name: "Rotate construction" }),
  ).toBeVisible();
});
test("audit panel distinguishes absent evidence from independently witnessed quorum", async ({
  page,
}) => {
  await base(page);
  await page.route("**/v1/audit/checkpoints?*", (r) =>
    r.fulfill({ json: { checkpoints: [], deploymentMode: "demonstration" } }),
  );
  await page.goto("/docs#trust");
  await expect(
    page.getByRole("heading", {
      name: "No signed checkpoints have been published.",
    }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "An empty record is not evidence of independent witness quorum.",
      { exact: false },
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Host reports witness quorum", { exact: true }),
  ).toHaveCount(0);
});

test("educational explorer link is secondary and bound to the exact registered science source", async ({
  page,
}) => {
  await base(page);
  let registered = true;
  await page.route("**/v1/challenges/quiet-echoes-labs512", (r) =>
    r.fulfill({
      json: {
        ...challenge,
        slug: "quiet-echoes-labs512",
        submissions: registered
          ? [
              {
                id: "test-completed",
                versionId: challenge.versionId,
                sequence: 1,
                status: "finalized",
                outcome: "valid",
                verificationStatus: "platform_verified",
                scoreTicks: "900719925474099312355",
                public: true,
                createdAt: challenge.createdAt,
                attribution: {},
                claims: [],
                runs: [],
              },
            ]
          : [],

        repository: "matbalez/science-ladder-quiet-echoes",
        sourceCommit: registered
          ? "f42f527e97563b1c068a1835732c6da44f21223f"
          : "b".repeat(40),
      },
    }),
  );
  await page.goto("/challenges/quiet-echoes-labs512");
  const link = page.getByRole("link", {
    name: "Explore the pulse and its echoes",
  });
  await expect(link).toHaveAttribute(
    "href",
    "/showcase/quiet-echoes/index.html",
  );
  const setup = page
    .locator(".content-section")
    .filter({
      has: page.getByRole("heading", { name: "Bring your own agent." }),
    })
    .locator(".code-block");
  await expect(setup).toContainText("python3 tools/reproduce.py --check");
  await expect(setup).toContainText("python3 -m unittest discover -s tests -v");
  await expect(setup).not.toContainText("sl challenge test");
  await expect(
    page.getByText("No verified improvement yet", { exact: true }),
  ).toBeVisible();
  await expect(
    page.locator(".challenge-header-actions .primary:visible"),
  ).toHaveCount(1);
  await link.click();
  await expect(
    page.getByRole("heading", { name: "Quiet Echoes." }),
  ).toBeVisible();
  await expect(page.locator("#pulse")).toBeVisible();
  registered = false;
  await page.goto("/challenges/quiet-echoes-labs512");
  await expect(
    page.getByRole("heading", { name: manifest.scientificQuestion }),
  ).toBeVisible();
  await expect(link).toHaveCount(0);
  await expect(
    page.getByText("Awaiting validation", { exact: true }),
  ).toBeVisible();
});

// Explicit test-only researcher metadata; no fixture is shipped to public pages.
const researcherFixture = {
  name: "TEST ONLY researcher",
  profileUrl: "https://example.org/researcher",
  connection:
    "Authored the test-only paper that motivates this synthetic browser fixture.",
  workTitle: "TEST ONLY research paper",
  workUrl: "https://example.org/research-paper",
};
const researcherContextFixture = {
  id: "test-context",
  versionId: challenge.versionId,
  researchers: [researcherFixture],
  updatedAt: "2026-09-04T22:00:00Z",
  updatedBy: { githubId: "123", login: "test-editor" },
  reason:
    "This browser-test context records a synthetic publication connection.",
};
const researcherNotice =
  "Listed for their research, not as challenge sponsors or endorsers. Their interest has not been confirmed.";

test("public researcher context is optional, informational, and contains only safe secondary links", async ({
  page,
}) => {
  await base(page);
  let context: typeof researcherContextFixture | null = null;
  await page.route("**/v1/challenges/test-only", (r) =>
    r.fulfill({ json: { ...challenge, researcherContext: context } }),
  );
  await page.goto("/challenges/test-only");
  await expect(
    page.getByRole("heading", { name: manifest.scientificQuestion }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Researchers to know" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Add researcher context" }),
  ).toHaveCount(0);
  context = researcherContextFixture;
  await page.reload();
  const section = page.locator(".researchers-section");
  await expect(
    section.getByRole("heading", { name: "Researchers to know" }),
  ).toBeVisible();
  await expect(
    section.getByText(researcherNotice, { exact: true }),
  ).toBeVisible();
  await expect(
    section.getByText(
      "Their published work connects to this question. The community could invite them to follow our progress.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(
    section.getByRole("link", { name: "Research profile" }),
  ).toHaveAttribute("href", researcherFixture.profileUrl);
  await expect(
    section.getByRole("link", { name: researcherFixture.workTitle }),
  ).toHaveAttribute("href", researcherFixture.workUrl);
  await expect(section.locator("button,input,textarea,img")).toHaveCount(0);
  await expect(
    page.locator(".challenge-header-actions .primary:visible"),
  ).toHaveCount(1);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: "test-results/researchers-mobile.png",
    fullPage: true,
  });
  context = {
    ...researcherContextFixture,
    researchers: [
      {
        ...researcherFixture,
        name: "<img src=x onerror=alert(1)>",
        profileUrl: "javascript:alert(1)",
        workUrl: "data:text/html,unsafe",
      },
    ],
  };
  await page.reload();
  await expect(
    section.getByText("<img src=x onerror=alert(1)>", { exact: true }),
  ).toBeVisible();
  await expect(section.getByRole("link")).toHaveCount(0);
  await expect(section.locator("img")).toHaveCount(0);
});

test("authorized editor saves researcher fields and public reason, including explicit empty-list clearing", async ({
  page,
}) => {
  const editorSession = {
    ...session,
    user: { ...session.user, role: "editor" },
    capabilities: { ...session.capabilities, review: true },
  };
  await page.route("**/v1/me", (r) => r.fulfill({ json: editorSession }));
  await page.route("**/v1/editor/queue", (r) =>
    r.fulfill({ json: { flags: [], reviews: [], candidates: [] } }),
  );
  await page.route("**/v1/challenges/test-only", (r) =>
    r.fulfill({
      json: { ...challenge, researcherContext: researcherContextFixture },
    }),
  );
  const writes: Record<string, unknown>[] = [];
  await page.route(
    "**/v1/editor/challenge-versions/test-version/researchers",
    async (r) => {
      expect(r.request().method()).toBe("POST");
      expect(r.request().headers()["idempotency-key"]).toBeTruthy();
      const body = r.request().postDataJSON();
      writes.push(body);
      await r.fulfill({
        status: 201,
        json: {
          ...researcherContextFixture,
          id: `test-edition-${writes.length}`,
          researchers: body.researchers,
          reason: body.reason,
        },
      });
    },
  );
  await page.goto("/challenges/test-only");
  await page
    .getByRole("link", { name: "Edit researcher context", exact: true })
    .click();
  await expect(page).toHaveURL(
    /\/review\?challenge=test-only&version=test-version#researcher-editor/,
  );
  const editor = page.locator("#researcher-editor");
  await expect(
    editor.getByLabel("Researcher 1 name", { exact: true }),
  ).toHaveValue(researcherFixture.name);
  await editor
    .getByLabel("Researcher 1 connection to the question", { exact: true })
    .fill("Updated test-only connection to the exact scientific question.");
  await editor
    .getByLabel("Reason for this change (public)", { exact: true })
    .fill("Correct the publication connection for this test-only researcher.");
  await editor
    .getByLabel("Researcher 1 profile URL", { exact: true })
    .fill("http://example.org/researcher");
  await editor.getByRole("button", { name: "Save researcher context" }).click();
  await expect(editor.getByRole("alert")).toContainText("public HTTPS");
  expect(writes).toHaveLength(0);
  await editor
    .getByLabel("Researcher 1 profile URL", { exact: true })
    .fill(researcherFixture.profileUrl);
  await editor.getByRole("button", { name: "Save researcher context" }).click();
  await expect(editor.getByRole("status")).toContainText(
    "Researcher context saved",
  );
  expect(writes[0]).toEqual({
    researchers: [
      {
        ...researcherFixture,
        connection:
          "Updated test-only connection to the exact scientific question.",
      },
    ],
    reason: "Correct the publication connection for this test-only researcher.",
  });
  await editor
    .getByRole("button", { name: "Remove researcher 1", exact: true })
    .click();
  await expect(
    editor.getByLabel("Researcher 1 name", { exact: true }),
  ).toHaveCount(0);
  await editor
    .getByLabel("Reason for this change (public)", { exact: true })
    .fill("Remove this test-only context while preserving its public history.");
  await editor.getByRole("button", { name: "Save researcher context" }).click();
  await expect(editor.getByRole("status")).toContainText(
    "Researcher context saved",
  );
  expect(writes[1]).toEqual({
    researchers: [],
    reason:
      "Remove this test-only context while preserving its public history.",
  });
  for (let i = 0; i < 6; i++)
    await editor
      .getByRole("button", { name: "Add researcher", exact: true })
      .click();
  await expect(
    editor.getByRole("button", { name: "Add researcher", exact: true }),
  ).toBeDisabled();
  await expect(
    editor.getByLabel("Researcher 6 name", { exact: true }),
  ).toBeVisible();
});

test("researcher editor is hidden without editor capability and refuses a mismatched selected version", async ({
  page,
}) => {
  await base(page);
  await page.goto("/review?challenge=test-only&version=other-version");
  await expect(
    page.getByRole("heading", { name: "Editor access required." }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Researcher 1 name", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Save researcher context" }),
  ).toHaveCount(0);
  await page.route("**/v1/me", (r) =>
    r.fulfill({
      json: {
        ...session,
        capabilities: { ...session.capabilities, review: true },
      },
    }),
  );
  await page.route("**/v1/editor/queue", (r) =>
    r.fulfill({ json: { flags: [], reviews: [], candidates: [] } }),
  );
  await page.route("**/v1/challenges/test-only", (r) =>
    r.fulfill({
      json: { ...challenge, researcherContext: researcherContextFixture },
    }),
  );
  await page.reload();
  await expect(
    page.locator("#researcher-editor").getByRole("alert"),
  ).toContainText("another challenge version");
  await expect(
    page.getByRole("button", { name: "Save researcher context" }),
  ).toHaveCount(0);
});

test("standalone explorer consumes optional public researcher context without hardcoded people", async ({
  page,
  request,
}) => {
  const response = await request.get("/showcase/quiet-echoes/index.html");
  expect(response.ok()).toBeTruthy();
  const html = await response.text();
  const origin = "https://science-ladder.fly.dev";
  let populated = false;
  await page.route(`${origin}/showcase/quiet-echoes/index.html`, (r) =>
    r.fulfill({ status: 200, contentType: "text/html", body: html }),
  );
  await page.route(`${origin}/v1/challenges/quiet-echoes-labs512`, (r) =>
    r.fulfill({
      json: {
        ...challenge,
        slug: "quiet-echoes-labs512",
        versionId: "56ddbf39-2b67-4172-9a9d-e3c78e44c7cf",
        sourceCommit: "f42f527e97563b1c068a1835732c6da44f21223f",
        researcherContext: populated ? researcherContextFixture : null,
      },
    }),
  );
  await page.goto(`${origin}/showcase/quiet-echoes/index.html`);
  await expect(page.locator("#researcher-context")).not.toBeVisible();
  populated = true;
  await page.reload();
  const section = page.locator("#researcher-context");
  await expect(
    section.getByRole("heading", { name: "Researchers to know" }),
  ).toBeVisible();
  await expect(
    section.getByText(researcherNotice, { exact: true }),
  ).toBeVisible();
  await expect(
    section.getByText(
      "Their published work connects to this question. The community could invite them to follow our progress.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(
    section.getByRole("link", { name: `Profile of ${researcherFixture.name}` }),
  ).toHaveAttribute("href", researcherFixture.profileUrl);
  await expect(section.locator("button,input,textarea,img")).toHaveCount(0);
  await expect(page.locator(".participate-primary:visible")).toHaveCount(1);
  await expect(page.locator("#energy")).toHaveText("17,996");
});
