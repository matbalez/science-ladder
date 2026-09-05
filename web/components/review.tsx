"use client";
import Link from "next/link";
import { useState } from "react";
import { Check, ClipboardCheck, Flag, LockKeyhole } from "lucide-react";
import { useAction, useResource } from "@/lib/api";
import { asText, dateLabel } from "@/lib/scientific";
import { useSession } from "./shell";
import { ResearcherEditor } from "./researchers";
import {
  Badge,
  Empty,
  ErrorMessage,
  Field,
  JsonViewer,
  Loading,
  Status,
} from "./ui";
export function ReviewConsole({
  initialChallenge = "",
  initialVersion = "",
}: {
  initialChallenge?: string;
  initialVersion?: string;
}) {
  const session = useSession();
  const allowed = session.data?.capabilities.review;
  const queue = useResource<{
    flags: Record<string, unknown>[];
    reviews: Record<string, unknown>[];
    candidates: Record<string, unknown>[];
  }>(allowed ? "/editor/queue" : null, 15000);
  const [versionId, setVersionId] = useState("");
  const [decision, setDecision] = useState("human_reviewed");
  const [reason, setReason] = useState("");
  const [done, setDone] = useState(false);
  const action = useAction();
  return (
    <div className="page review-page">
      <header className="page-heading">
        <div>
          <h1>Review</h1>
          <p>Review challenges and respond to reported concerns.</p>
        </div>
      </header>
      {session.loading ? (
        <Loading />
      ) : !allowed ? (
        <div className="panel admission-panel">
          <LockKeyhole size={24} />
          <div>
            <h3>Editor access required.</h3>
            <p>This console is available to invited editors and operators.</p>
          </div>
          <Link className="button ghost" href="/account">
            View your account
          </Link>
        </div>
      ) : (
        <>
          <ErrorMessage error={queue.error} retry={queue.refresh} />
          <ResearcherEditor
            initialChallenge={initialChallenge}
            initialVersion={initialVersion}
          />
          <div className="two-column">
            <div>
              {queue.loading && !queue.data ? (
                <Loading />
              ) : (
                [
                  ["flags", "Open flags"],
                  ["reviews", "Scientific reviews"],
                  ["candidates", "Candidate review"],
                ].map(([key, title]) => (
                  <section className="content-section" key={key}>
                    <div className="section-title">
                      <h2>{title}</h2>
                      <Badge>{queue.data?.[key as "flags"]?.length || 0}</Badge>
                    </div>
                    {queue.data?.[key as "flags"]?.length ? (
                      queue.data[key as "flags"].map((item, i) => (
                        <article
                          key={asText(item.id, String(i))}
                          className="queue-item"
                        >
                          <div className="section-title">
                            <h3>
                              {asText(
                                item.title,
                                asText(
                                  item.category,
                                  asText(item.type, `Record ${i + 1}`),
                                ),
                              )}
                            </h3>
                            <Status value={asText(item.status, "open")} />
                          </div>
                          <p>{asText(item.message, asText(item.reason))}</p>
                          <div className="inline-meta">
                            <span className="subtle">
                              {dateLabel(
                                asText(item.createdAt, asText(item.created_at)),
                              )}
                            </span>
                            {asText(
                              item.versionId,
                              asText(item.version_id),
                            ) && (
                              <button
                                className="button small ghost"
                                onClick={() => {
                                  setVersionId(
                                    asText(
                                      item.versionId,
                                      asText(item.version_id),
                                    ),
                                  );
                                  setDone(false);
                                  document
                                    .getElementById("decision-form")
                                    ?.scrollIntoView({ behavior: "smooth" });
                                }}
                              >
                                Review this version
                              </button>
                            )}
                          </div>
                          <JsonViewer
                            value={item}
                            label="Inspect evidence and references"
                          />
                        </article>
                      ))
                    ) : (
                      <Empty
                        title="No items in this queue."
                        icon={<Check size={22} />}
                      >
                        New flags and review requests will appear here.
                      </Empty>
                    )}
                  </section>
                ))
              )}
            </div>
            <aside>
              <form
                id="decision-form"
                className="panel editor-form"
                onSubmit={async (e) => {
                  e.preventDefault();
                  setDone(false);
                  const r = await action.run("/editor/decisions", {
                    versionId,
                    action: decision,
                    reason,
                  });
                  if (r) {
                    setDone(true);
                    queue.refresh();
                  }
                }}
              >
                <h3>Record an editorial decision</h3>
                <p>
                  Every action requires a public rationale. Historical scores
                  and receipts are preserved.
                </p>
                <Field label="Challenge version ID">
                  <input
                    required
                    value={versionId}
                    onChange={(e) => {
                      setVersionId(e.target.value.trim());
                      setDone(false);
                    }}
                  />
                </Field>
                <Field label="Action">
                  <select
                    value={decision}
                    onChange={(e) => {
                      setDecision(e.target.value);
                      setDone(false);
                    }}
                  >
                    <option value="human_reviewed">
                      Grant human-reviewed label
                    </option>
                    <option value="feature">Feature challenge</option>
                    <option value="unfeature">Remove featured label</option>
                    <option value="approve_review">
                      Approve scientific review
                    </option>
                    <option value="changes_required">Require changes</option>
                    <option value="reject">Reject review</option>
                    <option value="pause">Pause new submissions</option>
                    <option value="resume">Resume submissions</option>
                    <option value="compromise">Mark version compromised</option>
                  </select>
                </Field>
                <Field label="Evidence and public rationale">
                  <textarea
                    rows={5}
                    minLength={20}
                    required
                    value={reason}
                    onChange={(e) => {
                      setReason(e.target.value);
                      setDone(false);
                    }}
                  />
                </Field>
                <ErrorMessage error={action.error} />
                {done && (
                  <div className="success-note">
                    <Check size={17} />
                    Decision recorded.
                  </div>
                )}
                <button className="button primary" disabled={action.busy}>
                  <ClipboardCheck size={16} />
                  {action.busy ? "Recording…" : "Record decision"}
                </button>
              </form>
            </aside>
          </div>
        </>
      )}
    </div>
  );
}
