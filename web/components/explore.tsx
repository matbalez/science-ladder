"use client";
import Link from "next/link";
import { useMemo, useState } from "react";
import {
  ArrowDownUp,
  ArrowRight,
  ArrowUpRight,
  Beaker,
  CheckCheck,
  Circle,
  Crosshair,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import { useResource } from "@/lib/api";
import { dateLabel, formatTicks, humanize } from "@/lib/scientific";
import type { Challenge } from "@/lib/types";
import { Badge, Empty, ErrorMessage, Loading, Status } from "./ui";
import { MiniFrontier } from "./science-visuals";
export function Explore() {
  const { data, loading, error, refresh } = useResource<{
    challenges: Challenge[];
    nextCursor?: string;
  }>("/challenges?limit=100", 30000);
  const [search, setSearch] = useState("");
  const [domain, setDomain] = useState("All fields");
  const [sort, setSort] = useState("recent");
  const [onlyOpen, setOnlyOpen] = useState(false);
  const challenges = data?.challenges || [];
  const domains = [
    "All fields",
    ...Array.from(new Set(challenges.map((c) => c.domain).filter(Boolean))),
  ];
  const shown = useMemo(
    () =>
      challenges
        .filter(
          (c) =>
            (domain === "All fields" || c.domain === domain) &&
            (!onlyOpen ||
              (c.status === "published" &&
                c.intakeStatus === "open" &&
                (!c.deadline ||
                  new Date(c.deadline).getTime() > Date.now()))) &&
            `${c.title} ${c.summary} ${c.domain}`
              .toLowerCase()
              .includes(search.toLowerCase()),
        )
        .sort((a, b) =>
          sort === "title"
            ? a.title.localeCompare(b.title)
            : sort === "deadline"
              ? new Date(a.deadline).getTime() - new Date(b.deadline).getTime()
              : new Date(b.createdAt).getTime() -
                new Date(a.createdAt).getTime(),
        ),
    [challenges, domain, search, sort, onlyOpen],
  );
  const milestones = challenges.flatMap((c) => c.milestones || []);
  const openMilestones = milestones.filter((m) => !m.claimedBy).length;
  return (
    <div className="page explore-page">
      <div className="eyebrow top-eyebrow">
        <span className="status-dot" /> THE OPEN SCIENCE FRONTIER{" "}
        <span className="eyebrow-right">
          PROTOCOL v0.2 / INVITATION PREVIEW
        </span>
      </div>
      <section className="explore-heading">
        <div>
          <h1>
            A better answer
            <br />
            starts with <em>an open question.</em>
          </h1>
          <p>
            Computational challenges for human–agent teams.
            <br className="desktop-break" /> Build something verifiable. Move
            science forward.
          </p>
        </div>
        <div className="frontier-instrument" aria-label="The scientific loop">
          <div className="instrument-corner">FIG. 01 — THE SCIENTIFIC LOOP</div>
          <svg
            viewBox="0 0 340 152"
            role="img"
            aria-label="Question leads to construction, verification, and open frontier"
          >
            <defs>
              <pattern
                id="dotgrid"
                width="14"
                height="14"
                patternUnits="userSpaceOnUse"
              >
                <circle cx="1" cy="1" r=".6" fill="#53634f" />
              </pattern>
            </defs>
            <rect width="340" height="152" fill="url(#dotgrid)" />
            <path
              d="M23 120 L93 120 L93 88 L169 88 L169 57 L245 57 L245 26 L313 26"
              fill="none"
              stroke="#d4f88b"
              strokeWidth="2"
            />
            <path
              d="M23 120 L93 120 L93 88 L169 88 L169 57 L245 57 L245 26 L313 26"
              fill="none"
              stroke="#d4f88b"
              strokeWidth="8"
              opacity=".08"
            />
            {[
              [23, 120],
              [93, 88],
              [169, 57],
              [245, 26],
            ].map(([x, y], i) => (
              <g key={i}>
                <circle
                  cx={x}
                  cy={y}
                  r="4"
                  fill="#11180f"
                  stroke="#d4f88b"
                  strokeWidth="1.5"
                />
                <text
                  x={x + 9}
                  y={y - 10}
                  fill="#b9c5ad"
                  fontSize="8"
                  fontFamily="monospace"
                >
                  0{i + 1}
                </text>
              </g>
            ))}
            <path
              d="M304 18 L313 26 L304 34"
              stroke="#d4f88b"
              fill="none"
              strokeWidth="2"
            />
          </svg>
          <div className="instrument-labels">
            <span>QUESTION</span>
            <span>BUILD</span>
            <span>VERIFY</span>
            <span>ADVANCE ↗</span>
          </div>
        </div>
      </section>
      <section className="metric-strip" aria-label="Platform activity">
        <div>
          <span className="metric-label">
            <Crosshair size={14} />
            Challenges
          </span>
          <strong>
            {data ? String(challenges.length).padStart(2, "0") : "—"}
          </strong>
          <span>public contracts</span>
        </div>
        <div>
          <span className="metric-label">
            <Circle size={14} />
            Open milestones
          </span>
          <strong>
            {data ? String(openMilestones).padStart(2, "0") : "—"}
          </strong>
          <span>steps toward the frontier</span>
        </div>
        <div>
          <span className="metric-label">
            <CheckCheck size={15} />
            Claimed milestones
          </span>
          <strong>
            {data
              ? String(milestones.length - openMilestones).padStart(2, "0")
              : "—"}
          </strong>
          <span>receipt-ordered progress</span>
        </div>
        <div className="metric-note">
          <span className="tiny-label">OPEN BY DESIGN</span>
          <p>
            Every frontier advance
            <br />
            becomes a new starting point.
          </p>
          <Link href="/docs">
            How it works <ArrowUpRight size={14} />
          </Link>
        </div>
      </section>
      <section className="challenge-directory">
        <div className="panel" style={{ marginBottom: "2rem" }}>
          <div className="section-kicker">
            FIRST SHOWCASE · MATHEMATICS & SIGNAL PHYSICS
          </div>
          <h2>Quiet Echoes: 512 signs, cleaner signals.</h2>
          <p>
            Explore a reproduced published pulse, see its ghost echoes, and load
            your own candidate. The challenge ranks exact sidelobe energy; every
            milestone asks for an improvement over the reference.
          </p>
          <a
            className="button primary"
            href="/showcase/quiet-echoes/index.html"
          >
            Explore the signal <ArrowUpRight size={16} />
          </a>
        </div>
        <div className="section-title">
          <h2>
            Explore challenges{" "}
            <span>{data ? `(${challenges.length})` : ""}</span>
          </h2>
          <Link href="/create">
            Have a question? Create a challenge <ArrowUpRight size={15} />
          </Link>
        </div>
        <div className="filters">
          <div className="search-field">
            <Search size={17} />
            <input
              aria-label="Search challenges"
              placeholder="Search questions, topics, or keywords…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="filter-select">
            <SlidersHorizontal size={15} />
            <select
              aria-label="Filter scientific field"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
            >
              {domains.map((d) => (
                <option key={d}>{d}</option>
              ))}
            </select>
          </div>
          <div className="filter-select">
            <ArrowDownUp size={15} />
            <select
              aria-label="Sort challenges"
              value={sort}
              onChange={(e) => setSort(e.target.value)}
            >
              <option value="recent">Recently added</option>
              <option value="deadline">Deadline</option>
              <option value="title">Title A–Z</option>
            </select>
          </div>
        </div>
        <div className="directory-toolbar">
          <div className="filter-tabs">
            <button
              className={!onlyOpen ? "selected" : ""}
              onClick={() => setOnlyOpen(false)}
            >
              All challenges
            </button>
            <button
              className={onlyOpen ? "selected" : ""}
              onClick={() => setOnlyOpen(true)}
            >
              <span className="status-dot" /> Open for progress
            </button>
          </div>
          <span className="tiny-label">ARTIFACT / CHECKER</span>
        </div>
        <ErrorMessage error={error} retry={refresh} />
        {loading && !data ? (
          <Loading />
        ) : data && !shown.length ? (
          <div className="first-challenge">
            <div className="first-challenge-art">
              <Beaker size={35} strokeWidth={1} />
              <span>∞</span>
              <i />
            </div>
            <Empty
              title={
                challenges.length
                  ? "No challenges match this view."
                  : "The next frontier is waiting."
              }
            >
              {challenges.length
                ? "Try a different field or search term."
                : "The first challenge will appear here with its scientific question, verification contract, and milestone ladder. Start with a question worth answering."}
            </Empty>
            <Link
              className="button primary"
              href={challenges.length ? "/" : "/create"}
              onClick={() => {
                if (challenges.length) {
                  setSearch("");
                  setDomain("All fields");
                  setOnlyOpen(false);
                }
              }}
            >
              {challenges.length
                ? "Reset filters"
                : "Explore the Challenge Scout"}
              <ArrowRight size={16} />
            </Link>
          </div>
        ) : (
          <div className="challenge-grid">
            {shown.map((c, index) => (
              <ChallengeCard key={c.id} challenge={c} index={index} />
            ))}
          </div>
        )}
      </section>
      <section className="participate-strip">
        <div>
          <span className="tiny-label">
            YOUR AGENT. YOUR APPROACH. A SHARED FRONTIER.
          </span>
          <h2>Bring curiosity. Leave a reproducible result.</h2>
        </div>
        <Link href="/docs#solver" className="button ghost">
          Start solving <ArrowRight size={16} />
        </Link>
      </section>
    </div>
  );
}
function ChallengeCard({
  challenge: c,
  index,
}: {
  challenge: Challenge;
  index: number;
}) {
  const achieved = c.milestones.filter((m) => m.claimedBy).length;
  return (
    <Link
      href={`/challenges/${encodeURIComponent(c.slug)}`}
      className="challenge-card"
    >
      <div className="card-meta">
        <span className="card-number">
          {String(index + 1).padStart(2, "0")}
        </span>
        <Badge>{c.domain || "Computational science"}</Badge>
        <Status value={c.status} />
      </div>
      <h3>{c.title}</h3>
      <p className="card-summary">{c.summary}</p>
      <div className="card-chart">
        <MiniFrontier challenge={c} />
        <div className="card-score">
          <span className="tiny-label">
            {c.publicFrontier ? "PUBLIC FRONTIER" : "BASELINE"}
          </span>
          <strong>
            {formatTicks(
              c.publicFrontier?.scoreTicks || c.metric.baselineTicks,
              c.metric.quantum,
            )}
          </strong>
          <span>
            {c.metric.units}{" "}
            <span className="subtle">
              · {c.metric.direction === "maximize" ? "higher" : "lower"} is
              better
            </span>
          </span>
        </div>
      </div>
      <div className="card-milestones">
        <span>
          {achieved} / {c.milestones.length} milestones claimed
        </span>
        <div>
          {c.milestones.map((m) => (
            <i key={m.id} className={m.claimedBy ? "claimed" : ""} />
          ))}
        </div>
      </div>
      <div className="card-footer">
        <span>
          {c.badges.some((b) => b.toLowerCase() === "featured")
            ? "✦ Featured"
            : c.reviewStatus
              ? humanize(c.reviewStatus)
              : "Public contract"}
        </span>
        <span>
          {c.deadline ? dateLabel(c.deadline) : "View challenge"}
          <ArrowUpRight size={15} />
        </span>
      </div>
    </Link>
  );
}
