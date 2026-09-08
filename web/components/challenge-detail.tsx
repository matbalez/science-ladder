"use client";
import Link from "next/link";
import { displaySummary } from "@/lib/presentation";
import { useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  BookOpen,
  Check,
  Clock3,
  Download,
  Flag,
  GitBranch,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react";
import { ApiError, useAction, useResource } from "@/lib/api";
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
import type { Challenge } from "@/lib/types";
import { useSession } from "./shell";
import { ArtifactViewer, FrontierChart } from "./science-visuals";
import {
  Badge,
  CodeBlock,
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
import { ChallengeEducation, challengeEducation } from "./challenge-education";
import { LoadPathsExplorer } from "./load-paths";
import { MeasurementContract } from "./measurement-contract";
import { Participate } from "./participate";
import { ResearcherSection } from "./researchers";
import {
  hasNativeLoadPathsChecker,
  challengeSetupCommands,
  solverInstructions,
} from "@/lib/solver-prompt";
export function ChallengeDetail({ slug }: { slug: string }) {
  const session = useSession();
  const {
    data: c,
    error,
    loading,
    refresh,
  } = useResource<Challenge>(`/challenges/${encodeURIComponent(slug)}`, 15000);
  const [tab, setTab] = useState("overview");
  const [showFlag, setShowFlag] = useState(false);
  if (loading && !c)
    return (
      <div className="page">
        <Loading />
      </div>
    );
  if (error instanceof ApiError && error.code === "challenge_withdrawn")
    return (
      <div className="page">
        <Link href="/" className="back-link">
          <ArrowLeft size={14} />
          Explore challenges
        </Link>
        <h1>Challenge removed</h1>
        <p>{error.message}</p>
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
  // This explorer explains one immutable scientific source, not arbitrary later versions.
  const explorerUrl =
    c.repository === "matbalez/science-ladder-quiet-echoes" &&
    c.sourceCommit === "f42f527e97563b1c068a1835732c6da44f21223f"
      ? "/showcase/quiet-echoes/index.html"
      : undefined;
  const hasVerifiedAttempt = c.submissions?.some(
    (submission) =>
      submission.outcome === "valid" &&
      /^-?\d+$/.test(submission.scoreTicks || "") &&
      ["platform_verified", "independently_replicated"].includes(
        submission.verificationStatus || "",
      ),
  );
  const solverPrompt = solverInstructions(c);
  return (
    <div className="page challenge-detail">
      <Link href="/" className="back-link">
        <ArrowLeft size={14} /> All challenges
      </Link>
      <ErrorMessage error={error} retry={refresh} />
      <header className="challenge-header">
        <div>
          <div className="inline-meta">
            <span className="subtle">{c.domain}</span>
            <Status value={c.status} />
            {c.intakeStatus !== "open" && (
              <Badge tone="amber">Intake {c.intakeStatus}</Badge>
            )}
            {c.badges
              .filter((b) => b.toLowerCase() !== "featured")
              .map((b) => (
                <Badge
                  key={b}
                  tone={b.toLowerCase() === "featured" ? "lime" : ""}
                >
                  {humanize(b)}
                </Badge>
              ))}
          </div>
          <h1>{c.title}</h1>
          <p className="challenge-summary">{displaySummary(c)}</p>
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
          <Participate
            instructions={solverPrompt}
            challengeTitle={c.title}
            status={`${humanize(c.status)} · ${humanize(c.reviewStatus)} · Intake ${c.intakeStatus}`}
          />
        </div>
      </header>
      {hasNativeLoadPathsChecker(c) && <LoadPathsExplorer />}
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
            {c.verifiedBest
              ? "Validation complete"
              : hasVerifiedAttempt
                ? "No verified improvement yet"
                : "Awaiting validation"}
          </span>
        </div>
        <div>
          <span className="tiny-label">MILESTONES</span>
          <strong>
            {c.milestones.filter((m) => m.claimedBy).length}
            <small>/ {c.milestones.length} claimed</small>
          </strong>
          <span>First verified submission to each threshold</span>
        </div>
        <div>
          <span className="tiny-label">REVIEW</span>
          <strong className="stat-word">
            {humanize(c.reviewStatus || "Pending review")}
          </strong>
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
          ["evaluation", "Evaluation"],
          ["history", "Submissions"],
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
                <h2>{asText(science.question, c.summary)}</h2>
                <ChallengeEducation challenge={c} />
                {!challengeEducation(c) && asText(science.impactStatement) && (
                  <p>{asText(science.impactStatement)}</p>
                )}
                {explorerUrl && (
                  <p>
                    <Link
                      href={explorerUrl}
                      style={{
                        color: "var(--lime)",
                        textDecoration: "underline",
                        textUnderlineOffset: 4,
                      }}
                    >
                      Explore the pulse and its echoes →
                    </Link>
                  </p>
                )}
                {asText(asRecord(science).metricRationale) && (
                  <>
                    <h3>Why this metric matters</h3>
                    <p>{asText(asRecord(science).metricRationale)}</p>
                  </>
                )}
                <TextList
                  title="Assumptions"
                  value={asRecord(science).assumptions}
                />
                <TextList title="Limitations" value={science.limitations} />
              </section>
              <section className="content-section">
                <h2>Research background</h2>
                {asList(science.citations).length ? (
                  asList(science.citations).map((citation, i) => {
                    const cite = asRecord(citation);
                    const identifier = asText(
                      cite.identifier,
                      asText(cite.url),
                    );
                    const url =
                      safeWebUrl(cite.url) ||
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
              <ResearcherSection
                context={c.researcherContext}
                editHref={
                  session.data?.capabilities.review
                    ? `/review?challenge=${encodeURIComponent(c.slug)}&version=${encodeURIComponent(c.versionId)}#researcher-editor`
                    : undefined
                }
              />
              <details className="content-section local-setup">
                <summary>Local setup</summary>
                <p>
                  Clone this version and reproduce the baseline before changing
                  the candidate.
                </p>
                <CodeBlock code={challengeSetupCommands(c)} />
              </details>
            </div>
            <aside>
              <MilestoneLadder challenge={c} />
              <div className="trust-panel">
                <ShieldCheck size={20} />
                <h3>Challenge record</h3>
                <p>
                  Scores measure performance under this checker, not broader
                  scientific validity.
                </p>
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
                <h2>Verified progress</h2>
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
                <h2>Scoring and validity</h2>
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
                <MeasurementContract
                  evaluation={asRecord(manifest.evaluation)}
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
                <h2>Checker and scientific reviews</h2>
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
                <h3>Version rules</h3>
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
                <h2>Submissions</h2>
              </div>
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
