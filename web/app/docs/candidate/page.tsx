import Link from "next/link";
import { CodeBlock, ExternalLink } from "@/components/ui";
import styles from "./candidate.module.css";

export const metadata = { title: "Candidate YAML" };
const schemaRoot =
  "https://github.com/matbalez/science-ladder/blob/main/protocol/schemas/";
const exampleRoot =
  "https://github.com/matbalez/science-ladder-quiet-echoes/tree/f42f527e97563b1c068a1835732c6da44f21223f";

export default function Page() {
  return (
    <div className={`page ${styles.page}`}>
      <Link href="/create?path=import" className={styles.back}>
        ← Import a candidate
      </Link>
      <header className="page-heading">
        <h1>Candidate YAML</h1>
      </header>
      <p>
        Creation uses two files. Import the proposal first, then attach its
        repository.
      </p>
      <div className={styles.tableWrap}>
        <table>
          <thead>
            <tr>
              <th>File</th>
              <th>Purpose</th>
              <th>Reference</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>
                <code>science-ladder-candidate.yaml</code>
              </td>
              <td>
                The proposal: sources, uncertainties, provenance and proposed
                checker contract. Upload this on the creation page.
              </td>
              <td>
                <a
                  href="/examples/quiet-echoes-candidate.yaml"
                  download="science-ladder-candidate.yaml"
                >
                  Download example
                </a>
                <ExternalLink
                  href={`${schemaRoot}challenge-candidate-v1.schema.json`}
                >
                  Candidate schema
                </ExternalLink>
              </td>
            </tr>
            <tr>
              <td>
                <code>science-ladder.yaml</code>
              </td>
              <td>
                The checker contract. Commit it at the repository root with the
                checker, baseline and fixtures.
              </td>
              <td>
                <a
                  href="/examples/quiet-echoes-manifest.yaml"
                  download="science-ladder.yaml"
                >
                  Download example
                </a>
                <ExternalLink
                  href={`${schemaRoot}challenge-manifest-v1.schema.json`}
                >
                  Manifest schema
                </ExternalLink>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p className={styles.note}>
        Examples come from{" "}
        <a href={exampleRoot} target="_blank" rel="noreferrer">
          Quiet Echoes
        </a>
        . Replace its research, scores, authorship and licenses with your own
        challenge’s details.
      </p>

      <section>
        <h2>Required candidate fields</h2>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Field</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <code>apiVersion</code>
                </td>
                <td>
                  <code>science-ladder/v1</code>
                </td>
              </tr>
              <tr>
                <td>
                  <code>kind</code>
                </td>
                <td>
                  <code>ChallengeCandidate</code>
                </td>
              </tr>
              <tr>
                <td>
                  <code>id</code>, <code>createdAt</code>, <code>producer</code>
                </td>
                <td>
                  Candidate identifier, quoted ISO timestamp and creator or
                  agent attribution.
                </td>
              </tr>
              <tr>
                <td>
                  <code>promptVersion</code>
                </td>
                <td>
                  <code>"1.1.0"</code> for the current Scout prompt;{" "}
                  <code>"1.0.0"</code> is also accepted. Record the version
                  actually used.
                </td>
              </tr>
              <tr>
                <td>
                  <code>disposition</code>
                </td>
                <td>
                  <code>viable</code>, <code>needs_work</code> or{" "}
                  <code>rejected</code>. Only a viable candidate can become a
                  challenge.
                </td>
              </tr>
              <tr>
                <td>
                  <code>sources</code>
                </td>
                <td>
                  A list of primary sources. Each has <code>url</code> (HTTPS),{" "}
                  <code>title</code>, <code>evidence</code> and{" "}
                  <code>location</code> (section, figure or table).
                </td>
              </tr>
              <tr>
                <td>
                  <code>uncertainties</code>, <code>rejectedAlternatives</code>,{" "}
                  <code>repositoryPlan</code>
                </td>
                <td>
                  Lists recording unresolved questions, alternatives considered
                  and the planned repository files.
                </td>
              </tr>
              <tr>
                <td>
                  <code>manifest</code>
                </td>
                <td>
                  Required when <code>disposition: viable</code>. Embed the
                  complete challenge manifest as an object, not a filename.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>
          <code>model</code> is optional. Unknown fields are rejected. Quote
          timestamps, prompt versions, score ticks and the metric quantum so
          YAML preserves their types.
        </p>
        <CodeBlock code="sl candidate lint science-ladder-candidate.yaml" />
      </section>

      <section id="repository">
        <h2>Repository requirements</h2>
        <p>
          If you already have a repository, prepare the candidate file using its
          manifest and evidence. Import that file, then enter{" "}
          <code>owner/repository</code> and the full 40-character commit SHA.
          The GitHub App needs access to that repository.
        </p>
        <p>
          The manifest specifies the question and evidence, score and baseline,
          hard gates, milestones, deadline, submission paths and limits,
          licenses, checker command, runtime digest, resource limits, suite and
          fixtures. See the manifest schema above for every field.
        </p>
        <ul>
          <li>
            Include the checker, dependency lock, suite, baseline and every
            fixture named in the manifest. Baseline, valid, invalid and
            malformed fixtures are required; test boundary and resource failures
            too.
          </li>
          <li>
            Use a deterministic Python checker with bounded CPU and memory.
            Submissions contain data; hosted checkers have no network access.
          </li>
          <li>
            Keep the proposed <code>manifest</code> consistent with the
            repository’s <code>science-ladder.yaml</code>. Record source
            attribution, code/data licenses and limitations.
          </li>
        </ul>
        <CodeBlock code="sl challenge lint science-ladder.yaml" />
        <details className={styles.details}>
          <summary>Starting a new repository</summary>
          <p>
            This creates a new directory with a deliberately failing draft
            checker. Implement the checker and fixtures before attaching the
            repository.
          </p>
          <CodeBlock code="sl challenge init --candidate science-ladder-candidate.yaml --out my-challenge" />
        </details>
        <p>
          Schema validation checks the file’s structure. Publication also
          requires hosted verification and scientific review.
        </p>
      </section>
      <Link href="/create?path=import" className="button primary">
        Import a candidate
      </Link>
    </div>
  );
}
