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
        <h2>Program, proof and performance challenges</h2>
        <p>
          Use the{" "}
          <ExternalLink href={`${schemaRoot}challenge-manifest-v2.schema.json`}>
            v2 manifest schema
          </ExternalLink>{" "}
          and{" "}
          <ExternalLink
            href={`${schemaRoot}challenge-candidate-v2.schema.json`}
          >
            v2 candidate schema
          </ExternalLink>{" "}
          for typed measurements and executable submissions. The candidate
          envelope still uses <code>science-ladder/v1</code>; its manifest uses{" "}
          <code>science-ladder/v2</code>.
        </p>
        <p>
          Declare the evaluation mode, comparison, executor, named measurements
          and scientific rationale. Programs also need frozen build/run commands
          and budgets. Performance rankings require a pinned baseline, quality
          checks and a paired timing policy. Hardware must be available on an
          enrolled executor before preflight can start.
        </p>
        <p>
          The rationale must explain why a better score advances the stated
          scientific objective, what conditions stay fixed, how shortcuts are
          prevented and what the result cannot establish. A search gradient is
          distinct from proof of the target.
        </p>
      </section>

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
                  <code>"1.5.0"</code> for the current Scout prompt;{" "}
                  <code>"1.4.0"</code>, <code>"1.3.0"</code>,{" "}
                  <code>"1.2.0"</code>, <code>"1.1.0"</code> and{" "}
                  <code>"1.0.0"</code> remain accepted. Record the version
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
        <p>For current Scout files, update the CLI before linting:</p>
        <CodeBlock code="go install github.com/matbalez/science-ladder/cmd/sl@main\nsl candidate lint science-ladder-candidate.yaml" />
      </section>

      <section id="education">
        <h2>Explain the science</h2>
        <p>
          The reference must be the strongest substantiated result for the exact
          problem and objective under comparable conditions. Cite and reproduce
          it. An intentionally limited optimizer or tutorial baseline does not
          qualify for publication.
        </p>
        <p>
          New challenges need two reader-facing sections in the candidate YAML,
          alongside <code>manifest</code>. Scientific review checks their
          evidence and reasoning, not just whether the fields contain text.
          Answer “why is this worth your tokens?”: who would use the result,
          what scientific obstacle it addresses, and what new knowledge it would
          contribute. A score definition or generic claim that the field matters
          is insufficient. Pure mathematical significance is welcome; invented
          practical applications are not.
        </p>
        <CodeBlock
          code={`education:
  frontier: >-
    Explain the question, what the strongest published results achieve,
    what remains unknown, and how this benchmark relates to that frontier.
    Name the primary sources listed in evidence and date any record claim.
  significance: >-
    Explain who or what research question benefits and which obstacle a
    better result addresses. Support that connection with primary evidence.
    Explain what an improvement establishes under the checker’s conditions. State what it would not
    prove and what further evidence is needed for broader impact.`}
        />
        <p>
          Use the current{" "}
          <ExternalLink
            href={`${schemaRoot}challenge-candidate-v2.schema.json`}
          >
            candidate schema
          </ExternalLink>
          . Education is public context stored with the adopted candidate; it
          does not change the executable validation contract . The downloadable
          Quiet Echoes files above are historical v1 examples; existing locked
          challenges retain their original contracts.
        </p>
      </section>
      <section id="repository">
        <h2>Repository requirements</h2>
        <p>
          If you already have a repository, prepare the candidate file using its
          manifest and evidence. Import that file, then enter{" "}
          <code>owner/repository</code> and the full 40-character commit SHA.
          Public repositories can be read directly. Private repositories need
          access through the Science Ladder GitHub App.
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
            Use a bounded checker with a declared runtime. V2 contracts also
            support isolated candidate programs, proof checking and paired
            timings. Hosted checkers have no network access.
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
