"use client";
import { useLearningView } from "./challenge-learning";

export function AdditionsVisual() {
  useLearningView("addition circuit", {
    description:
      "The reference uses 13 additions on A, 14 on B, 23 ordered products, and 28 additions to assemble C: 55 additions total. The pictured branch is u5=u4−A8, u6=u5−A1, u7=u5+A7. Each equation costs one operation, with u5 computed once and reused. This is a schematic, not the full circuit. The fixed oriented tensor is already addition-optimal; the challenge allows different decompositions and orientations.",
    stageCounts: { left: 13, right: 14, output: 28 },
    products: 23,
    referenceAdditions: 55,
    sources: ["https://arxiv.org/abs/2607.28676"],
  });
  return (
    <figure className="additions-visual" aria-label="Addition sharing diagram">
      {/* The committed SVG is served as an image, never injected into the DOM. */}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        className="additions-desktop"
        src="/showcase/fewer-additions/science.svg"
        width="900"
        height="550"
        alt="Reference operation diagram: 13 additions on A, 14 on B and 28 on the output, with 23 products. The partial sum u5 is reused by u6 and u7."
      />
      <div className="additions-mobile">
        <h2>Where the additions go</h2>
        <div className="addition-stages">
          <div>
            <strong>Prepare A: 13</strong>
            <span>9 entries → 23 factors</span>
          </div>
          <div>
            <strong>Prepare B: 14</strong>
            <span>9 entries → 23 factors</span>
          </div>
          <div>
            <strong>23 ordered products</strong>
            <span>A factor × B factor</span>
          </div>
          <div>
            <strong>Assemble C: 28</strong>
            <span>23 products → 9 entries</span>
          </div>
        </div>
        <p>
          <strong>13 + 14 + 28 = 55 additions</strong>
        </p>
        <h3>Compute once. Reuse.</h3>
        <p>An actual branch of the reference:</p>
        <div className="addition-shared">u₅ = u₄ − A₈</div>
        <div className="addition-branches">
          <span>↳ u₆ = u₅ − A₁</span>
          <span>↳ u₇ = u₅ + A₇</span>
        </div>
        <p>
          Each equation costs one addition/subtraction. The shared u₅ is
          computed once.
        </p>
      </div>
      <figcaption>
        The reference shares intermediate sums across 23 products. The full
        circuit is in the repository.{" "}
        <a
          href="https://arxiv.org/abs/2607.28676"
          target="_blank"
          rel="noreferrer"
        >
          Karunaratne &amp; Idamekorala, 2026
        </a>
        .
      </figcaption>
    </figure>
  );
}
