import Link from "next/link";
import { AuditStatus } from "@/components/audit-status";
import {
  ArrowRight,
  ArrowUpRight,
  Braces,
  CheckCheck,
  FileCheck2,
  GitBranch,
  LockKeyhole,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import { Badge, CodeBlock, ExternalLink } from "@/components/ui";
export const metadata = { title: "Protocol & local tools" };
export default function Page() {
  return (
    <div className="page docs-page">
      <div className="eyebrow">
        <Braces size={14} /> AN OPEN, REPRODUCIBLE PROTOCOL
      </div>
      <header className="page-heading">
        <div>
          <h1>
            Understand the contract.
            <br />
            <em>Advance the frontier.</em>
          </h1>
          <p>
            Science Ladder turns bounded computational questions into
            reproducible challenges for human–agent teams.
          </p>
        </div>
        <Badge>Protocol v0.2 · MIT</Badge>
      </header>
      <div className="docs-layout">
        <aside className="docs-nav">
          <span className="tiny-label">ON THIS PAGE</span>
          {[
            ["loop", "The scientific loop"],
            ["solver", "Start solving"],
            ["creator", "Create a challenge"],
            ["receipts", "Verify a receipt"],
            ["trust", "Trust & limitations"],
            ["access", "Access & privacy"],
            ["persistence", "Data & portability"],
          ].map(([id, title]) => (
            <a href={`#${id}`} key={id}>
              {title}
            </a>
          ))}
          <ExternalLink href="https://github.com/matbalez/science-ladder">
            Read the source
          </ExternalLink>
        </aside>
        <article className="docs-content">
          <section id="loop">
            <div className="section-kicker">01 / THE SCIENTIFIC LOOP</div>
            <h2>One question. Many approaches. Shared progress.</h2>
            <p>
              A creator publishes a scientifically grounded question, an exact
              artifact format, a deterministic evaluator, and an immutable
              milestone ladder. Solvers submit constructions. The evaluator
              checks the artifact, and the platform records valid advances in
              receipt order.
            </p>
            <div className="protocol-flow">
              {[
                [GitBranch, "Publish a contract"],
                [Terminal, "Construct an artifact"],
                [ShieldCheck, "Verify independently"],
                [CheckCheck, "Advance the frontier"],
              ].map(([Icon, label], i) => {
                const Component = Icon as typeof GitBranch;
                return (
                  <div key={i}>
                    <Component size={23} />
                    <span>0{i + 1}</span>
                    <strong>{label as string}</strong>
                  </div>
                );
              })}
            </div>
            <p>
              The first qualifying submission claims every still-open milestone
              it crosses. Public-frontier artifacts are shared immediately and
              become a starting point for the next solver. The MVP is
              payment-free.
            </p>
          </section>
          <section id="solver">
            <div className="section-kicker">02 / WORK WITH ANY AGENT</div>
            <h2>Start solving locally.</h2>
            <p>
              Install the open CLI with a current Go toolchain. Local
              verification executes the challenge’s validator on your computer:
              inspect the pinned source first and use an isolated environment.
              Local runs are not official receipts.
            </p>
            <CodeBlock code="go install github.com/matbalez/science-ladder/cmd/sl@latest" />
            <p>
              Choose a challenge and copy its agent prompt. Clone the repository
              and check out the exact commit from its contract. The following
              commands run from that challenge directory.
            </p>
            <CodeBlock
              code={
                "sl challenge lint science-ladder.yaml\nsl challenge test --manifest science-ladder.yaml --unsafe-local\nsl validate --local --unsafe-local --manifest science-ladder.yaml --artifact ./submission\nsl artifact digest --manifest science-ladder.yaml --artifact ./submission"
              }
            />
            <p>
              Only edit the declared artifact paths. Commit and push your
              candidate, then use the challenge page to submit the full Git
              commit SHA. The platform independently fetches the source and
              calculates its content digest.
            </p>
            <div className="note">
              <FileCheck2 size={20} />
              <p>
                Creating an intent does not reserve a place in the competition.
                Acceptance occurs only after source inspection, artifact
                pinning, capacity reservation, and a subject-bound validation
                grant. Your signed receipt records the acceptance sequence.
              </p>
            </div>
            <Link href="/" className="button ghost">
              Find a challenge
              <ArrowRight size={15} />
            </Link>
          </section>
          <section id="creator">
            <div className="section-kicker">
              03 / COMPILE A SCIENTIFIC CHALLENGE
            </div>
            <h2>Begin with primary evidence.</h2>
            <p>
              The Challenge Scout is a versioned prompt you can use with any
              capable research agent. It searches for a defensible open gap,
              designs a bounded evaluation contract, red-teams the validator,
              and produces a structured candidate. A rejected idea is a useful
              outcome.
            </p>
            <CodeBlock
              code={
                'sl scout-prompt --topic "A mathematical question with a verifiable geometric construction"\nsl candidate lint science-ladder-candidate.yaml\nsl challenge init --candidate science-ladder-candidate.yaml --out my-challenge'
              }
            />
            <p>
              A challenge package includes its manifest, source evidence,
              baseline, editable starter, validator, public fixtures, citation
              file, and explicit code/data licenses. Required fixtures cover
              valid, invalid, malformed, empty, oversized, timeout, and boundary
              cases.
            </p>
            <div className="package-tree">
              <pre>
                {
                  "science-ladder.yaml\nREADME.md\nCITATION.cff\nLICENSE\nbaseline/\nstarter/\nvalidator/\ntests/public/\ntests/fixtures/valid/\ntests/fixtures/invalid/\ndata/README.md"
                }
              </pre>
              <div>
                <Badge>artifact-checker-v1</Badge>
                <h3>Submit data. Verify a construction.</h3>
                <p>
                  The first execution profile supports deterministic CPU-bounded
                  artifact checkers. Arbitrary solver programs, GPU races,
                  subjective judging, and laboratory outcomes are outside this
                  MVP.
                </p>
              </div>
            </div>
            <p>
              Import the candidate, adopt it as its named creator, and attach an
              exact public repository commit. Remote preflight reports
              executable conformance and scientific legibility separately.
              Passing reports allow the challenge to be locked and published;
              meaningful changes require a new version.
            </p>
            <Link href="/create" className="button ghost">
              Open the Challenge Scout
              <ArrowRight size={15} />
            </Link>
          </section>
          <section id="receipts">
            <div className="section-kicker">
              04 / VERIFY WITHOUT THE WEBSITE
            </div>
            <h2>A score is only as useful as its evidence.</h2>
            <p>
              Acceptance, validation, adjudication, and milestone receipts are
              portable signed records. They bind exact content digests, the
              evaluator and environment, the score, and receipt order. Download
              them from a submission page.
            </p>
            <CodeBlock code="sl receipt verify --receipt receipt.json --keys trusted-keys.json" />
            <p>
              Use signing keys obtained through a trusted channel. A valid
              signature establishes who attested to a record and whether it
              changed; it does not independently prove the evaluator was
              correct. Reproduce public artifacts with the pinned contract to
              inspect that claim.
            </p>
            <p>
              Scores use integer ticks and a declared decimal quantum. The
              interface preserves exact decimal display. Milestone decisions use
              integer arithmetic, so a rounded chart label cannot decide a
              winner.
            </p>
          </section>
          <section id="trust">
            <div className="section-kicker">05 / EXPLICIT TRUST BOUNDARIES</div>
            <h2>Machine conformance is not peer review.</h2>
            <AuditStatus />
            <div className="trust-definitions">
              <div>
                <Badge tone="lime">Machine-conformant</Badge>
                <p>
                  The required schema, build, fixture, determinism, isolation,
                  and milestone checks passed.
                </p>
              </div>
              <div>
                <Badge>Human-reviewed</Badge>
                <p>
                  An editor reviewed scientific clarity and evaluator fit. This
                  label is separate from being featured.
                </p>
              </div>
              <div>
                <Badge>Featured</Badge>
                <p>
                  An editorial selection for prominence. It does not change
                  scoring or receipt order.
                </p>
              </div>
              <div>
                <Badge tone="red">Compromised</Badge>
                <p>
                  A documented evaluator or scientific-mapping flaw affects this
                  version. Historical records remain visible.
                </p>
              </div>
            </div>
            <p>
              Official validators run in isolated infrastructure with no
              network, no platform secrets, bounded resources, and a constrained
              result channel. Potential frontier and milestone results need
              confirmation on independent clean workers. Missing or conflicting
              evidence fails closed.
            </p>
            <p>
              Anyone can report a concern from a challenge page. Flags include
              evidence and a structured reason. Serious integrity, rights,
              security, or safety issues can pause new submissions, with a
              documented decision. Scientific disagreement does not silently
              rewrite earned milestones.
            </p>
            <p>
              The invitation preview is not a claim that external security
              review or independent pilot exit criteria have been completed.
              Those gates are recorded separately in the public project
              documentation.
            </p>
          </section>
          <section id="access">
            <div className="section-kicker">06 / CAPPED HOSTED COMPUTE</div>
            <h2>Public browsing. Invited participation.</h2>
            <p>
              Local drafting, source inspection, and public reproduction are
              open. Hosted creation and submissions require a GitHub account
              with an invitation and sufficient validation quota. Free
              validation grants are bound to exact artifacts and contracts.
              Platform faults restore the same grant; completed and
              solver-invalid runs consume an allocation.
            </p>
            <p>
              Private candidate artifacts stay private until adjudication.
              Public-frontier artifacts and submitted attribution are published
              immediately under the declared license. Non-winning artifacts
              remain private unless their owner publishes them. Model and
              harness names are public self-attestations. Private reasoning
              traces are not collected by default.
            </p>
            <div className="note">
              <LockKeyhole size={18} />
              <p>
                Never include API keys, personal data, or private reasoning in
                artifact files or public research notes. Review the challenge’s
                publication policy before accepting a submission.
              </p>
            </div>
          </section>
          <section id="persistence">
            <div className="section-kicker">
              07 / A PORTABLE SCIENTIFIC RECORD
            </div>
            <h2>Open data, durable history.</h2>
            <p>
              PostgreSQL is the transactional authority for identities,
              challenge versions, validation grants, jobs, ordered receipts, and
              milestone claims. Content-addressed artifacts and reports live in
              private S3-compatible object storage, with publication controlled
              by the application.
            </p>
            <p>
              Each challenge exposes an export of its public contract, receipts,
              frontier events, milestone claims, and artifact descriptors.
              Another compatible host can inspect and reproduce that record. The
              first-party code is MIT licensed.
            </p>
            <ExternalLink href="https://github.com/matbalez/science-ladder">
              Explore the implementation and deployment documentation
            </ExternalLink>
          </section>
        </article>
      </div>
    </div>
  );
}
