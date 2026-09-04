"use client";
import { useMemo, useState } from "react";
import { pulseStatistics } from "@/lib/signals";

export function BinaryPulse({ pulse }: { pulse: number[] }) {
  const [visibleLags, setVisibleLags] = useState(128);
  const stats = useMemo(() => pulseStatistics(pulse), [pulse]);
  const scale = 110 / Math.max(stats.peak, 1);
  return (
    <div className="binary-pulse-preview">
      <p className="artifact-info">
        Exact binary pulse from the published artifact. These browser
        calculations help inspect the result; signed receipts record platform
        verification.
      </p>
      <svg
        viewBox="0 0 700 125"
        role="img"
        aria-label="512 binary signs, read left to right then down"
      >
        {pulse.map((value, i) => (
          <rect
            key={i}
            x={30 + (i % 64) * 10}
            y={10 + Math.floor(i / 64) * 13}
            width="8"
            height="11"
            fill={value === 1 ? "#d4f88b" : "#37434a"}
          >
            <title>
              Sign {i + 1}: {value}
            </title>
          </rect>
        ))}
      </svg>
      <dl className="contract-grid">
        <div>
          <dt>Sidelobe energy · lower is better</dt>
          <dd>{stats.energy.toLocaleString("en-US")}</dd>
        </div>
        <div>
          <dt>Derived merit factor</dt>
          <dd>{stats.merit?.toFixed(6) ?? "Undefined"}</dd>
        </div>
        <div>
          <dt>Largest sidelobe · diagnostic</dt>
          <dd>{stats.peak}</dd>
        </div>
      </dl>
      <svg
        className="artifact-canvas"
        viewBox="0 0 700 270"
        role="img"
        aria-label={`Aperiodic correlations at lags 1 to ${visibleLags}; all 511 lags contribute to energy ${stats.energy}`}
      >
        <line x1="30" x2="675" y1="135" y2="135" stroke="#59666d" />
        {stats.correlations.slice(0, visibleLags).map((c, i) => (
          <line
            key={i}
            x1={30 + ((i + 0.5) * 640) / visibleLags}
            x2={30 + ((i + 0.5) * 640) / visibleLags}
            y1="135"
            y2={135 - c * scale}
            stroke="#d4f88b"
            strokeWidth={Math.max(1, 400 / visibleLags)}
          >
            <title>
              Lag {i + 1}: {c}
            </title>
          </line>
        ))}
      </svg>
      <div className="artifact-controls">
        <label>
          Visible lags: {visibleLags}{" "}
          <input
            aria-label="Visible autocorrelation lags"
            type="range"
            min="32"
            max="511"
            value={visibleLags}
            onChange={(e) => setVisibleLags(Number(e.target.value))}
          />
        </label>
      </div>
      <p className="artifact-info">
        The alignment peak C₀ = 512 is omitted. Energy always includes all 511
        nonzero lags. Lower total energy need not reduce the largest individual
        echo.
      </p>
    </div>
  );
}
