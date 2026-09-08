"use client";
import { useLearningView } from "./challenge-learning";
import { useMemo, useState } from "react";
import { TRIANGLE_POINTS } from "@/lib/triangle-reference";
type Point = [number, number];

export function SmallestTriangleExplorer() {
  const [points, setPoints] = useState<Point[]>(TRIANGLE_POINTS);
  const [drag, setDrag] = useState<number | null>(null);
  const [selected, setSelected] = useState(0);
  const [edited, setEdited] = useState(false);
  const triangles = useMemo(() => {
    const result: { indices: number[]; area: number }[] = [];
    for (let i = 0; i < 12; i++)
      for (let j = i + 1; j < 13; j++)
        for (let k = j + 1; k < 14; k++) {
          const [a, b, c] = [points[i], points[j], points[k]];
          result.push({
            indices: [i, j, k],
            area:
              Math.abs(
                (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]),
              ) / 2,
          });
        }
    return result.sort((a, b) => a.area - b.area);
  }, [points]);
  const smallest = triangles[0].area;
  const bottlenecks = triangles.filter((t) => t.area - smallest < 1e-12);
  const triangle = bottlenecks[Math.min(selected, bottlenecks.length - 1)];
  useLearningView("smallest triangle", {
    description:
      "Fourteen draggable points in a unit square; the selected smallest triangle is shaded. Browser arithmetic is approximate and is not an official result.",
    points,
    edited,
    smallestArea: smallest,
    bottleneckCount: bottlenecks.length,
    selectedBottleneck: Math.min(selected + 1, bottlenecks.length),
    selectedPointNumbers: triangle.indices.map((i) => i + 1),
  });
  function move(index: number, x: number, y: number) {
    setPoints((previous) =>
      previous.map((p, i) =>
        i === index
          ? [Math.max(0, Math.min(1, x)), Math.max(0, Math.min(1, y))]
          : p,
      ),
    );
    setEdited(true);
    setSelected(0);
  }
  return (
    <section
      className="triangle-explorer"
      aria-label="Fourteen-point geometry explorer"
    >
      <div className="triangle-intro">
        <div>
          <h2>The smallest triangle sets the limit.</h2>
          <p>Drag a point. Every one of its triangles changes.</p>
        </div>
        <button
          className="button secondary"
          onClick={() => {
            setPoints(TRIANGLE_POINTS);
            setEdited(false);
            setSelected(0);
          }}
        >
          Reset reference
        </button>
      </div>
      <div className="triangle-layout">
        <svg
          viewBox="0 0 600 600"
          className="triangle-square"
          aria-label="Fourteen points in a unit square; the selected smallest triangle is shaded"
          onPointerMove={(event) => {
            if (drag === null) return;
            const box = event.currentTarget.getBoundingClientRect();
            move(
              drag,
              (((event.clientX - box.left) / box.width) * 600 - 50) / 500,
              1 - (((event.clientY - box.top) / box.height) * 600 - 50) / 500,
            );
          }}
          onPointerUp={() => setDrag(null)}
          onPointerCancel={() => setDrag(null)}
        >
          <rect
            x="50"
            y="50"
            width="500"
            height="500"
            fill="#fffdf7"
            stroke="#898b76"
            strokeWidth="1.5"
          />
          <polygon
            points={triangle.indices
              .map(
                (i) => `${50 + 500 * points[i][0]},${550 - 500 * points[i][1]}`,
              )
              .join(" ")}
            fill="#c66a3938"
            stroke="#ad512d"
            strokeWidth="2"
          />
          {points.map(([x, y], i) => (
            <g key={i}>
              <circle
                cx={50 + 500 * x}
                cy={550 - 500 * y}
                r="12"
                fill="transparent"
                tabIndex={0}
                role="button"
                aria-label={`Point ${i + 1}; use arrow keys to move`}
                style={{ cursor: "grab" }}
                onPointerDown={(event) => {
                  event.currentTarget.setPointerCapture(event.pointerId);
                  setDrag(i);
                }}
                onKeyDown={(event) => {
                  const d = event.shiftKey ? 0.001 : 0.005;
                  const delta: Record<string, Point> = {
                    ArrowLeft: [-d, 0],
                    ArrowRight: [d, 0],
                    ArrowUp: [0, d],
                    ArrowDown: [0, -d],
                  };
                  if (delta[event.key]) {
                    event.preventDefault();
                    move(i, x + delta[event.key][0], y + delta[event.key][1]);
                  }
                }}
              />
              <circle
                cx={50 + 500 * x}
                cy={550 - 500 * y}
                r="5"
                fill={triangle.indices.includes(i) ? "#ad512d" : "#25291f"}
                pointerEvents="none"
              />
              <text
                x={50 + 500 * x}
                y={550 - 500 * y - 14}
                textAnchor="middle"
                fontSize="12"
                fill="#505642"
                pointerEvents="none"
              >
                {i + 1}
              </text>
            </g>
          ))}
          <text x="50" y="585" fontSize="13" fill="#686e5a">
            0
          </text>
          <text x="550" y="585" textAnchor="end" fontSize="13" fill="#686e5a">
            1
          </text>
        </svg>
        <div className="triangle-reading">
          <span>
            {edited ? "Your exploration" : "Best substantiated reference"}
          </span>
          <strong>{smallest.toFixed(12)}</strong>
          <p>Minimum triangle area</p>
          <p>
            {edited
              ? "Moving one point can improve some triangles while making another smaller."
              : "Beyleveld’s construction has 26 equally small triangles. Improving it means lifting every bottleneck."}
          </p>
          {bottlenecks.length > 1 && (
            <label>
              Inspect a bottleneck ·{" "}
              {Math.min(selected + 1, bottlenecks.length)} /{" "}
              {bottlenecks.length}
              <input
                type="range"
                aria-label="Bottleneck triangle"
                min="0"
                max={bottlenecks.length - 1}
                value={Math.min(selected, bottlenecks.length - 1)}
                onChange={(e) => setSelected(Number(e.target.value))}
              />
            </label>
          )}
          <small>
            {edited
              ? "Interactive approximation. Official submissions use exact arithmetic."
              : "The figure uses decimal coordinates. The verifier reproduces the reference exactly."}
          </small>
        </div>
      </div>
    </section>
  );
}
