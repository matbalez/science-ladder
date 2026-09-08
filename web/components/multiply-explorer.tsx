"use client";
import { useLearningView } from "./challenge-learning";
import { useState } from "react";
import { MULTIPLY_REFERENCE as reference } from "@/lib/multiply-reference";

function Matrix({
  values,
  weights,
  label,
}: {
  values: number[];
  weights: number[];
  label: string;
}) {
  return (
    <div className="multiply-matrix-wrap">
      <h3>{label}</h3>
      <div
        className="multiply-matrix"
        role="img"
        aria-label={`${label}: ${values.join(", ")}`}
      >
        {values.map((value, i) => (
          <span key={i} className={weights[i] ? "active" : ""}>
            {value}
            <small>
              {weights[i] === 1 ? "+" : weights[i] === -1 ? "−" : ""}
            </small>
          </span>
        ))}
      </div>
    </div>
  );
}

export function MultiplyExplorer() {
  const [term, setTerm] = useState(0);
  const a = [1, 2, 3, 4, 5, 6, 7, 8, 9];
  const b = [9, 8, 7, 6, 5, 4, 3, 2, 1];
  const { U, V, W } = reference;
  const dot = (x: number[], y: number[]) =>
    x.reduce((sum, value, i) => sum + value * y[i], 0);
  const left = dot(U[term], a),
    right = dot(V[term], b);
  const answer = a.map((_, i) =>
    [0, 1, 2].reduce(
      (sum, k) => sum + a[3 * Math.floor(i / 3) + k] * b[3 * k + (i % 3)],
      0,
    ),
  );
  const contribution = W[term].map((value) => value * left * right);
  useLearningView("matrix multiplication", {
    description:
      "Green entries and signs select coefficients for the current bilinear product; output highlights show where that product is added or subtracted. The final answer matrix shows the sum of all 23 products, not just this contribution.",
    selectedProduct: term + 1,
    totalProducts: 23,
    A: a,
    B: b,
    answer,
    leftCoefficients: U[term],
    rightCoefficients: V[term],
    outputCoefficients: W[term],
    leftSum: left,
    rightSum: right,
    product: left * right,
    contribution,
  });
  return (
    <section
      className="multiply-explorer"
      aria-label="Matrix multiplication explorer"
    >
      <div className="multiply-header">
        <div>
          <h2>One product can help several answers</h2>
          <p>
            Add and subtract entries, multiply once, then distribute the result.
          </p>
        </div>
        <div className="multiply-counts">
          <span>
            Usual recipe<strong>27</strong>
          </span>
          <span>
            Reference<strong>23</strong>
          </span>
          <span>
            Target<strong>22</strong>
          </span>
        </div>
      </div>
      <div className="multiply-matrices">
        <Matrix label="A" values={a} weights={U[term]} />
        <span className="multiply-symbol" aria-hidden="true">
          ×
        </span>
        <Matrix label="B" values={b} weights={V[term]} />
        <span className="multiply-symbol" aria-hidden="true">
          =
        </span>
        <Matrix label="A × B" values={answer} weights={W[term]} />
      </div>
      <label className="multiply-slider">
        Product {term + 1} of 23
        <input
          type="range"
          aria-label="Reference product"
          min="0"
          max="22"
          value={term}
          onChange={(e) => setTerm(Number(e.target.value))}
        />
      </label>
      <p className="multiply-equation" aria-live="polite">
        {left} × {right} = {left * right}
      </p>
      <p>
        Highlighted signs select the input entries and show where this product
        is added or subtracted in the answer. Sum all 23 contributions to
        recover A × B.
      </p>
      <details>
        <summary>This product’s contribution</summary>
        <p className="mono">
          {contribution.slice(0, 3).join(", ")}
          <br />
          {contribution.slice(3, 6).join(", ")}
          <br />
          {contribution.slice(6).join(", ")}
        </p>
        <p>
          <a href="/attributions/matrix-reference.txt">
            Reference attribution and MIT notices
          </a>
        </p>
      </details>
      <p className="subtle">
        These numbers illustrate the reference. Verification proves the formula
        for every input by checking all 729 coefficients exactly.
      </p>
    </section>
  );
}
