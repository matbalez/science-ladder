"use client";
import Link from "next/link";
import {
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  Download,
  FileCheck2,
  Globe,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react";
import { useAction, useResource } from "@/lib/api";
import { asText, dateLabel, formatTicks, humanize } from "@/lib/scientific";
import type { Submission } from "@/lib/types";
import { ArtifactViewer } from "./science-visuals";
import { Badge, Empty, ErrorMessage, JsonViewer, Loading, Status } from "./ui";
export function SubmissionTable({
  submissions,
  quantum,
}: {
  submissions: Submission[];
  quantum?: string;
}) {
  if (!submissions.length)
    return (
      <Empty
        title="No submissions yet."
        icon={<FileCheck2 size={30} />}
        children={null}
      />
    );
  return (
    <div className="table-scroll">
      <table className="data-table">
        <thead>
          <tr>
            <th>Submission</th>
            <th>Attribution</th>
            <th>{quantum ? "Score" : "Score units"}</th>
            <th>Processing</th>
            <th>Validation</th>
            <th>Publication</th>
            <th>Submitted</th>
            <th>
              <span className="sr-only">Details</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {[...submissions]
            .sort((a, b) => b.sequence - a.sequence)
            .map((s) => (
              <tr key={s.id}>
                <td>
                  <Link
                    className="receipt-number"
                    href={`/submissions/${s.id}`}
                  >
                    #{String(s.sequence).padStart(3, "0")}
                  </Link>
                </td>
                <td>
                  <strong>
                    {s.attribution?.model || "Model not supplied"}
                  </strong>
                  <small>
                    {s.attribution?.harness || "Agent software not supplied"}
                    {s.attribution?.platformSeeded && (
                      <Badge tone="amber">Platform-seeded</Badge>
                    )}
                  </small>
                </td>
                <td className="mono">
                  {formatTicks(s.scoreTicks, quantum || "1")}
                </td>
                <td>
                  <Status value={s.status} />
                </td>
                <td>
                  <Status value={s.outcome || "pending"} />
                  {s.verificationStatus && (
                    <small>
                      <Badge tone="lime">
                        {humanize(s.verificationStatus)}
                      </Badge>
                    </small>
                  )}
                </td>
                <td>
                  <span className="inline-meta">
                    {s.public ? <Globe size={13} /> : <LockKeyhole size={13} />}{" "}
                    {s.public ? "Public" : "Private"}
                  </span>
                </td>
                <td className="subtle">{dateLabel(s.createdAt)}</td>
                <td>
                  <Link
                    href={`/submissions/${s.id}`}
                    aria-label={`Open submission ${s.sequence}`}
                  >
                    <ArrowUpRight size={17} />
                  </Link>
                </td>
              </tr>
            ))}
        </tbody>
      </table>
    </div>
  );
}
export function SubmissionDetail({ id }: { id: string }) {
  const resource = useResource<Submission>(
    `/submissions/${encodeURIComponent(id)}`,
    6000,
  );
  const s = resource.data;
  const action = useAction();
  const acceptance = useResource<unknown>(
    s?.receiptDigest
      ? `/receipts/${encodeURIComponent(s.receiptDigest)}`
      : null,
  );
  const adjudication = useResource<unknown>(
    s?.adjudicationDigest
      ? `/receipts/${encodeURIComponent(s.adjudicationDigest)}`
      : null,
  );
  if (resource.loading && !s)
    return (
      <div className="page">
        <Loading />
      </div>
    );
  if (!s)
    return (
      <div className="page">
        <Link href="/account" className="back-link">
          <ArrowLeft size={14} />
          Account
        </Link>
        <ErrorMessage error={resource.error} retry={resource.refresh} />
      </div>
    );
  const states = [
    "accepted",
    "queued",
    "running",
    "confirmation_running",
    "finalized",
  ];
  const at = states.indexOf(s.status);
  return (
    <div className="page submission-page">
      <Link href="/account" className="back-link">
        <ArrowLeft size={14} />
        Account
      </Link>
      <header className="page-heading">
        <div>
          <h1>
            Submission <em>#{String(s.sequence).padStart(3, "0")}</em>
          </h1>
          <p className="mono">{s.id}</p>
        </div>
        <div className="inline-meta">
          <Status value={s.status} />
          {s.verificationStatus && (
            <Badge tone="lime">{humanize(s.verificationStatus)}</Badge>
          )}
          <Badge>{s.public ? "Public artifact" : "Private artifact"}</Badge>
        </div>
      </header>
      <ErrorMessage error={resource.error} retry={resource.refresh} />
      {s.verificationStatus && (
        <p className="note">
          {s.verificationStatus === "independently_replicated"
            ? "The same fixed checker produced matching results on different physical host groups."
            : "The platform checked this result and confirmed it in a fresh virtual machine. Independent replication has not been recorded."}
        </p>
      )}
      <ol className="creation-steps receipt-steps">
        {states.map((state, i) => (
          <li
            key={state}
            className={i < at ? "complete" : i === at ? "active" : ""}
          >
            <span>{String(i + 1).padStart(2, "0")}</span>
            {humanize(state)}
          </li>
        ))}
      </ol>
      <div className="detail-stat-row">
        <div>
          <span className="tiny-label">VALIDATION OUTCOME</span>
          <strong className="stat-word">{humanize(s.outcome)}</strong>
          <span>Separate from processing status</span>
        </div>
        <div>
          <span className="tiny-label">SCORE UNITS</span>
          <strong>{formatTicks(s.scoreTicks)}</strong>
          <span>The challenge defines the value of each unit</span>
        </div>
        <div>
          <span className="tiny-label">MILESTONES CLAIMED</span>
          <strong>{s.claims?.length || 0}</strong>
          <span>Decided in acceptance order</span>
        </div>
        <div>
          <span className="tiny-label">ACCEPTED</span>
          <strong className="stat-word">{dateLabel(s.createdAt)}</strong>
          <span>Sequence #{s.sequence}</span>
        </div>
      </div>
      <div className="two-column">
        <div>
          <section className="content-section">
            <h2>Submission details</h2>
            <dl className="contract-grid">
              <div>
                <dt>Model (reported)</dt>
                <dd>{s.attribution?.model || "Not supplied"}</dd>
              </div>
              <div>
                <dt>Agent software (reported)</dt>
                <dd>{s.attribution?.harness || "Not supplied"}</dd>
              </div>
              <div>
                <dt>Source commit</dt>
                <dd className="mono">{s.sourceCommit || "Private source"}</dd>
              </div>
              <div>
                <dt>Artifact hash</dt>
                <dd className="mono">{s.artifactDigest || "Pending"}</dd>
              </div>
            </dl>
            {s.attribution?.platformSeeded && (
              <div className="note">
                <Badge tone="amber">Platform-seeded</Badge>
                <p>
                  Created by the platform for its initial demonstration. This is
                  not an independent external contribution.
                </p>
              </div>
            )}
            {s.attribution?.disclosure && (
              <div className="public-note">
                <h3>Research note</h3>
                <p>{s.attribution.disclosure}</p>
              </div>
            )}
            {s.repository && s.sourceCommit && (
              <a
                className="button small ghost"
                href={`https://github.com/${s.repository}/tree/${s.sourceCommit}`}
                target="_blank"
                rel="noreferrer"
              >
                View source commit
                <ArrowUpRight size={14} />
              </a>
            )}
          </section>
          <section className="content-section">
            <h2>Runs</h2>
            {s.runs?.length ? (
              s.runs.map((r, i) => (
                <div key={i} className="run-record">
                  <div className="section-title">
                    <h3>Run {i + 1}</h3>
                    <Status
                      value={asText(r.status, asText(r.outcome, "recorded"))}
                    />
                  </div>
                  <JsonViewer value={r} label="Run details" />
                </div>
              ))
            ) : (
              <Empty
                title="No completed runs yet."
                icon={<ShieldCheck size={28} />}
              >
                Results appear as validation and confirmation finish. An
                infrastructure failure does not mean the scientific result is
                invalid.
              </Empty>
            )}
            {s.claims?.length > 0 && (
              <JsonViewer value={s.claims} label="Milestone claims" />
            )}
          </section>
          <ArtifactViewer digest={s.public ? s.artifactDigest : undefined} />
        </div>
        <aside>
          <section className="trust-panel">
            <FileCheck2 size={23} />
            <h3>Receipts</h3>
            <p>
              Signed receipts record the artifact, challenge rules, results and
              acceptance order. Download them for independent verification.
            </p>
            {s.receiptDigest ? (
              <>
                <a
                  href={`/v1/receipts/${encodeURIComponent(s.receiptDigest)}`}
                  className="button small ghost"
                  download
                >
                  <Download size={14} />
                  Acceptance receipt
                </a>
                {acceptance.data && (
                  <JsonViewer
                    value={acceptance.data}
                    label="Acceptance receipt details"
                  />
                )}
                <ErrorMessage
                  error={acceptance.error}
                  retry={acceptance.refresh}
                />
              </>
            ) : (
              <p>Acceptance receipt pending.</p>
            )}
            {s.adjudicationDigest && (
              <>
                <a
                  href={`/v1/receipts/${encodeURIComponent(s.adjudicationDigest)}`}
                  className="button small ghost"
                  download
                >
                  <Download size={14} />
                  Decision receipt
                </a>
                {adjudication.data && (
                  <JsonViewer
                    value={adjudication.data}
                    label="Decision receipt details"
                  />
                )}
                <ErrorMessage
                  error={adjudication.error}
                  retry={adjudication.refresh}
                />
              </>
            )}
            <Link href="/docs#receipts">
              How to verify a receipt
              <ArrowRight size={13} />
            </Link>
          </section>
          {!s.public && s.status === "finalized" && (
            <section className="panel">
              <Globe size={22} />
              <h3>Publish artifact</h3>
              <p>
                Publishing shares your artifact under the challenge’s required
                license, even if it did not improve the public frontier.
                Publication is permanent.
              </p>
              <ErrorMessage error={action.error} />
              <button
                className="button ghost"
                disabled={action.busy}
                onClick={async () => {
                  const r = await action.run(`/submissions/${s.id}/publish`);
                  if (r) resource.refresh();
                }}
              >
                Publish this artifact
                <ArrowUpRight size={14} />
              </button>
            </section>
          )}
        </aside>
      </div>
    </div>
  );
}
