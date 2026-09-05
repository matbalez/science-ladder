"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import {
  ArrowRight,
  ArrowUpRight,
  Check,
  CheckCheck,
  FileCode2,
  FileSearch,
  GitBranch,
  LockKeyhole,
  Search,
  Upload,
} from "lucide-react";
import { parseDocument } from "yaml";
import styles from "./creator.module.css";
import { useAction, useResource } from "@/lib/api";
import { asList, asRecord, asText, humanize } from "@/lib/scientific";
import type { Candidate, Finding, Preflight } from "@/lib/types";
import { useSession } from "./shell";
import {
  CodeBlock,
  CopyButton,
  DownloadButton,
  ErrorMessage,
  Field,
  Findings,
  JsonViewer,
  Loading,
  Status,
} from "./ui";
type Creation = { id: string; slug: string; versionId: string; status: string };
export function Creator() {
  const session = useSession();
  const [path, setPath] = useState<"scout" | "import">("scout");
  const [inputs, setInputs] = useState({
    topic: "",
    question: "",
    papers: "",
    resources: "",
    compute: "",
    constraints: "",
  });
  const prompt = useResource<{ version: string; prompt: string }>(
    "/prompts/challenge-scout/v1",
  );
  const [document, setDocument] = useState("");
  const [parseError, setParseError] = useState<Error>();
  const [validation, setValidation] = useState<{
    valid: boolean;
    findings: Finding[];
    candidate?: Record<string, unknown>;
  }>();
  const [candidate, setCandidate] = useState<Candidate>();
  const candidateResource = useResource<Candidate>(
    candidate ? `/candidates/${candidate.id}` : null,
    5000,
  );
  const currentCandidate = candidateResource.data || candidate;
  const [creation, setCreation] = useState<Creation>();
  const [repository, setRepository] = useState("");
  const [ref, setRef] = useState("");
  const [adoption, setAdoption] = useState("");
  const [preflight, setPreflight] = useState<Preflight>();
  const preflightResource = useResource<Preflight>(
    preflight ? `/preflights/${preflight.id}` : null,
    5000,
  );
  const currentPreflight = preflightResource.data || preflight;
  const preflightReports = Array.isArray(currentPreflight?.reports)
    ? currentPreflight.reports
    : currentPreflight?.reports
      ? [currentPreflight.reports]
      : [];
  const [lock, setLock] = useState<{ lockDigest: string; status: string }>();
  const [published, setPublished] = useState(false);
  const action = useAction();
  const inputFile = useRef<HTMLInputElement>(null);
  const [restored, setRestored] = useState(false);
  useEffect(() => {
    try {
      const saved = sessionStorage.getItem("science-ladder-creator-draft");
      if (saved) {
        const s = JSON.parse(saved);
        setInputs(s.inputs || inputs);
        setDocument(s.document || "");
        if (s.path === "scout" || s.path === "import") setPath(s.path);
        setRepository(s.repository || "");
        setRef(s.ref || "");
        if (s.candidate) setCandidate(s.candidate);
        if (s.creation) setCreation(s.creation);
        if (s.preflight) setPreflight(s.preflight);
        if (s.lock) setLock(s.lock);
      }
    } catch {}
    const params = new URLSearchParams(window.location.search);
    if (params.get("path") === "import") setPath("import");
    const selected = params.get("candidate");
    if (selected) {
      setCandidate({
        id: selected,
        status: "resolving_sources",
        candidate: {},
        findings: [],
      });
      setCreation(undefined);
      setPreflight(undefined);
      setLock(undefined);
    }
    setRestored(true);
  }, []);
  useEffect(() => {
    if (restored)
      try {
        sessionStorage.setItem(
          "science-ladder-creator-draft",
          JSON.stringify({
            inputs,
            path,
            document,
            repository,
            ref,
            candidate,
            creation,
            preflight,
            lock,
          }),
        );
      } catch {}
  }, [
    inputs,
    path,
    document,
    repository,
    ref,
    candidate,
    creation,
    preflight,
    lock,
    restored,
  ]);
  const replacements: Record<string, string> = {
    FIELD_OR_TOPIC: inputs.topic,
    OPEN_QUESTION_OR_BLANK: inputs.question,
    SEED_PAPERS_OR_BLANK: inputs.papers,
    RESOURCES_OR_BLANK: inputs.resources,
    RESOURCE_CEILING_OR_BLANK: inputs.compute,
    CONSTRAINTS_OR_BLANK: inputs.constraints,
  };
  const filledPrompt =
    prompt.data?.prompt.replace(
      /\{\{([A-Z_]+)\}\}/g,
      (_, key) => replacements[key] || "(investigate)",
    ) || "";
  const steps = ["Candidate", "Repository", "Verification", "Publish"];
  const active = published
    ? 4
    : lock
      ? 3
      : preflight
        ? 2
        : currentCandidate
          ? 1
          : 0;
  async function inspect() {
    setParseError(undefined);
    setValidation(undefined);
    try {
      const parsed = parseDocument(document, { uniqueKeys: true });
      if (parsed.errors.length) throw new Error(parsed.errors[0].message);
      const value = parsed.toJS();
      if (!value || typeof value !== "object" || Array.isArray(value))
        throw new Error("The candidate must be a YAML object.");
      if (value.kind === "ChallengeManifest")
        throw new Error(
          "This is a challenge manifest (science-ladder.yaml). Import science-ladder-candidate.yaml here; attach the repository in the next step.",
        );
      const result = await action.run<{
        valid: boolean;
        findings: Finding[];
        candidate?: Record<string, unknown>;
      }>("/candidates/validate", { document });
      if (result) setValidation(result);
    } catch (e) {
      setParseError(e as Error);
    }
  }
  return (
    <div className="page creator-page">
      <header className="page-heading">
        <h1>Create a challenge</h1>
      </header>
      <ol className="creation-steps">
        {steps.map((step, i) => (
          <li
            key={step}
            className={active > i ? "complete" : active === i ? "active" : ""}
          >
            <span>
              {active > i ? (
                <Check size={14} />
              ) : (
                String(i + 1).padStart(2, "0")
              )}
            </span>
            {step}
          </li>
        ))}
      </ol>
      {!currentCandidate ? (
        <>
          <div className="creator-paths" aria-label="Starting point">
            <button
              className={path === "scout" ? "selected" : ""}
              aria-pressed={path === "scout"}
              onClick={() => setPath("scout")}
            >
              <Search size={21} />
              <div>
                <strong>Find a challenge</strong>
                <span>Give your agent a research prompt.</span>
              </div>
            </button>
            <button
              className={path === "import" ? "selected" : ""}
              aria-pressed={path === "import"}
              onClick={() => setPath("import")}
            >
              <FileCode2 size={21} />
              <div>
                <strong>Import a candidate</strong>
                <span>Upload YAML, then attach your repository.</span>
              </div>
            </button>
          </div>
          {path === "scout" && (
            <section className="panel scout-panel">
              <h2>Research a challenge</h2>
              <p>
                Give this prompt to your agent. It will research the question
                and produce a candidate file for review.
              </p>
              <Field label="Field or topic">
                <input
                  placeholder="e.g. sphere packing, fusion, quantum error correction"
                  value={inputs.topic}
                  onChange={(e) =>
                    setInputs({ ...inputs, topic: e.target.value })
                  }
                />
              </Field>
              <details className={styles.options}>
                <summary>Add a question, sources or constraints</summary>
                <div className="form-grid">
                  <Field label="Open question (optional)">
                    <input
                      value={inputs.question}
                      onChange={(e) =>
                        setInputs({ ...inputs, question: e.target.value })
                      }
                    />
                  </Field>
                  <Field label="Papers or URLs (optional)">
                    <textarea
                      rows={3}
                      value={inputs.papers}
                      onChange={(e) =>
                        setInputs({ ...inputs, papers: e.target.value })
                      }
                    />
                  </Field>
                  <Field label="Data, code or benchmarks (optional)">
                    <textarea
                      rows={3}
                      value={inputs.resources}
                      onChange={(e) =>
                        setInputs({ ...inputs, resources: e.target.value })
                      }
                    />
                  </Field>
                  <Field label="Verification compute limit (optional)">
                    <input
                      placeholder="e.g. 1 CPU, 2 GB RAM, 60 seconds"
                      value={inputs.compute}
                      onChange={(e) =>
                        setInputs({ ...inputs, compute: e.target.value })
                      }
                    />
                  </Field>
                  <Field label="Other constraints (optional)">
                    <input
                      value={inputs.constraints}
                      onChange={(e) =>
                        setInputs({ ...inputs, constraints: e.target.value })
                      }
                    />
                  </Field>
                </div>
              </details>
              <ErrorMessage error={prompt.error} retry={prompt.refresh} />
              {prompt.loading ? (
                <Loading label="Loading prompt" />
              ) : (
                filledPrompt && (
                  <>
                    <div className="scout-actions">
                      <CopyButton text={filledPrompt} className="primary">
                        Copy agent prompt
                      </CopyButton>
                      <DownloadButton
                        text={filledPrompt}
                        filename={`challenge-scout-${prompt.data?.version}.md`}
                      >
                        Download
                      </DownloadButton>
                    </div>
                    <details className="prompt-details">
                      <summary>Read prompt</summary>
                      <pre>{filledPrompt}</pre>
                    </details>
                  </>
                )
              )}
              <p className={styles.nextStep}>
                Have the candidate file?{" "}
                <button
                  className={styles.textButton}
                  onClick={() => setPath("import")}
                >
                  Import it for review <ArrowRight size={14} />
                </button>
              </p>
            </section>
          )}
          {path === "import" && (
            <section className="panel import-panel">
              <div className="section-title">
                <div>
                  <h2>Candidate file</h2>
                </div>
                <button
                  className="button small ghost"
                  onClick={() => inputFile.current?.click()}
                >
                  <Upload size={14} />
                  Choose YAML file
                </button>
                <input
                  type="file"
                  accept=".yaml,.yml,text/yaml,text/plain"
                  ref={inputFile}
                  hidden
                  onChange={async (e) => {
                    const file = e.target.files?.[0];
                    if (file) {
                      if (file.size > 1_000_000) {
                        setParseError(
                          new Error(
                            "Candidate YAML must be smaller than 1 MB.",
                          ),
                        );
                        return;
                      }
                      setDocument(await file.text());
                      setParseError(undefined);
                      setValidation(undefined);
                    }
                  }}
                />
              </div>
              <p>
                Upload or paste <code>science-ladder-candidate.yaml</code>.
                You’ll attach the public GitHub repository and exact commit
                next.
              </p>
              <div className={styles.guideLinks}>
                <Link href="/docs/candidate" target="_blank">
                  YAML format &amp; example <ArrowUpRight size={14} />
                </Link>
                <span>
                  <code>science-ladder.yaml</code> belongs in the repository;
                  this step needs the candidate file.
                </span>
              </div>
              <textarea
                className="yaml-input"
                aria-label="Candidate YAML"
                placeholder="Paste science-ladder-candidate.yaml here…"
                value={document}
                onChange={(e) => {
                  setDocument(e.target.value);
                  setParseError(undefined);
                  setValidation(undefined);
                }}
                spellCheck={false}
                rows={12}
              />
              <ErrorMessage error={parseError} />
              <ErrorMessage error={action.error} />
              {validation && (
                <div className="validation-result">
                  <Status
                    value={
                      validation.valid ? "schema_valid" : "changes_required"
                    }
                  />
                  <Findings findings={validation.findings} />
                  {validation.candidate && (
                    <CandidateSummary candidate={validation.candidate} />
                  )}
                </div>
              )}
              <div className="form-actions">
                <button
                  className="button ghost"
                  disabled={!document.trim() || action.busy}
                  onClick={inspect}
                >
                  <FileSearch size={16} />
                  {action.busy ? "Checking…" : "Validate candidate"}
                </button>
                {validation?.valid &&
                  (session.data?.capabilities.creation ? (
                    <button
                      className="button primary"
                      disabled={action.busy}
                      onClick={async () => {
                        const result = await action.run<Candidate>(
                          "/candidates/import",
                          { document },
                        );
                        if (result) setCandidate(result);
                      }}
                    >
                      Import candidate
                      <ArrowRight size={16} />
                    </button>
                  ) : (
                    <Link href="/account" className="button primary">
                      <LockKeyhole size={15} />
                      Sign in to import
                    </Link>
                  ))}
              </div>
            </section>
          )}
        </>
      ) : (
        <>
          <section className="panel">
            <div className="section-title">
              <div>
                <h2>
                  {asText(
                    asRecord(currentCandidate.candidate.manifest).title,
                    asText(currentCandidate.candidate.title, "Candidate"),
                  )}
                </h2>
              </div>
              <Status value={currentCandidate.status} />
            </div>
            <CandidateSummary candidate={currentCandidate.candidate} />
            <Findings findings={currentCandidate.findings} />
            <ErrorMessage
              error={candidateResource.error}
              retry={candidateResource.refresh}
            />
            <JsonViewer
              value={currentCandidate.candidate}
              label="Candidate details"
            />
          </section>
          {!creation ? (
            <section className="panel">
              <h2>Attach a repository</h2>
              <p>
                Push the manifest, checker, baseline and test fixtures to
                GitHub. Attach the commit you want reviewed.{" "}
                <Link href="/docs/candidate#repository" target="_blank">
                  Repository requirements
                </Link>
              </p>
              <details className={styles.options}>
                <summary>Need to create a repository?</summary>
                <p>
                  This creates a new directory with a draft checker. Implement
                  the checker and fixtures before attaching it.
                </p>
                <CodeBlock
                  code={
                    "sl challenge init --candidate science-ladder-candidate.yaml --out my-challenge"
                  }
                />
              </details>
              <form
                onSubmit={async (e) => {
                  e.preventDefault();
                  const r = await action.run<Creation>("/challenges", {
                    candidateId: currentCandidate.id,
                    repository,
                    ref,
                    adoptionStatement: adoption,
                  });
                  if (r) setCreation(r);
                }}
              >
                <div className="form-grid">
                  <Field label="Public GitHub repository">
                    <input
                      required
                      value={repository}
                      pattern="[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+"
                      placeholder="owner/repository"
                      onChange={(e) => setRepository(e.target.value.trim())}
                    />
                  </Field>
                  <Field label="Exact pushed commit SHA">
                    <input
                      required
                      className="mono"
                      pattern="[a-fA-F0-9]{40}"
                      placeholder="40-character SHA"
                      value={ref}
                      onChange={(e) => setRef(e.target.value.trim())}
                    />
                  </Field>
                </div>
                <Field
                  label="Review statement"
                  help="State what evidence, rights, metric, and limitations you have reviewed. This is recorded with the challenge."
                >
                  <textarea
                    required
                    rows={4}
                    minLength={30}
                    value={adoption}
                    placeholder="Describe the evidence, baseline and licenses you checked, and any limitations."
                    onChange={(e) => setAdoption(e.target.value)}
                  />
                </Field>
                <ErrorMessage error={action.error} />
                <button
                  className="button primary"
                  disabled={
                    action.busy ||
                    currentCandidate.status === "resolving_sources"
                  }
                >
                  {action.busy ? "Fetching source…" : "Attach repository"}
                  <GitBranch size={16} />
                </button>
                {currentCandidate.status === "resolving_sources" && (
                  <p className="field-help">
                    Source resolution is still running. This page updates
                    automatically.
                  </p>
                )}
              </form>
            </section>
          ) : (
            <section className="panel">
              <div className="section-title">
                <div>
                  <h2>
                    {published ? "Challenge published" : "Review and publish"}
                  </h2>
                </div>
                <Status
                  value={
                    published ? "published" : lock ? "locked" : creation.status
                  }
                />
              </div>
              <div className="source-reference">
                <GitBranch size={16} />
                {repository}
                <code>{ref}</code>
              </div>
              {!preflight ? (
                <>
                  <p>
                    Run the hosted checker tests, then complete scientific
                    review before publication.
                  </p>
                  <button
                    className="button primary"
                    disabled={action.busy}
                    onClick={async () => {
                      const r = await action.run<Preflight>(
                        `/challenge-versions/${creation.versionId}/preflights`,
                      );
                      if (r) setPreflight(r);
                    }}
                  >
                    Run verification
                    <ArrowRight size={16} />
                  </button>
                </>
              ) : (
                <>
                  <div className="preflight-status">
                    <FileSearch size={22} />
                    <div>
                      <h3>Verification</h3>
                      <Status value={currentPreflight?.status || "queued"} />
                    </div>
                  </div>
                  <Findings findings={currentPreflight?.findings} />
                  {preflightReports.map((r, i) => (
                    <JsonViewer
                      key={i}
                      value={r}
                      label={`Report ${i + 1}: ${asText(r.kind, asText(r.type, "checks and evidence"))}`}
                    />
                  ))}
                  <ErrorMessage
                    error={preflightResource.error}
                    retry={preflightResource.refresh}
                  />
                  {[
                    "machine_ready",
                    "ready_with_warnings",
                    "passed",
                    "pass",
                    "completed",
                  ].includes(currentPreflight?.status || "") &&
                    !lock && (
                      <button
                        className="button primary"
                        disabled={action.busy}
                        onClick={async () => {
                          const r = await action.run<{
                            lockDigest: string;
                            status: string;
                          }>(`/challenge-versions/${creation.versionId}/lock`);
                          if (r) setLock(r);
                        }}
                      >
                        <LockKeyhole size={16} />
                        Lock challenge
                      </button>
                    )}
                  {lock && (
                    <div className="lock-record">
                      <CheckCheck size={22} />
                      <div>
                        <h3>Challenge locked</h3>
                        <p>
                          The checker, thresholds, deadline and licenses are
                          fixed.
                        </p>
                        <a
                          className="mono"
                          href={`/v1/receipts/${encodeURIComponent(lock.lockDigest)}`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          {lock.lockDigest}
                          <ArrowUpRight size={14} />
                        </a>
                      </div>
                    </div>
                  )}
                  {lock && !published && (
                    <button
                      className="button primary"
                      disabled={action.busy}
                      onClick={async () => {
                        const r = await action.run(
                          `/challenge-versions/${creation.versionId}/publish`,
                        );
                        if (r) {
                          setPublished(true);
                          sessionStorage.removeItem(
                            "science-ladder-creator-draft",
                          );
                        }
                      }}
                    >
                      Publish challenge
                      <ArrowUpRight size={16} />
                    </button>
                  )}
                  {published && (
                    <Link
                      className="button primary"
                      href={`/challenges/${creation.slug}`}
                    >
                      View the live challenge
                      <ArrowRight size={16} />
                    </Link>
                  )}
                </>
              )}
              <ErrorMessage error={action.error} />
            </section>
          )}
          <button
            className="text-button"
            onClick={() => {
              setCandidate(undefined);
              setCreation(undefined);
              setPreflight(undefined);
              setLock(undefined);
              setPublished(false);
              setValidation(undefined);
              sessionStorage.removeItem("science-ladder-creator-draft");
            }}
          >
            Start another candidate
          </button>
        </>
      )}
    </div>
  );
}
function CandidateSummary({
  candidate,
}: {
  candidate: Record<string, unknown>;
}) {
  const manifest = asRecord(candidate.manifest);
  const science = asRecord(candidate.science);
  const provenance = asRecord(candidate.provenance);
  return (
    <div className="candidate-summary">
      <dl className="contract-grid">
        <div>
          <dt>Assessment</dt>
          <dd>
            {humanize(
              asText(candidate.disposition, asText(candidate.verdict, "Draft")),
            )}
          </dd>
        </div>
        <div>
          <dt>Prompt version</dt>
          <dd>
            {asText(
              candidate.promptVersion,
              asText(provenance.promptVersion, "Recorded in candidate"),
            )}
          </dd>
        </div>
        <div>
          <dt>Primary evidence</dt>
          <dd>
            {
              asList(
                candidate.sources || candidate.citations || science.citations,
              ).length
            }{" "}
            citations
          </dd>
        </div>
        <div>
          <dt>Field</dt>
          <dd>
            {asText(
              candidate.field,
              asText(manifest.title, "Computational science"),
            )}
          </dd>
        </div>
      </dl>
      {asText(
        manifest.scientificQuestion,
        asText(candidate.question, asText(science.question)),
      ) && (
        <p>
          {asText(
            manifest.scientificQuestion,
            asText(candidate.question, asText(science.question)),
          )}
        </p>
      )}
      {asList(candidate.unresolvedQuestions || candidate.uncertainties).length >
        0 && (
        <div className="note">
          <FileSearch size={17} />
          <div>
            <strong>Unresolved questions</strong>
            <ul>
              {asList(
                candidate.unresolvedQuestions || candidate.uncertainties,
              ).map((v, i) => (
                <li key={i}>{typeof v === "string" ? v : JSON.stringify(v)}</li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}
