"use client";
import { useEffect, useMemo, useState } from "react";
import { densityContours, densityColor } from "@/lib/density-contours";
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
  const [mesh, setMesh] = useState(false);
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
  const contours = useMemo(
    () => (c ? densityContours(c.physicalDensity) : []),
    [c],
  );
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
    const ix = Math.min(Math.floor(x), c.nx - 1),
      iy = Math.min(Math.floor(y), c.ny - 1);
    const tx = x - ix,
      ty = y - iy;
    const displacement = (axis: number) =>
      [
        [ix, iy, (1 - tx) * (1 - ty)],
        [ix + 1, iy, tx * (1 - ty)],
        [ix + 1, iy + 1, tx * ty],
        [ix, iy + 1, (1 - tx) * ty],
      ].reduce(
        (v, [xx, yy, w]) =>
          v + w * c.displacements[2 * (yy * (c.nx + 1) + xx) + axis][load],
        0,
      );
    return `${(x + 4 + factor * displacement(0)).toFixed(4)},${(c.ny - y + 5 - factor * displacement(1)).toFixed(4)}`;
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
        viewBox={`1 3 ${c.nx + 8} ${c.ny + 7}`}
        role="img"
        aria-label={`Reference ${c.id.replace("cantilever-", "")} cantilever. Darker regions contain more material. Load ${load + 1}; ${deformation ? "illustrative displacement shown" : "undeformed"}.`}
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
        {!mesh &&
          contours.map(({ level, polygons }) => (
            <path
              key={level}
              data-density-contour={level}
              d={polygons
                .map(
                  (vertices) =>
                    `M${vertices.map(([x, y]) => point(x, y)).join("L")}Z`,
                )
                .join("")}
              fill={densityColor(level)}
            />
          ))}
        {mesh &&
          c.physicalDensity.flatMap((row, y) =>
            row.map((density, x) => (
              <polygon
                key={`${x}-${y}`}
                points={[
                  point(x, y),
                  point(x + 1, y),
                  point(x + 1, y + 1),
                  point(x, y + 1),
                ].join(" ")}
                fill={densityColor(density)}
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
      <div className={styles.legend}>
        <span>Less material</span>
        <span className={styles.scale} />
        <span>More material</span>
      </div>
      <div className={styles.controls}>
        <label>
          Display
          <select
            aria-label="Density display"
            value={mesh ? "mesh" : "contours"}
            onChange={(e) => setMesh(e.target.value === "mesh")}
          >
            <option value="contours">Density contours</option>
            <option value="mesh">Simulation cells</option>
          </select>
        </label>
        <label>
          Load
          <select
            aria-label="Load"
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
        Frozen reference design · {c.nx} × {c.ny} simulation cells.
        {mesh
          ? " Showing the exact cell densities."
          : " Contours interpolate density between cells for readability; they are not a finer simulation or a manufactured boundary."}{" "}
        Displacement is rescaled for illustration; the score uses the unchanged
        linear-elastic calculation.
      </p>
    </section>
  );
}
