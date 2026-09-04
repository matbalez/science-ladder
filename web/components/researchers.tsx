"use client";
import Link from "next/link";
import { useState } from "react";
import { Check, Plus, Save, Trash2 } from "lucide-react";
import { useAction, useResource } from "@/lib/api";
import { dateLabel, safeWebUrl } from "@/lib/scientific";
import type { Challenge, Researcher, ResearcherContext } from "@/lib/types";
import { ErrorMessage, ExternalLink, Field, Loading } from "./ui";

export const RESEARCHER_INTRO =
  "Their published work connects to this question. The community could invite them to follow our progress.";
export const RESEARCHER_NOTICE =
  "Listed for their research, not as challenge sponsors or endorsers. Their interest has not been confirmed.";

export function ResearcherSection({
  context,
  editHref,
}: {
  context?: ResearcherContext | null;
  editHref?: string;
}) {
  const researchers = context?.researchers || [];
  if (!researchers.length)
    return editHref ? (
      <p className="researcher-edit-entry">
        <Link href={editHref}>Add researcher context</Link>
      </p>
    ) : null;
  return (
    <section
      className="content-section researchers-section"
      aria-labelledby="researchers-title"
    >
      <div className="section-title">
        <h2 id="researchers-title">Researchers to know</h2>
        {editHref && <Link href={editHref}>Edit researcher context</Link>}
      </div>
      <p>{RESEARCHER_INTRO}</p>
      <p className="researcher-notice">{RESEARCHER_NOTICE}</p>
      <div className="researcher-list">
        {researchers.slice(0, 6).map((person, index) => {
          const profile = safeWebUrl(person.profileUrl),
            work = safeWebUrl(person.workUrl);
          return (
            <article
              className="researcher-card"
              key={`${person.name}-${index}`}
            >
              <h3>{person.name}</h3>
              <p>{person.connection}</p>
              <div className="researcher-links">
                {profile && (
                  <ExternalLink href={profile}>Research profile</ExternalLink>
                )}
                {work ? (
                  <ExternalLink href={work}>{person.workTitle}</ExternalLink>
                ) : (
                  <span>{person.workTitle}</span>
                )}
              </div>
            </article>
          );
        })}
      </div>
      {context && (
        <details className="researcher-provenance">
          <summary>Editorial context</summary>
          <p>
            Updated {dateLabel(context.updatedAt)} by @{context.updatedBy.login}
            .
          </p>
          <p>{context.reason}</p>
        </details>
      )}
    </section>
  );
}

export function ResearcherEditor({
  initialChallenge = "",
  initialVersion = "",
}: {
  initialChallenge?: string;
  initialVersion?: string;
}) {
  const [input, setInput] = useState(initialChallenge);
  const [slug, setSlug] = useState(initialChallenge);
  const [expectedVersion, setExpectedVersion] = useState(initialVersion);
  const resource = useResource<Challenge>(
    slug ? `/challenges/${encodeURIComponent(slug)}` : null,
  );
  const challenge = resource.data;
  const mismatch = !!(
    challenge &&
    expectedVersion &&
    expectedVersion !== challenge.versionId
  );
  return (
    <section
      id="researcher-editor"
      className="panel researcher-editor"
      aria-labelledby="researcher-editor-title"
    >
      <h2 id="researcher-editor-title">Researcher context</h2>
      <p>
        Add researchers whose published work connects to the question. This
        public editorial context has its own history and does not change the
        frozen scientific contract.
      </p>
      <form
        className="researcher-selector"
        onSubmit={(event) => {
          event.preventDefault();
          const next = input.trim();
          setExpectedVersion("");
          if (slug === next) resource.refresh();
          else setSlug(next);
        }}
      >
        <Field
          label="Challenge slug"
          help="Open this editor from a challenge to select its version automatically."
        >
          <input
            required
            maxLength={200}
            value={input}
            onChange={(event) => setInput(event.target.value)}
          />
        </Field>
        <button className="button ghost" type="submit">
          Load challenge
        </button>
      </form>
      <ErrorMessage error={resource.error} retry={resource.refresh} />
      {resource.loading && slug && (
        <Loading label="Reading existing researcher context" />
      )}
      {mismatch ? (
        <p className="error-message" role="alert">
          This link refers to another challenge version. Open that version’s
          challenge page, or load this challenge again to select its current
          version.
        </p>
      ) : (
        challenge &&
        !resource.loading && (
          <ResearcherEditionForm
            key={`${challenge.versionId}:${challenge.researcherContext?.id || "none"}`}
            challenge={challenge}
          />
        )
      )}
    </section>
  );
}

const emptyResearcher = (): Researcher => ({
  name: "",
  profileUrl: "",
  connection: "",
  workTitle: "",
  workUrl: "",
});
function ResearcherEditionForm({ challenge }: { challenge: Challenge }) {
  const [rows, setRows] = useState<Researcher[]>(() =>
    (challenge.researcherContext?.researchers || []).map((row) => ({ ...row })),
  );
  const [reason, setReason] = useState("");
  const [saved, setSaved] = useState<ResearcherContext>();
  const [validation, setValidation] = useState("");
  const [done, setDone] = useState(false);
  const action = useAction();
  const edition = saved || challenge.researcherContext;
  function change(index: number, key: keyof Researcher, value: string) {
    setRows((current) =>
      current.map((row, i) => (i === index ? { ...row, [key]: value } : row)),
    );
    setDone(false);
    setValidation("");
  }
  return (
    <form
      className="researcher-edition-form"
      onSubmit={async (event) => {
        event.preventDefault();
        setDone(false);
        setValidation("");
        // Publish only the five supported public fields, never incidental API properties.
        const researchers = rows.map((row) => ({
          name: row.name.trim(),
          profileUrl: row.profileUrl.trim(),
          connection: row.connection.trim(),
          workTitle: row.workTitle.trim(),
          workUrl: row.workUrl.trim(),
        }));
        for (const person of researchers) {
          if (!person.name || !person.connection || !person.workTitle) {
            setValidation(
              "Complete every researcher’s name, connection and relevant work.",
            );
            return;
          }
          for (const value of [person.profileUrl, person.workUrl]) {
            try {
              const url = new URL(value);
              if (url.protocol !== "https:" || url.username || url.password)
                throw new Error();
            } catch {
              setValidation(
                "Use a public HTTPS link for each research profile and relevant publication.",
              );
              return;
            }
          }
        }
        if (reason.trim().length < 20) {
          setValidation(
            "Explain this public change in at least 20 characters.",
          );
          return;
        }
        const result = await action.run<ResearcherContext>(
          `/editor/challenge-versions/${encodeURIComponent(challenge.versionId)}/researchers`,
          { researchers, reason: reason.trim() },
        );
        if (result) {
          setSaved(result);
          setRows(result.researchers.map((row) => ({ ...row })));
          setReason("");
          setDone(true);
        }
      }}
    >
      <div className="researcher-edit-target">
        <h3>{challenge.title}</h3>
        <span className="mono">Version {challenge.versionId}</span>
        {edition && (
          <span>
            Last updated {dateLabel(edition.updatedAt)} by @
            {edition.updatedBy.login}
          </span>
        )}
      </div>
      <p className="researcher-notice">{RESEARCHER_NOTICE}</p>
      <fieldset className="researcher-editor-fields" disabled={action.busy}>
        <legend className="sr-only">
          Researchers listed in the public context
        </legend>
        {!rows.length && (
          <p className="subtle">
            No researchers listed. Saving an empty list removes this section
            from the public page.
          </p>
        )}
        {rows.map((row, index) => (
          <fieldset className="researcher-fields" key={index}>
            <legend>Researcher {index + 1}</legend>
            <div className="researcher-field-grid">
              <Field label={`Researcher ${index + 1} name`}>
                <input
                  required
                  maxLength={120}
                  value={row.name}
                  onChange={(event) =>
                    change(index, "name", event.target.value)
                  }
                />
              </Field>
              <Field
                label={`Researcher ${index + 1} profile URL`}
                help="Public HTTPS profile"
              >
                <input
                  type="url"
                  required
                  maxLength={2048}
                  value={row.profileUrl}
                  onChange={(event) =>
                    change(index, "profileUrl", event.target.value)
                  }
                />
              </Field>
            </div>
            <Field label={`Researcher ${index + 1} connection to the question`}>
              <textarea
                rows={2}
                required
                maxLength={1000}
                value={row.connection}
                onChange={(event) =>
                  change(index, "connection", event.target.value)
                }
              />
            </Field>
            <div className="researcher-field-grid">
              <Field label={`Researcher ${index + 1} relevant work title`}>
                <input
                  required
                  maxLength={300}
                  value={row.workTitle}
                  onChange={(event) =>
                    change(index, "workTitle", event.target.value)
                  }
                />
              </Field>
              <Field
                label={`Researcher ${index + 1} relevant work URL`}
                help="Public HTTPS publication"
              >
                <input
                  type="url"
                  required
                  maxLength={2048}
                  value={row.workUrl}
                  onChange={(event) =>
                    change(index, "workUrl", event.target.value)
                  }
                />
              </Field>
            </div>
            <button
              type="button"
              className="button small ghost"
              aria-label={`Remove researcher ${index + 1}`}
              onClick={() => {
                setRows((current) => current.filter((_, i) => i !== index));
                setDone(false);
              }}
            >
              <Trash2 size={14} />
              Remove researcher
            </button>
          </fieldset>
        ))}
        <div className="researcher-add-row">
          <button
            type="button"
            className="button small ghost"
            disabled={rows.length >= 6}
            onClick={() => {
              setRows((current) => [...current, emptyResearcher()]);
              setDone(false);
            }}
          >
            <Plus size={15} />
            Add researcher
          </button>
          <span className="subtle">{rows.length} of 6 entries</span>
        </div>
        <Field label="Reason for this change (public)">
          <textarea
            rows={3}
            required
            minLength={20}
            maxLength={2000}
            value={reason}
            onChange={(event) => {
              setReason(event.target.value);
              setDone(false);
            }}
          />
        </Field>
        {validation && (
          <p role="alert" className="error-message">
            {validation}
          </p>
        )}
        <ErrorMessage error={action.error} />
        <button type="submit" className="button ghost">
          <Save size={16} />
          {action.busy ? "Saving…" : "Save researcher context"}
        </button>
      </fieldset>
      {done && (
        <p className="success-note" role="status">
          <Check size={16} />
          Researcher context saved. Earlier editions remain in the public
          history.
        </p>
      )}
    </form>
  );
}
