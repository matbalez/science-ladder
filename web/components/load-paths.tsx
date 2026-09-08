"use client";
import { useEffect, useState } from "react";
import styles from "./load-paths.module.css";

type Geometry = {
  id: string;
  nx: number;
  ny: number;
  physicalDensity: number[][];
  displacements: number[][];
  compliances: number[];
  volume: number;
  loads: { y: number; fx: number; fy: number }[];
};
export function LoadPathsExplorer() {
  const [cases, setCases] = useState<Geometry[]>([]);
  const [error, setError] = useState(false);
  const [geometry, setGeometry] = useState(0);
  const [load, setLoad] = useState(0);
  const [deformation, setDeformation] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    fetch("/showcase/load-paths/reference.json", { signal: abort.signal })
      .then((r) => {
        if (!r.ok) throw Error();
        return r.json();
      })
      .then((d) => setCases(d.cases))
      .catch(() => {
        if (!abort.signal.aborted) setError(true);
      });
    return () => abort.abort();
  }, []);
  const c = cases[geometry];
  if (!c)
    return (
      <section className={styles.panel}>
        <p>
          {error
            ? "The reference illustration could not be loaded."
            : "Loading reference design…"}
        </p>
      </section>
    );
  const maxMove = Math.max(
    ...c.displacements.map((v) => Math.abs(v[load])),
    1e-20,
  );
  const factor = (deformation * 0.04) / maxMove;
  const point = (x: number, y: number) => {
    const node = y * (c.nx + 1) + x;
    return `${x + 4 + factor * c.displacements[2 * node][load]},${c.ny - y + 5 - factor * c.displacements[2 * node + 1][load]}`;
  };
  const force = c.loads[load];
  return (
    <section
      className={styles.panel}
      aria-label="Load Paths reference explorer"
    >
      <div className={styles.heading}>
        <div>
          <h2>Where the material goes</h2>
          <p>One design must resist all three loads.</p>
        </div>
        <div className={styles.choices} role="group" aria-label="Geometry">
          {cases.map((v, i) => (
            <button
              key={v.id}
              aria-pressed={geometry === i}
              onClick={() => setGeometry(i)}
            >
              {v.id.replace("cantilever-", "")}
            </button>
          ))}
        </div>
      </div>
      <svg
        className={styles.drawing}
        viewBox={`0 0 ${c.nx + 14} ${c.ny + 12}`}
        role="img"
        aria-label={`Reference ${c.id.replace("cantilever-", "")} cantilever. Darker cells contain more material. Load ${load + 1}; ${deformation ? "illustrative displacement shown" : "undeformed"}.`}
      >
        <defs>
          <pattern
            id="load-path-support"
            width=".8"
            height=".8"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(45)"
          >
            <line
              x1="0"
              y1="0"
              x2="0"
              y2=".8"
              stroke="#756957"
              strokeWidth=".18"
            />
          </pattern>
          <marker
            id="load-path-arrow"
            markerWidth="4"
            markerHeight="4"
            refX="3"
            refY="2"
            orient="auto"
          >
            <path d="M0 0L4 2L0 4Z" fill="#a44422" />
          </marker>
        </defs>
        <rect
          x="2.8"
          y="5"
          width="1.2"
          height={c.ny}
          fill="url(#load-path-support)"
        />
        <rect x="4" y="5" width={c.nx} height={c.ny} fill="#e8e3d8" />
        {c.physicalDensity.flatMap((row, y) =>
          row.map((density, x) => (
            <polygon
              key={`${x}-${y}`}
              points={[
                point(x, y),
                point(x + 1, y),
                point(x + 1, y + 1),
                point(x, y + 1),
              ].join(" ")}
              fill={`rgb(${Math.round(235 - density * 199)},${Math.round(229 - density * 190)},${Math.round(216 - density * 181)})`}
            />
          )),
        )}
        <line
          x1={c.nx + 5}
          y1={c.ny - force.y + 5}
          x2={c.nx + 5 + force.fx * 3}
          y2={c.ny - force.y + 5 - force.fy * 3}
          stroke="#a44422"
          strokeWidth=".22"
          markerEnd="url(#load-path-arrow)"
        />
      </svg>
      <div className={styles.controls}>
        <label>
          Load
          <select
            value={load}
            onChange={(e) => setLoad(Number(e.target.value))}
          >
            {c.loads.map((_, i) => (
              <option key={i} value={i}>
                Load {i + 1}
              </option>
            ))}
          </select>
        </label>
        <label>
          Show displacement
          <input
            aria-label="Illustrative displacement"
            type="range"
            min="0"
            max="100"
            value={deformation}
            onChange={(e) => setDeformation(Number(e.target.value))}
          />
        </label>
        <div>
          <span>Compliance · lower is stiffer</span>
          <strong>{c.compliances[load].toFixed(2)}</strong>
        </div>
        <div>
          <span>Material used</span>
          <strong>{(c.volume * 100).toFixed(2)}%</strong>
        </div>
      </div>
      <p className={styles.caption}>
        Frozen reference design. Darker cells contain more material.
        Displacement is rescaled for illustration; the score uses the unchanged
        linear-elastic calculation.
      </p>
    </section>
  );
}
