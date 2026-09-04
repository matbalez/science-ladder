"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ArrowRight,
  ArrowUpRight,
  Check,
  CircleUserRound,
  Github,
  KeyRound,
  LockKeyhole,
  LogOut,
  Plus,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import { useAction, useResource } from "@/lib/api";
import { asRecord, asText, dateLabel, humanize } from "@/lib/scientific";
import type { Candidate, Challenge, Intent, Submission } from "@/lib/types";
import { useSession } from "./shell";
import {
  Badge,
  CodeBlock,
  Empty,
  ErrorMessage,
  Field,
  Loading,
  Status,
} from "./ui";
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
  const [approved, setApproved] = useState(false);
  const [authError, setAuthError] = useState("");
  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    setCliId(
      q.get("cliSession") || q.get("session") || q.get("sessionId") || "",
    );
    setCliCode(q.get("userCode") || q.get("code") || "");
    setAuthError(q.get("error") || "");
  }, []);
  return (
    <div className="page account-page">
      <div className="eyebrow">
        <CircleUserRound size={14} /> YOUR SCIENTIFIC WORKSPACE
      </div>
      <header className="page-heading">
        <div>
          <h1>
            {me?.user ? "Hello, " : ""}
            <em>{me?.user?.login || "Make your contribution."}</em>
          </h1>
          <p>
            {me?.user
              ? "Your challenges, artifacts, and durable contribution record."
              : "Connect GitHub to create a challenge or submit an exact, reproducible artifact."}
          </p>
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
            <h2>Identity, attached to the work.</h2>
            <p>
              Your GitHub identity attributes challenges and candidate
              artifacts. Winning constructions are shared under the declared
              open license.
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
                    ? "Sign-in availability could not be read. Retry the connection above. Public protocol documentation and local tools remain available."
                    : "GitHub sign-in is not configured on this host yet. Public challenges, the Scout prompt, and local tools are available."}
                </p>
              </div>
            )}
            <span className="tiny-label">
              NO PRIVATE AGENT TRACES COLLECTED
            </span>
          </div>
          <div className="signin-description">
            <div className="section-kicker">INVITATION PREVIEW</div>
            <h2>
              Public science.
              <br />
              Carefully opened compute.
            </h2>
            <p>
              Anyone can browse the record, explore the protocol, draft a
              candidate, and reproduce results locally. Hosted creation and
              validation are currently reserved for invited accounts.
            </p>
            <Link href="/docs#solver">
              Start with the local tools
              <ArrowUpRight size={15} />
            </Link>
            <Link href="/create">
              Try the Challenge Scout
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
              <p>Free, subject-bound validation grants</p>
            </div>
            <div>
              <span className="tiny-label">ACTIVE RUN LIMIT</span>
              <strong>{me.quotas.activeLimit}</strong>
              <p>Per-account concurrent validation</p>
            </div>
            <div>
              <span className="tiny-label">HOST CAPABILITIES</span>
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
                Your account is ready, but an invitation is needed to create
                challenges and reserve official validation. An operator grants
                access to your GitHub account. You can continue drafting and
                testing locally.
              </p>
            </div>
          )}
          {me.quotas.remaining === 0 && me.user.invited && (
            <div className="note">
              <LockKeyhole size={20} />
              <p>
                Your hosted validation quota is exhausted. Local validation
                remains available. An operator must increase your allocation
                before a new official run can be accepted.
              </p>
            </div>
          )}
          <section className="panel">
            <div className="section-kicker">REPOSITORY ACCESS</div>
            <h2>Connect the repositories you use.</h2>
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
          </section>
          <section className="panel cli-session">
            <div className="section-title">
              <div>
                <div className="section-kicker">CONNECT YOUR TERMINAL</div>
                <h2>Approve a CLI session</h2>
              </div>
              <Terminal size={22} />
            </div>
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
          </section>
          <ErrorMessage error={dashboard.error} retry={dashboard.refresh} />
          {dashboard.loading && !dashboard.data ? (
            <Loading label="Loading your scientific activity" />
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
                  <Empty title="A question worth asking starts here.">
                    Use the Challenge Scout to turn primary evidence into a
                    candidate, then attach a verified repository.
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
            View receipt
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
              ? "Revise source from challenge page"
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
      <div className="section-kicker">OPERATOR ACCESS CONTROL</div>
      <h2>Invite a participant</h2>
      <p>Grant account-bound access and a finite hosted-validation quota.</p>
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
