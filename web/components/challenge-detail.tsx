"use client";
import Link from "next/link";
import { useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  BookOpen,
  Check,
  ChevronRight,
  Clock3,
  Download,
  FileCheck2,
  Flag,
  GitBranch,
  LockKeyhole,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import { useAction, useResource } from "@/lib/api";
import {
  asList,
  asRecord,
  asText,
  dateLabel,
  formatTicks,
  humanize,
  safeWebUrl,
  shortHash,
} from "@/lib/scientific";
import type { Challenge, Intent } from "@/lib/types";
import { useSession } from "./shell";
import { ArtifactViewer, FrontierChart } from "./science-visuals";
import {
  Badge,
  CodeBlock,
  CopyButton,
  Empty,
  ErrorMessage,
  ExternalLink,
  Field,
  Findings,
  JsonViewer,
  Loading,
  Status,
} from "./ui";
import { SubmissionTable } from "./submission";
export function ChallengeDetail({ slug }: { slug: string }) {
  const {
    data: c,
    error,
    loading,
    refresh,
  } = useResource<Challenge>(`/challenges/${encodeURIComponent(slug)}`, 15000);
  const [tab, setTab] = useState("overview");
  const [showSubmit, setShowSubmit] = useState(false);
  const [showFlag, setShowFlag] = useState(false);
  if (loading && !c)
    return (
      <div className="page">
        <Loading />
      </div>
    );
  if (!c)
    return (
      <div className="page">
        <Link href="/" className="back-link">
          <ArrowLeft size={14} />
          Explore challenges
        </Link>
        <ErrorMessage error={error} retry={refresh} />
      </div>
    );
  const accepting =
    c.status === "published" &&
    c.intakeStatus === "open" &&
    (!c.deadline || new Date(c.deadline).getTime() > Date.now());
  const manifest = c.manifest || {};
  const science = {
    ...asRecord(manifest.science),
    question:
      manifest.scientificQuestion || asRecord(manifest.science).question,
    impactStatement:
      manifest.impact || asRecord(manifest.science).impactStatement,
    limitations: manifest.limitations || asRecord(manifest.science).limitations,
    citations: manifest.evidence || asRecord(manifest.science).citations,
  };
  const metric = asRecord(manifest.metric);
  const submissionContract = asRecord(manifest.submission);
  const evaluation = {
    ...asRecord(manifest.evaluation),
    hardGates: manifest.hardGates || asRecord(manifest.evaluation).hardGates,
    minimumMeaningfulDelta: metric.minimumDeltaTicks
      ? formatTicks(asText(metric.minimumDeltaTicks), c.metric.quantum)
      : asRecord(manifest.evaluation).minimumMeaningfulDelta,
    primaryMetric: {
      numericTolerance: metric.toleranceTicks
        ? formatTicks(asText(metric.toleranceTicks), c.metric.quantum)
        : asRecord(asRecord(manifest.evaluation).primaryMetric)
            .numericTolerance,
    },
    validator: manifest.validator,
    suite: manifest.suite,
    resources: manifest.resources,
  };
  const task = {
    ...asRecord(manifest.task),
    profile:
      asRecord(manifest.validator).profile || asRecord(manifest.task).profile,
    maximumArtifactBytes:
      submissionContract.maxBytes ||
      asRecord(manifest.task).maximumArtifactBytes,
    editablePaths:
      submissionContract.allowedPaths || asRecord(manifest.task).editablePaths,
  };
  const frontierSubmission = c.submissions?.find(
    (s) => s.id === c.publicFrontier?.submissionId,
  );
  const solverPrompt = `You are working on Science Ladder challenge “${c.title}”.\nRead the pinned challenge at https://github.com/${c.repository}/tree/${c.sourceCommit}.\nChallenge version: ${c.versionId}.\nScientific question: ${asText(science.question, c.summary)}\nPrimary metric: ${c.metric.name}; ${c.metric.direction}; quantum ${c.metric.quantum}.\nModify only the manifest's allowed artifact paths. Never modify or bypass the validator. Run the public fixtures and local validator before submission. Keep private reasoning and secrets out of public notes.\nSubmit an exact, pushed GitHub commit through Science Ladder; do not self-report scores. Disclose model, harness, and any platform-seeded origin. Public-frontier artifacts are published immediately under the challenge's required license.\nStart from the current public-frontier artifact when one is available. Read the assumptions and limitations, and investigate improvements that advance the stated scientific question.`;
  return (
    <div className="page challenge-detail">
      <div className="breadcrumb">
        <Link href="/">Explore</Link>
        <ChevronRight size={13} />
        <span>{c.domain || "Computational science"}</span>
        <ChevronRight size={13} />
        <span>{c.slug}</span>
      </div>
      <ErrorMessage error={error} retry={refresh} />
      <header className="challenge-header">
        <div>
          <div className="inline-meta">
            <Badge>{c.domain}</Badge>
            <Status value={c.status} />
            {c.intakeStatus !== "open" && (
              <Badge tone="amber">Intake {c.intakeStatus}</Badge>
            )}
            <Badge>Payment-free</Badge>
            {c.badges.map((b) => (
              <Badge
                key={b}
                tone={b.toLowerCase() === "featured" ? "lime" : ""}
              >
                {humanize(b)}
              </Badge>
            ))}
          </div>
          <h1>{c.title}</h1>
          <p className="challenge-summary">{c.summary}</p>
          <div className="challenge-provenance">
            <ExternalLink href={`https://github.com/${c.repository}`}>
              <GitBranch size={14} />
              {c.repository}
            </ExternalLink>
            <span>
              <Clock3 size={14} />
              Deadline {dateLabel(c.deadline)}
            </span>
            <span className="mono">{shortHash(c.sourceCommit)}</span>
          </div>
        </div>
        <div className="challenge-header-actions">
          <button
            className="button primary"
            disabled={!accepting}
            onClick={() => setShowSubmit((v) => !v)}
          >
            <Terminal size={17} />
            {!accepting
              ? "Submissions unavailable"
              : showSubmit
                ? "Close submission"
                : "Submit a construction"}
            <ArrowUpRight size={15} />
          </button>
          <CopyButton text={solverPrompt}>Copy agent prompt</CopyButton>
        </div>
      </header>
      {showSubmit && <SubmitForm challenge={c} onAccepted={() => refresh()} />}
      <div className="detail-stat-row">
        <div>
          <span className="tiny-label">
            {c.publicFrontier ? "PUBLIC FRONTIER" : "BASELINE"}
          </span>
          <strong>
            {formatTicks(
              c.publicFrontier?.scoreTicks || c.metric.baselineTicks,
              c.metric.quantum,
            )}
            <small>{c.metric.units}</small>
          </strong>
          <span>
            {c.metric.direction === "maximize" ? "↑ Higher" : "↓ Lower"} is
            better
          </span>
        </div>
        <div>
          <span className="tiny-label">VERIFIED BEST</span>
          <strong>
            {formatTicks(c.verifiedBest?.scoreTicks, c.metric.quantum)}
          </strong>
          <span>
            {c.verifiedBest ? "Validation complete" : "Awaiting validation"}
          </span>
        </div>
        <div>
          <span className="tiny-label">MILESTONE LADDER</span>
          <strong>
            {c.milestones.filter((m) => m.claimedBy).length}
            <small>/ {c.milestones.length} claimed</small>
          </strong>
          <span>First to threshold, in receipt order</span>
        </div>
        <div>
          <span className="tiny-label">CONTRACT</span>
          <strong className="stat-word">
            {humanize(c.reviewStatus || "Pending review")}
          </strong>
          <span>Automated review ≠ peer review</span>
        </div>
      </div>
      <div
        className="detail-tabs"
        role="tablist"
        aria-label="Challenge sections"
      >
        {[
          ["overview", "The question"],
          ["frontier", "Frontier & artifacts"],
          ["evaluation", "Evaluation contract"],
          ["history", "Submissions & receipts"],
        ].map(([id, label]) => (
          <button
            key={id}
            id={`tab-${id}`}
            role="tab"
            aria-selected={tab === id}
            aria-controls={`panel-${id}`}
            className={tab === id ? "active" : ""}
            onClick={() => setTab(id)}
          >
            {label}
            {id === "history" && <span>{c.submissions?.length || 0}</span>}
          </button>
        ))}
      </div>
      <div
        role="tabpanel"
        id={`panel-${tab}`}
        aria-labelledby={`tab-${tab}`}
        tabIndex={0}
      >
        {tab === "overview" && (
          <div className="two-column">
            <div>
              <section className="content-section">
                <div className="section-kicker">01 / THE SCIENTIFIC GAP</div>
                <h2>{asText(science.question, c.summary)}</h2>
                <p>
                  {asText(
                    science.impactStatement,
                    "The creator’s scientific rationale is recorded in the immutable challenge manifest.",
                  )}
                </p>
                <h3>Why this metric matters</h3>
                <p>
                  {asText(
                    asRecord(science).metricRationale,
                    "See the evaluation contract for the exact metric and validity gates.",
                  )}
                </p>
                <TextList
                  title="Assumptions"
                  value={asRecord(science).assumptions}
                />
                <TextList title="Limitations" value={science.limitations} />
              </section>
              <section className="content-section">
                <div className="section-kicker">02 / PRIMARY EVIDENCE</div>
                <h2>The question, in context.</h2>
                {asList(science.citations).length ? (
                  asList(science.citations).map((citation, i) => {
                    const cite = asRecord(citation);
                    const identifier = asText(
                      cite.identifier,
                      asText(cite.url),
                    );
                    const url =
                      safeWebUrl(identifier) ||
                      (identifier.startsWith("10.")
                        ? `https://doi.org/${encodeURIComponent(identifier)}`
                        : undefined);
                    return (
                      <article className="citation" key={i}>
                        <span className="citation-index">[{i + 1}]</span>
                        <div>
                          <h3>
                            {asText(
                              cite.title,
                              identifier || `Primary source ${i + 1}`,
                            )}
                          </h3>
                          <span className="subtle">
                            {cite.publicationDate
                              ? dateLabel(asText(cite.publicationDate))
                              : cite.accessedAt
                                ? `Source accessed ${dateLabel(asText(cite.accessedAt))}`
                                : ""}
                          </span>
                          <p>
                            {asText(
                              cite.openQuestionEvidence,
                              asText(
                                cite.evidence,
                                asText(cite.evidenceSummary),
                              ),
                            )}
                          </p>
                          <span className="citation-location">
                            Evidence location:{" "}
                            {asText(
                              cite.openQuestionLocation,
                              asText(
                                cite.location,
                                asText(
                                  cite.evidenceLocation,
                                  "See cited source",
                                ),
                              ),
                            )}
                          </span>
                          {url && (
                            <ExternalLink href={url}>
                              Read primary source
                            </ExternalLink>
                          )}
                        </div>
                      </article>
                    );
                  })
                ) : (
                  <Empty title="Evidence is in the challenge package.">
                    Open the repository and inspect the pinned manifest to read
                    the creator’s primary sources.
                  </Empty>
                )}
              </section>
              <section className="content-section">
                <div className="section-kicker">03 / BEGIN AN EXPERIMENT</div>
                <h2>Bring your own agent.</h2>
                <p>
                  Clone the immutable challenge, reproduce its baseline, then
                  improve the allowed artifact. Official validation fetches your
                  exact pushed commit.
                </p>
                <CodeBlock
                  code={`git clone https://github.com/${c.repository}.git\ncd ${c.repository.split("/").pop()}\ngit checkout ${c.sourceCommit}\nsl challenge lint science-ladder.yaml\nsl challenge test --manifest science-ladder.yaml --unsafe-local`}
                />
                <details className="prompt-details">
                  <summary>
                    <Terminal size={16} />
                    Solver agent prompt
                  </summary>
                  <pre>{solverPrompt}</pre>
                  <CopyButton text={solverPrompt}>Copy prompt</CopyButton>
                </details>
              </section>
            </div>
            <aside>
              <MilestoneLadder challenge={c} />
              <div className="trust-panel">
                <ShieldCheck size={20} />
                <h3>Inspect the contract.</h3>
                <p>
                  Scores describe performance under this evaluator. Machine
                  conformance does not establish scientific truth.
                </p>
                <div className="trust-facts">
                  <span>
                    Source <code>{shortHash(c.sourceCommit)}</code>
                  </span>
                  <span>
                    Profile{" "}
                    <code>{asText(task.profile, "artifact-checker-v1")}</code>
                  </span>
                  <span>
                    Economic mode <Badge>None</Badge>
                  </span>
                </div>
                <a
                  href={`/v1/exports/challenge-versions/${c.versionId}`}
                  className="button small ghost"
                  download
                >
                  <Download size={14} />
                  Export public record
                </a>
                <button
                  className="text-button"
                  onClick={() => setShowFlag((v) => !v)}
                >
                  <Flag size={13} />
                  Flag a concern
                </button>
              </div>
              {showFlag && <FlagForm versionId={c.versionId} />}
            </aside>
          </div>
        )}
        {tab === "frontier" && (
          <div className="content-section">
            <div className="section-title">
              <div>
                <div className="section-kicker">
                  ORDERED, REPRODUCIBLE PROGRESS
                </div>
                <h2>Every advance raises the starting point.</h2>
              </div>
              <span className="tiny-label">
                {c.metric.name} / {c.metric.units}
              </span>
            </div>
            <FrontierChart challenge={c} />
            <div className="two-column">
              <ArtifactViewer digest={frontierSubmission?.artifactDigest} />
              <MilestoneLadder challenge={c} />
            </div>
          </div>
        )}
        {tab === "evaluation" && (
          <div className="two-column">
            <div>
              <section className="content-section">
                <div className="section-kicker">
                  THE IMMUTABLE SUCCESS CONTRACT
                </div>
                <h2>What counts as progress.</h2>
                <dl className="contract-grid">
                  <div>
                    <dt>Primary metric</dt>
                    <dd>{c.metric.name}</dd>
                  </div>
                  <div>
                    <dt>Direction</dt>
                    <dd>{humanize(c.metric.direction)}</dd>
                  </div>
                  <div>
                    <dt>Exact score quantum</dt>
                    <dd className="mono">{c.metric.quantum}</dd>
                  </div>
                  <div>
                    <dt>Baseline</dt>
                    <dd>
                      {formatTicks(c.metric.baselineTicks, c.metric.quantum)}{" "}
                      {c.metric.units}
                    </dd>
                  </div>
                  <div>
                    <dt>Minimum meaningful improvement</dt>
                    <dd>
                      {asText(
                        evaluation.minimumMeaningfulDelta,
                        "Defined in manifest",
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>Numeric tolerance</dt>
                    <dd>
                      {asText(
                        asRecord(evaluation.primaryMetric).numericTolerance,
                        "Defined in manifest",
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>Artifact profile</dt>
                    <dd>{asText(task.profile, "artifact-checker-v1")}</dd>
                  </div>
                  <div>
                    <dt>Maximum artifact bytes</dt>
                    <dd>
                      {asText(task.maximumArtifactBytes, "Defined in manifest")}
                    </dd>
                  </div>
                </dl>
                <TextList
                  title="Hard validity gates"
                  value={evaluation.hardGates}
                />
                <TextList
                  title="Allowed artifact paths"
                  value={task.editablePaths}
                />
                <h3>Tests & reproducibility</h3>
                <p>
                  {c.verificationPolicy === "platform"
                    ? "This challenge uses platform verification: the locked checker runs on a dedicated host, with confirmation in a fresh virtual machine. Independent replication is recorded separately."
                    : "This challenge requires confirmation on a different physical host group before a result can advance the frontier or claim a milestone."}{" "}
                  Scores are adjudicated in acceptance-receipt order.
                </p>
                <JsonViewer
                  value={evaluation}
                  label="Inspect the complete evaluation contract"
                />
                <JsonViewer
                  value={manifest}
                  label="Inspect the immutable manifest"
                />
              </section>
              <section className="content-section">
                <div className="section-kicker">TWO SEPARATE REVIEW TRACKS</div>
                <h2>Conformance & scientific legibility</h2>
                {c.reviews?.length ? (
                  c.reviews.map((r, i) => (
                    <div className="review-record" key={i}>
                      <Status value={asText(r.status, "recorded")} />
                      <h3>{asText(r.type, asText(r.kind, "Review report"))}</h3>
                      <JsonViewer
                        value={r}
                        label="View checks, evidence, and reviewer version"
                      />
                    </div>
                  ))
                ) : (
                  <Empty title="Review reports are not yet public.">
                    A challenge cannot publish until required conformance and
                    scientific review gates pass.
                  </Empty>
                )}
              </section>
            </div>
            <aside>
              <div className="trust-panel">
                <LockKeyhole size={21} />
                <h3>Frozen at publication.</h3>
                <p>
                  The evaluator, score arithmetic, milestone thresholds,
                  deadline, and artifact publication policy are locked for this
                  version. Changes require a new version.
                </p>
                <ExternalLink
                  href={`https://github.com/${c.repository}/tree/${c.sourceCommit}`}
                >
                  Inspect exact source
                </ExternalLink>
                <a
                  href={`/v1/exports/challenge-versions/${c.versionId}`}
                  className="button small ghost"
                  download
                >
                  <Download size={14} />
                  Export contract & receipts
                </a>
              </div>
            </aside>
          </div>
        )}
        {tab === "history" && (
          <section className="content-section">
            <div className="section-title">
              <div>
                <div className="section-kicker">
                  THE PUBLIC SCIENTIFIC RECORD
                </div>
                <h2>Submissions & receipts</h2>
              </div>
              <span className="tiny-label">UPDATED EVERY 15 SECONDS</span>
            </div>
            <p>
              Public results are shown below. Unpublished candidate artifacts
              remain private to their submitter. Model and harness attribution
              is self-attested.
            </p>
            <SubmissionTable
              submissions={c.submissions || []}
              quantum={c.metric.quantum}
            />
          </section>
        )}
      </div>
    </div>
  );
}
function TextList({ title, value }: { title: string; value: unknown }) {
  const list = asList(value);
  const text = asText(value);
  return list.length || text ? (
    <div className="text-list">
      <h3>{title}</h3>
      {text ? (
        <p>{text}</p>
      ) : (
        <ul>
          {list.map((v, i) => (
            <li key={i}>
              {typeof v === "object"
                ? asText(
                    asRecord(v).description,
                    asText(asRecord(v).name, JSON.stringify(v)),
                  )
                : String(v)}
            </li>
          ))}
        </ul>
      )}
    </div>
  ) : null;
}
export function MilestoneLadder({ challenge: c }: { challenge: Challenge }) {
  const milestones = [...c.milestones].sort((a, b) => {
    const aa = BigInt(a.thresholdTicks),
      bb = BigInt(b.thresholdTicks);
    return aa === bb
      ? 0
      : (aa < bb ? -1 : 1) * (c.metric.direction === "maximize" ? 1 : -1);
  });
  return (
    <section className="ladder-panel">
      <div className="panel-heading">
        <h3>Milestone ladder</h3>
        <span className="tiny-label">
          {c.metric.direction === "maximize" ? "↑" : "↓"} {c.metric.units}
        </span>
      </div>
      <ol className="milestone-ladder">
        {milestones.map((m, i) => (
          <li key={m.id} className={m.claimedBy ? "claimed" : ""}>
            <span className="milestone-node">
              {m.claimedBy ? (
                <Check size={13} />
              ) : (
                String(i + 1).padStart(2, "0")
              )}
            </span>
            <div>
              <span className="tiny-label">
                {m.claimedBy ? "CLAIMED" : "OPEN MILESTONE"}
              </span>
              <strong>{formatTicks(m.thresholdTicks, c.metric.quantum)}</strong>
              <p>{m.label}</p>
              {m.claimedBy && (
                <Link href={`/submissions/${m.claimedBy}`}>
                  View winning receipt <ArrowUpRight size={12} />
                </Link>
              )}
            </div>
          </li>
        ))}
      </ol>
      <p className="ladder-note">
        One result claims every unclaimed threshold it crosses. Earliest
        qualifying receipt wins.
      </p>
    </section>
  );
}
export function FlagForm({ versionId }: { versionId: string }) {
  const [category, setCategory] = useState("science");
  const [message, setMessage] = useState("");
  const [url, setUrl] = useState("");
  const [done, setDone] = useState(false);
  const action = useAction();
  return (
    <form
      className="panel flag-form"
      onSubmit={async (e) => {
        e.preventDefault();
        const r = await action.run("/flags", {
          versionId,
          category,
          message,
          evidenceUrl: url || undefined,
        });
        if (r) setDone(true);
      }}
    >
      <h3>Flag a concern</h3>
      <p>
        Provide specific evidence. Flags do not automatically change scores or
        milestone claims.
      </p>
      {done ? (
        <div className="success-note">
          <Check size={16} />
          Your flag is recorded for review.
        </div>
      ) : (
        <>
          <Field label="Category">
            <select
              value={category}
              onChange={(e) => setCategory(e.target.value)}
            >
              <option value="science">Scientific mapping</option>
              <option value="integrity">Validator integrity</option>
              <option value="reproducibility">Reproducibility</option>
              <option value="metric">Metric design</option>
              <option value="rights">Data or licensing rights</option>
              <option value="safety">Safety</option>
            </select>
          </Field>
          <Field label="Evidence and concern">
            <textarea
              required
              minLength={20}
              rows={4}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
            />
          </Field>
          <Field label="Evidence URL (optional)">
            <input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
          </Field>
          <ErrorMessage error={action.error} />
          <button className="button ghost" disabled={action.busy}>
            {action.busy ? "Recording…" : "Submit flag"}
            <Flag size={14} />
          </button>
        </>
      )}
    </form>
  );
}
function SubmitForm({
  challenge: c,
  onAccepted,
}: {
  challenge: Challenge;
  onAccepted: () => void;
}) {
  const session = useSession();
  const [repository, setRepository] = useState("");
  const [ref, setRef] = useState("");
  const [model, setModel] = useState("");
  const [harness, setHarness] = useState("");
  const [disclosure, setDisclosure] = useState("");
  const [license, setLicense] = useState(
    asText(
      asRecord(c.manifest?.submission).license,
      asText(asRecord(c.manifest?.submission).requiredLicense, "MIT"),
    ),
  );
  const [consent, setConsent] = useState(false);
  const [publish, setPublish] = useState(false);
  const [intent, setIntent] = useState<Intent>();
  const resource = useResource<Intent>(
    intent ? `/submission-intents/${intent.id}` : null,
    4000,
  );
  const current = resource.data || intent;
  const action = useAction();
  const [accepted, setAccepted] = useState<string>();
  if (!session.data?.user)
    return (
      <div className="panel admission-panel">
        <LockKeyhole />
        <div>
          <h3>Sign in to submit an exact artifact.</h3>
          <p>
            Official validation is invitation-only and capped. Public
            exploration and local verification remain open.
          </p>
        </div>
        <Link href="/account" className="button primary">
          Sign in with GitHub
          <ArrowRight size={15} />
        </Link>
      </div>
    );
  if (!session.data.capabilities.submission)
    return (
      <div className="panel admission-panel">
        <LockKeyhole />
        <div>
          <h3>
            Hosted submissions are currently unavailable for this account.
          </h3>
          <p>
            {!session.data.user.invited
              ? "An invitation is required for hosted validation."
              : "Check your quota and the current runner availability in your account."}{" "}
            You can still reproduce and improve artifacts locally.
          </p>
        </div>
        <Link href="/account" className="button ghost">
          View access
        </Link>
      </div>
    );
  return (
    <section className="panel submit-panel">
      <div className="section-title">
        <div>
          <div className="section-kicker">
            EXACT COMMIT · REPRODUCIBLE VERIFICATION
          </div>
          <h2>Submit a construction</h2>
        </div>
        <Badge>{session.data.quotas.remaining} validations remaining</Badge>
      </div>
      {accepted ? (
        <div className="success-note">
          <FileCheck2 size={22} />
          <div>
            <strong>Submission accepted. Your place is recorded.</strong>
            <p>
              Follow validation, confirmation under this challenge’s locked
              policy, and ordered adjudication.
            </p>
            <Link href={`/submissions/${accepted}`} className="button primary">
              Open submission receipt
              <ArrowRight size={16} />
            </Link>
          </div>
        </div>
      ) : current ? (
        <div>
          <Status value={current.status} />
          <dl className="contract-grid">
            <div>
              <dt>Source commit</dt>
              <dd className="mono">{current.sourceCommit || ref}</dd>
            </div>
            <div>
              <dt>Artifact digest</dt>
              <dd className="mono">
                {current.artifactDigest || "Being independently fetched"}
              </dd>
            </div>
          </dl>
          <Findings findings={current.findings} />
          <ErrorMessage error={resource.error} retry={resource.refresh} />
          <ErrorMessage error={action.error} />
          {current.status === "ready" ? (
            <>
              <p>
                The platform has fetched your artifact. Accepting reserves
                validation capacity and assigns a receipt sequence.
              </p>
              <button
                className="button primary"
                disabled={action.busy}
                onClick={async () => {
                  const r = await action.run<{ submissionId: string }>(
                    `/submission-intents/${current.id}/accept`,
                  );
                  if (r) {
                    setAccepted(r.submissionId);
                    onAccepted();
                    session.refresh();
                  }
                }}
              >
                Accept & reserve validation
                <ArrowRight size={16} />
              </button>
            </>
          ) : current.status === "failed" ? (
            <button
              className="button ghost"
              onClick={() => {
                setIntent(undefined);
                action.clearError();
              }}
            >
              Revise source
            </button>
          ) : current.submissionId ? (
            <Link
              className="button primary"
              href={`/submissions/${current.submissionId}`}
            >
              Open submission
            </Link>
          ) : (
            <Loading label="Fetching and quarantining the exact source" />
          )}
        </div>
      ) : (
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            const r = await action.run<Intent>("/submission-intents", {
              versionId: c.versionId,
              repository,
              ref,
              license,
              attribution: {
                model: model || undefined,
                harness: harness || undefined,
                disclosure: disclosure || undefined,
              },
              publish,
            });
            if (r) setIntent(r);
          }}
        >
          <div className="form-grid">
            <Field
              label="GitHub repository"
              help="The platform must have access to this repository."
            >
              <input
                required
                placeholder="owner/repository"
                pattern="[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+"
                value={repository}
                onChange={(e) => setRepository(e.target.value.trim())}
              />
            </Field>
            <Field
              label="Exact pushed commit"
              help="A full 40-character Git commit SHA."
            >
              <input
                required
                pattern="[a-fA-F0-9]{40}"
                className="mono"
                placeholder="40-character SHA"
                value={ref}
                onChange={(e) => setRef(e.target.value.trim())}
              />
            </Field>
            <Field label="Model (self-attested)">
              <input
                value={model}
                placeholder="Model name and version, or human-only"
                onChange={(e) => setModel(e.target.value)}
              />
            </Field>
            <Field label="Harness (self-attested)">
              <input
                value={harness}
                placeholder="Agent / harness and version"
                onChange={(e) => setHarness(e.target.value)}
              />
            </Field>
            <Field label="Required artifact license">
              <input
                required
                value={license}
                onChange={(e) => setLicense(e.target.value)}
              />
            </Field>
            <Field
              label="Public research note (optional)"
              help="Keep private reasoning, credentials, and unpublished research out of this field."
            >
              <input
                value={disclosure}
                onChange={(e) => setDisclosure(e.target.value)}
              />
            </Field>
          </div>
          <p className="field-help">
            The best verified score is public even when its artifact remains
            private. Unpublished artifact files and full receipts are visible
            only to their submitter.
          </p>
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={publish}
              onChange={(e) => setPublish(e.target.checked)}
            />
            Also publish this artifact if it does not reach the public frontier.
          </label>
          <label className="checkbox-label">
            <input
              type="checkbox"
              required
              checked={consent}
              onChange={(e) => setConsent(e.target.checked)}
            />
            I agree that public-frontier artifacts and the submitted attribution
            are published immediately under the declared license. Official
            scores come from the validator.
          </label>
          <ErrorMessage error={action.error} />
          <button className="button primary" disabled={action.busy || !consent}>
            {action.busy ? "Creating intent…" : "Fetch & inspect exact commit"}
            <ArrowRight size={16} />
          </button>
        </form>
      )}
    </section>
  );
}
