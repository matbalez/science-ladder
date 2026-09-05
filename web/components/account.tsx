"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ArrowRight,
  ArrowUpRight,
  Check,
  Github,
  KeyRound,
  LockKeyhole,
  LogOut,
  Plus,
  ShieldCheck,
} from "lucide-react";
import { useAction, useResource } from "@/lib/api";
import { asRecord, asText, dateLabel, humanize } from "@/lib/scientific";
import type { Candidate, Challenge, Intent, Submission } from "@/lib/types";
import { useSession } from "./shell";
import { Badge, Empty, ErrorMessage, Field, Loading, Status } from "./ui";
import { SubmissionTable } from "./submission";
export function Account() {
  const session = useSession();
  const me = session.data;
  const action = useAction();
  const dashboard = useResource<{
    challenges: Challenge[];
    candidates: Candidate[];
    submissions: Submission[];
    intents: Intent[];
  }>(me?.user ? "/dashboard" : null, 15000);
  const [cliId, setCliId] = useState("");
  const [cliCode, setCliCode] = useState("");
  const [cliOpen, setCliOpen] = useState(false);
  const [approved, setApproved] = useState(false);
  const [authError, setAuthError] = useState("");
  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    const callbackSession =
      q.get("cliSession") || q.get("session") || q.get("sessionId") || "";
    setCliId(callbackSession);
    if (callbackSession) setCliOpen(true);
    setCliCode(q.get("userCode") || q.get("code") || "");
    setAuthError(q.get("error") || "");
  }, []);
  return (
    <div className="page account-page">
      <header className="page-heading">
        <div>
          <h1>Account</h1>
        </div>
        {me?.user && (
          <button
            className="button small ghost"
            disabled={action.busy}
            onClick={async () => {
              await action.run("/auth/logout");
              session.refresh();
            }}
          >
            <LogOut size={14} />
            Sign out
          </button>
        )}
      </header>
      <ErrorMessage error={session.error} retry={session.refresh} />
      <ErrorMessage error={action.error} />
      {authError && (
        <div className="error" role="alert">
          GitHub sign-in could not finish: {humanize(authError)}. Please try
          again.
        </div>
      )}
      {session.loading && !me ? (
        <Loading />
      ) : !me?.user ? (
        <div className="account-signin">
          <div className="signin-card">
            <Github size={36} strokeWidth={1.3} />
            <h2>Sign in</h2>
            <p>
              Your GitHub account identifies your challenges and submissions.
              Results that improve the public frontier are shared under the
              challenge’s license.
            </p>
            {me?.configuration.githubAuth ? (
              <a className="button primary" href="/v1/auth/github">
                <Github size={17} />
                Continue with GitHub
                <ArrowRight size={16} />
              </a>
            ) : (
              <div className="note">
                <LockKeyhole size={18} />
                <p>
                  {session.error
                    ? "Could not check sign-in availability. Retry above. Documentation and local tools remain available."
                    : "GitHub sign-in is not configured yet. You can browse public challenges and use the local tools."}
                </p>
              </div>
            )}
          </div>
          <div className="signin-description">
            <h2>Access</h2>
            <p>
              Anyone can browse challenges, draft candidates and test locally.
              Creating hosted challenges and running platform validation
              currently require an invitation.
            </p>
            <Link href="/docs#solver">
              Local tools
              <ArrowUpRight size={15} />
            </Link>
            <Link href="/create">
              Challenge Scout
              <ArrowUpRight size={15} />
            </Link>
          </div>
        </div>
      ) : (
        <>
          <section className="account-summary">
            <div>
              <Badge tone={me.user.invited ? "lime" : "amber"}>
                {me.user.invited ? "Invited participant" : "Public account"}
              </Badge>
              <h2>{me.user.login}</h2>
              <span>{humanize(me.user.role)}</span>
            </div>
            <div>
              <span className="tiny-label">VALIDATIONS REMAINING</span>
              <strong>{me.quotas.remaining}</strong>
              <p>Free grants, each tied to a specific submission</p>
            </div>
            <div>
              <span className="tiny-label">ACTIVE RUN LIMIT</span>
              <strong>{me.quotas.activeLimit}</strong>
              <p>Validations that can run at once</p>
            </div>
            <div>
              <span className="tiny-label">SERVICES</span>
              <ul>
                <li>
                  <i
                    className={
                      me.configuration.officialRunner ? "status-dot" : "off-dot"
                    }
                  />
                  {me.configuration.officialRunner
                    ? "Platform verification configured"
                    : "Platform verification unavailable"}
                </li>
                <li>
                  <i
                    className={
                      me.configuration.scientificReview
                        ? "status-dot"
                        : "off-dot"
                    }
                  />
                  {me.configuration.scientificReview
                    ? "Science review configured"
                    : "Science review unavailable"}
                </li>
              </ul>
            </div>
          </section>
          {!me.user.invited && (
            <div className="note">
              <LockKeyhole size={20} />
              <p>
                An operator must invite your GitHub account before you can
                create challenges or request platform validation. You can draft
                and test locally now.
              </p>
            </div>
          )}
          {me.quotas.remaining === 0 && me.user.invited && (
            <div className="note">
              <LockKeyhole size={20} />
              <p>
                You have no hosted validations remaining. An operator must
                increase your quota before another run can be accepted. Local
                testing remains available.
              </p>
            </div>
          )}
          <details className="panel account-settings">
            <summary>Repository access</summary>
            <p>
              The Science Ladder GitHub App reads exact commits from
              repositories you authorize. Install it on your challenge and
              solver repositories before submitting.
            </p>
            <a
              className="button ghost"
              href="https://github.com/apps/science-ladder/installations/new"
              target="_blank"
              rel="noreferrer"
            >
              <Github size={16} />
              Install or configure GitHub App
              <ArrowUpRight size={14} />
            </a>
          </details>
          <details
            className="panel account-settings cli-session"
            open={cliOpen}
            onToggle={(event) => setCliOpen(event.currentTarget.open)}
          >
            <summary>Approve a CLI session</summary>
            <p>
              Only approve a session you initiated in your own terminal. Enter
              its session ID and displayed verification code.
            </p>
            {approved ? (
              <div className="success-note">
                <Check size={18} />
                CLI session approved. Return to your terminal to finish
                authentication.
              </div>
            ) : (
              <form
                onSubmit={async (e) => {
                  e.preventDefault();
                  const r = await action.run<{ approved: boolean }>(
                    `/auth/cli-sessions/${encodeURIComponent(cliId)}/approve`,
                    { userCode: cliCode },
                  );
                  if (r?.approved) setApproved(true);
                }}
              >
                <div className="form-grid">
                  <Field label="CLI session ID">
                    <input
                      required
                      value={cliId}
                      onChange={(e) => setCliId(e.target.value.trim())}
                      autoComplete="off"
                    />
                  </Field>
                  <Field label="Verification code">
                    <input
                      required
                      className="mono"
                      value={cliCode}
                      onChange={(e) => setCliCode(e.target.value.trim())}
                      autoComplete="off"
                    />
                  </Field>
                </div>
                <button
                  className="button ghost"
                  disabled={action.busy || !me.user.invited}
                >
                  <KeyRound size={15} />
                  Approve my CLI session
                </button>
              </form>
            )}
          </details>
          <ErrorMessage error={dashboard.error} retry={dashboard.refresh} />
          {dashboard.loading && !dashboard.data ? (
            <Loading label="Loading your activity" />
          ) : (
            <>
              <section className="content-section">
                <div className="section-title">
                  <h2>Your challenges</h2>
                  <Link href="/create" className="button small ghost">
                    <Plus size={14} />
                    Create challenge
                  </Link>
                </div>
                {dashboard.data?.challenges?.length ? (
                  <div className="account-challenges">
                    {dashboard.data.challenges.map((c) => (
                      <Link
                        href={`/challenges/${c.slug}`}
                        className="account-challenge"
                        key={c.id}
                      >
                        <div>
                          <span className="tiny-label">{c.domain}</span>
                          <h3>{c.title}</h3>
                        </div>
                        <Status value={c.status} />
                        <ArrowUpRight size={17} />
                      </Link>
                    ))}
                  </div>
                ) : (
                  <Empty title="No challenges yet.">
                    Use the Challenge Scout to draft a challenge, then attach
                    its repository.
                  </Empty>
                )}
                {dashboard.data?.candidates?.length ? (
                  <div className="candidate-list">
                    <h3>Imported candidates</h3>
                    {dashboard.data.candidates.map((c) => (
                      <div key={c.id}>
                        <span>
                          {asText(
                            asRecord(c.candidate.manifest).title,
                            asText(c.candidate.title, c.id),
                          )}
                        </span>
                        <Status value={c.status} />
                        <Link href={`/create?candidate=${c.id}`}>
                          Continue
                          <ArrowRight size={13} />
                        </Link>
                      </div>
                    ))}
                  </div>
                ) : null}
              </section>
              <section className="content-section">
                <div className="section-title">
                  <h2>Your submissions</h2>
                  <Link href="/">
                    Find a challenge
                    <ArrowUpRight size={14} />
                  </Link>
                </div>
                <SubmissionTable
                  submissions={dashboard.data?.submissions || []}
                />
              </section>
              {dashboard.data?.intents?.length ? (
                <section className="content-section">
                  <h2>Source inspections</h2>
                  <div className="table-scroll">
                    <table className="data-table">
                      <thead>
                        <tr>
                          <th>Repository</th>
                          <th>State</th>
                          <th>Created</th>
                          <th>Next step</th>
                        </tr>
                      </thead>
                      <tbody>
                        {dashboard.data.intents.map((i) => (
                          <IntentRow
                            key={i.id}
                            intent={i}
                            refresh={() => {
                              dashboard.refresh();
                              session.refresh();
                            }}
                          />
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>
              ) : null}
            </>
          )}
          {me.user.role === "operator" && <InviteForm />}
        </>
      )}
    </div>
  );
}
function IntentRow({
  intent: i,
  refresh,
}: {
  intent: Intent;
  refresh: () => void;
}) {
  const action = useAction();
  return (
    <tr>
      <td>{i.repository}</td>
      <td>
        <Status value={i.status} />
        <ErrorMessage error={action.error} />
      </td>
      <td>{dateLabel(i.createdAt)}</td>
      <td>
        {i.submissionId ? (
          <Link href={`/submissions/${i.submissionId}`}>
            View submission
            <ArrowRight size={13} />
          </Link>
        ) : i.status === "ready" ? (
          <button
            className="button small ghost"
            disabled={action.busy}
            onClick={async () => {
              const r = await action.run(`/submission-intents/${i.id}/accept`);
              if (r) refresh();
            }}
          >
            Reserve validation
          </button>
        ) : (
          <span className="subtle">
            {i.status === "failed"
              ? "Revise the source on the challenge page"
              : "Inspection in progress"}
          </span>
        )}
      </td>
    </tr>
  );
}
function InviteForm() {
  const action = useAction();
  const [githubId, setId] = useState("");
  const [role, setRole] = useState("member");
  const [quota, setQuota] = useState(20);
  const [success, setSuccess] = useState(false);
  return (
    <section className="panel">
      <h2>Invite a participant</h2>
      <p>Give a GitHub account access and a hosted validation quota.</p>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setSuccess(false);
          const r = await action.run("/invites", {
            githubId: Number(githubId),
            role,
            validationQuota: quota,
          });
          if (r) setSuccess(true);
        }}
      >
        <div className="form-grid">
          <Field label="Numeric GitHub user ID">
            <input
              required
              pattern="[0-9]{1,15}"
              value={githubId}
              onChange={(e) => setId(e.target.value)}
            />
          </Field>
          <Field label="Access role">
            <select value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="member">Member</option>
              <option value="editor">Editor</option>
            </select>
          </Field>
          <Field label="Validation allocation">
            <input
              type="number"
              min="0"
              max="100000"
              required
              value={quota}
              onChange={(e) => setQuota(Number(e.target.value))}
            />
          </Field>
        </div>
        <ErrorMessage error={action.error} />
        {success && (
          <div className="success-note">
            <Check size={16} />
            Invitation and quota recorded.
          </div>
        )}
        <button className="button ghost" disabled={action.busy}>
          <ShieldCheck size={15} />
          Grant invitation
        </button>
      </form>
    </section>
  );
}
