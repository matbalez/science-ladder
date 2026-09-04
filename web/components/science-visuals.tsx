"use client";
import { useEffect, useMemo, useState } from "react";
import { Box, Download, RotateCw } from "lucide-react";
import { formatTicks, plotRatio } from "@/lib/scientific";
import type { Challenge, Submission } from "@/lib/types";
import { Empty, ErrorMessage } from "./ui";
function chartData(challenge: Challenge) {
  const items = (challenge.submissions || [])
    .filter(
      (s) =>
        s.scoreTicks !== undefined &&
        /^-?\d+$/.test(s.scoreTicks) &&
        s.outcome === "valid",
    )
    .sort((a, b) => a.sequence - b.sequence);
  const scores = [
    challenge.metric.baselineTicks,
    challenge.publicFrontier?.scoreTicks || challenge.metric.baselineTicks,
    ...items.map((s) => s.scoreTicks!),
    ...challenge.milestones.map((m) => m.thresholdTicks),
  ].filter((s) => /^-?\d+$/.test(s || ""));
  const values = scores.map(BigInt);
  const min = values.reduce((a, b) => (a < b ? a : b), values[0] || 0n);
  const max = values.reduce((a, b) => (a > b ? a : b), values[0] || 1n);
  let best = challenge.metric.baselineTicks || "0";
  const events = [
    {
      sequence: 0,
      scoreTicks: best,
      submission: undefined as Submission | undefined,
    },
  ];
  for (const s of items) {
    const better =
      challenge.metric.direction === "maximize"
        ? BigInt(s.scoreTicks!) > BigInt(best)
        : BigInt(s.scoreTicks!) < BigInt(best);
    if (better) {
      best = s.scoreTicks!;
      events.push({ sequence: s.sequence, scoreTicks: best, submission: s });
    }
  }
  return { items, events, min, max };
}
export function MiniFrontier({ challenge }: { challenge: Challenge }) {
  const { events, min, max } = chartData(challenge);
  const summaryOnly = events.length === 1 && !!challenge.publicFrontier;
  if (summaryOnly)
    events.push({
      sequence: 1,
      scoreTicks: challenge.publicFrontier!.scoreTicks,
      submission: undefined,
    });
  const points = events.map(
    (e, i) =>
      `${15 + (i / Math.max(events.length - 1, 1)) * 180},${70 - plotRatio(e.scoreTicks, min, max) * 50}`,
  );
  return (
    <svg
      className="mini-frontier"
      viewBox="0 0 210 90"
      role="img"
      aria-label={
        summaryOnly
          ? "Baseline compared with the current public frontier"
          : events.length === 1
            ? "Baseline only: no public frontier advances yet"
            : `${events.length - 1} public score advances`
      }
    >
      <path
        d="M10 20H202 M10 45H202 M10 70H202"
        stroke="currentColor"
        opacity=".13"
        fill="none"
      />
      {events.length === 1 ? (
        <>
          <path
            d={`M15 ${70 - plotRatio(events[0].scoreTicks, min, max) * 50}H195`}
            stroke="currentColor"
            strokeDasharray="3 5"
            opacity=".4"
          />
          <text x="112" y="86" fill="currentColor" fontSize="7" opacity=".6">
            BASELINE · NO ADVANCE YET
          </text>
        </>
      ) : (
        <polyline
          points={points.join(" ")}
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        />
      )}
      {summaryOnly && (
        <text x="108" y="86" fill="currentColor" fontSize="7" opacity=".6">
          BASELINE → CURRENT
        </text>
      )}
      {points.map((p, i) => (
        <circle
          key={i}
          cx={p.split(",")[0]}
          cy={p.split(",")[1]}
          r="3"
          fill="currentColor"
        />
      ))}
    </svg>
  );
}
export function FrontierChart({ challenge }: { challenge: Challenge }) {
  const { items, events, min, max } = useMemo(
    () => chartData(challenge),
    [challenge],
  );
  const [selected, setSelected] = useState<number>();
  const maxSequence = Math.max(...items.map((s) => s.sequence), 1);
  const x = (seq: number) => 90 + (seq / maxSequence) * 760;
  const y = (score: string) => 248 - plotRatio(score, min, max) * 200;
  const path =
    events
      .map(
        (e, i) =>
          `${i ? "H" : "M"}${x(e.sequence)}${i ? "V" : ","}${y(e.scoreTicks)}`,
      )
      .join(" ") + `H${x(maxSequence)}`;
  return (
    <div className="frontier-chart">
      <div className="chart-key">
        <span>
          <i className="lime-dot" />
          Public score frontier
        </span>
        <span>
          <i className="muted-dot" />
          Valid result
        </span>
        <span>Receipt order →</span>
      </div>
      <svg
        viewBox="0 0 930 310"
        role="img"
        aria-label="Verified score history in receipt order. Milestone thresholds are shown as horizontal lines."
      >
        {[0, 0.25, 0.5, 0.75, 1].map((v, i) => (
          <g key={i}>
            <path d={`M90 ${48 + v * 200}H870`} stroke="#29312a" />
            <text
              x="75"
              y={52 + v * 200}
              textAnchor="end"
              fill="#8f9b8c"
              fontSize="10"
              fontFamily="monospace"
            >
              {formatTicks(
                (max - ((max - min) * BigInt(i)) / 4n).toString(),
                challenge.metric.quantum,
              )}
            </text>
          </g>
        ))}
        {challenge.milestones.map((m) => (
          <g key={m.id}>
            <path
              d={`M90 ${y(m.thresholdTicks)}H870`}
              stroke={m.claimedBy ? "#b8e970" : "#67754e"}
              strokeDasharray="4 6"
              opacity=".65"
            />
            <text
              x="868"
              y={y(m.thresholdTicks) - 7}
              textAnchor="end"
              fill="#9ead8c"
              fontSize="10"
            >
              {m.label}
            </text>
          </g>
        ))}
        <path d={path} stroke="#cdf992" strokeWidth="2.5" fill="none" />
        {items.map((s) => (
          <circle
            key={s.id}
            cx={x(s.sequence)}
            cy={y(s.scoreTicks!)}
            r={selected === s.sequence ? 6 : 4}
            fill="#779b8c"
            tabIndex={0}
            role="button"
            aria-label={`Receipt ${s.sequence}, score ${formatTicks(s.scoreTicks, challenge.metric.quantum)}`}
            onMouseEnter={() => setSelected(s.sequence)}
            onFocus={() => setSelected(s.sequence)}
            onClick={() => setSelected(s.sequence)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") setSelected(s.sequence);
            }}
          />
        ))}
        {events.map((e) => (
          <circle
            key={e.sequence}
            cx={x(e.sequence)}
            cy={y(e.scoreTicks)}
            r="4"
            fill="#cdf992"
            pointerEvents="none"
          />
        ))}
        <text x="90" y="275" fill="#929d8b" fontSize="10">
          Baseline
        </text>
        <text x="870" y="275" textAnchor="end" fill="#929d8b" fontSize="10">
          {items.length
            ? `Receipt #${maxSequence}`
            : "Awaiting the first verified result"}
        </text>
      </svg>
      {selected !== undefined && (
        <div className="chart-inspection">
          Receipt #{selected}{" "}
          <strong>
            {formatTicks(
              items.find((s) => s.sequence === selected)?.scoreTicks,
              challenge.metric.quantum,
            )}{" "}
            {challenge.metric.units}
          </strong>
          <span>
            {items.find((s) => s.sequence === selected)?.attribution?.model ||
              "No model attestation"}
          </span>
        </div>
      )}
      {!items.length && (
        <p className="chart-empty-note">
          The baseline and declared milestones are shown. Verified submissions
          will trace the actual frontier here.
        </p>
      )}
    </div>
  );
}
interface Point {
  x: number;
  y: number;
  z?: number;
  r?: number;
}
function readPoints(value: unknown): Point[] {
  const obj = value as Record<string, unknown>;
  const raw = Array.isArray(value)
    ? value
    : obj?.points ||
      obj?.centers ||
      obj?.coordinates ||
      obj?.vertices ||
      obj?.circles;
  if (!Array.isArray(raw) || raw.length > 20000) return [];
  return raw
    .map((p: unknown) => {
      if (Array.isArray(p))
        return {
          x: Number(p[0]),
          y: Number(p[1]),
          z: p[2] === undefined ? undefined : Number(p[2]),
        };
      const q = p as Record<string, unknown>;
      return {
        x: Number(q?.x),
        y: Number(q?.y),
        z: q?.z === undefined ? undefined : Number(q.z),
        r: q?.r === undefined ? undefined : Number(q.r),
      };
    })
    .filter(
      (p) =>
        Number.isFinite(p.x) &&
        Number.isFinite(p.y) &&
        (p.z === undefined || Number.isFinite(p.z)),
    );
}
export function ArtifactViewer({ digest }: { digest?: string }) {
  const [points, setPoints] = useState<Point[]>([]);
  const [angle, setAngle] = useState(0);
  const [error, setError] = useState<Error>();
  const [loaded, setLoaded] = useState(false);
  const [name, setName] = useState("");
  useEffect(() => {
    if (!digest) return;
    const controller = new AbortController();
    setLoaded(false);
    setError(undefined);
    setPoints([]);
    (async () => {
      try {
        const r = await fetch(`/v1/artifacts/${encodeURIComponent(digest)}`, {
          signal: controller.signal,
        });
        if (!r.ok) throw new Error("This artifact is not publicly available.");
        const length = Number(r.headers.get("content-length"));
        if (length > 2_000_000)
          throw new Error(
            "This artifact is too large for the interactive preview. Download it to inspect locally.",
          );
        if (!(r.headers.get("content-type") || "").includes("json"))
          throw new Error(
            "Download the canonical artifact bundle to inspect this construction. Interactive preview supports JSON point sets.",
          );
        const reader = r.body?.getReader();
        if (!reader) throw new Error("Artifact content is unavailable.");
        const chunks: Uint8Array[] = [];
        let size = 0;
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          size += value.byteLength;
          if (size > 2_000_000) {
            await reader.cancel();
            throw new Error("Artifact exceeds the interactive preview limit.");
          }
          chunks.push(value);
        }
        const all = new Uint8Array(size);
        let offset = 0;
        for (const chunk of chunks) {
          all.set(chunk, offset);
          offset += chunk.byteLength;
        }
        const value = JSON.parse(new TextDecoder().decode(all));
        let pts = readPoints(value);
        let artifactFile = "";
        if (!pts.length && value.files && typeof value.files === "object") {
          for (const [path, encoded] of Object.entries(value.files)) {
            if (
              !path.endsWith(".json") ||
              typeof encoded !== "string" ||
              encoded.length > 2_000_000
            )
              continue;
            try {
              const bytes = Uint8Array.from(atob(encoded), (c) =>
                c.charCodeAt(0),
              );
              const candidatePoints = readPoints(
                JSON.parse(new TextDecoder().decode(bytes)),
              );
              if (candidatePoints.length) {
                pts = candidatePoints;
                artifactFile = path;
                break;
              }
            } catch {
              /* Non-geometry files remain available in the exported bundle. */
            }
          }
        }
        setPoints(pts);
        setName(
          typeof value.title === "string"
            ? value.title
            : artifactFile || "Public artifact",
        );
      } catch (e) {
        if ((e as Error).name !== "AbortError") setError(e as Error);
      } finally {
        if (!controller.signal.aborted) setLoaded(true);
      }
    })();
    return () => controller.abort();
  }, [digest]);
  const projected = points.map((p) => ({
    x: p.x * Math.cos(angle) - (p.z || 0) * Math.sin(angle),
    y: p.y,
    depth: p.x * Math.sin(angle) + (p.z || 0) * Math.cos(angle),
    r: p.r,
  }));
  const xs = projected.map((p) => p.x);
  const ys = projected.map((p) => p.y);
  const xmin = Math.min(...xs, 0),
    xmax = Math.max(...xs, 1),
    ymin = Math.min(...ys, 0),
    ymax = Math.max(...ys, 1);
  const scale = Math.min(600 / (xmax - xmin || 1), 340 / (ymax - ymin || 1));
  return (
    <div className="artifact-panel">
      <div className="panel-heading">
        <h3>
          <Box size={17} /> Construction viewer
        </h3>
        {digest && (
          <a
            className="button small ghost"
            href={`/v1/artifacts/${encodeURIComponent(digest)}`}
            download
          >
            <Download size={14} />
            Artifact
          </a>
        )}
      </div>
      {!digest ? (
        <Empty
          title="The first open artifact belongs here."
          icon={<Box size={30} />}
        >
          A public-frontier construction will become available after validation
          and adjudication.
        </Empty>
      ) : error ? (
        <div className="artifact-info">
          <ErrorMessage error={error} />
        </div>
      ) : !loaded ? (
        <div className="loading">Reading artifact…</div>
      ) : !points.length ? (
        <Empty title="No supported geometry in this artifact.">
          Download the artifact to inspect its exact contents and reproduce the
          result.
        </Empty>
      ) : (
        <>
          <svg
            className="artifact-canvas"
            viewBox="0 0 700 440"
            role="img"
            aria-label={`${name}: ${points.length} artifact coordinates`}
          >
            <defs>
              <pattern
                id="artifact-grid"
                width="25"
                height="25"
                patternUnits="userSpaceOnUse"
              >
                <path
                  d="M25 0H0V25"
                  fill="none"
                  stroke="#29332a"
                  strokeWidth=".5"
                />
              </pattern>
            </defs>
            <rect width="700" height="440" fill="url(#artifact-grid)" />
            {projected
              .sort((a, b) => a.depth - b.depth)
              .map((p, i) => (
                <circle
                  key={i}
                  cx={50 + (p.x - xmin) * scale}
                  cy={390 - (p.y - ymin) * scale}
                  r={p.r ? Math.max(1, p.r * scale) : 4}
                  fill={p.r ? "#c9f59520" : "#c9f595"}
                  stroke="#c9f595"
                  strokeWidth={p.r ? 1 : 0.5}
                >
                  <title>
                    ({p.x}, {p.y})
                  </title>
                </circle>
              ))}
          </svg>
          <div className="artifact-controls">
            <span className="mono">
              {points.length} points · coordinates from artifact
            </span>
            {points.some((p) => p.z !== undefined) && (
              <label>
                <RotateCw size={14} />
                <input
                  aria-label="Rotate construction"
                  type="range"
                  min="0"
                  max="6.283"
                  step="0.02"
                  value={angle}
                  onChange={(e) => setAngle(Number(e.target.value))}
                />
              </label>
            )}
          </div>
        </>
      )}
    </div>
  );
}
