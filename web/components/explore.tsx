"use client";
import Link from "next/link";
import { displaySummary } from "@/lib/presentation";
import { useMemo, useState } from "react";
import {
  ArrowDownUp,
  ArrowRight,
  ArrowUpRight,
  Beaker,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import { useResource } from "@/lib/api";
import { dateLabel, formatTicks, humanize } from "@/lib/scientific";
import type { Challenge } from "@/lib/types";
import { Empty, ErrorMessage, Loading, Status } from "./ui";
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
  return (
    <div className="page explore-page">
      <header className="page-heading directory-heading">
        <div>
          <h1>Explore challenges</h1>
          <p>Computational science problems for you and your coding agent.</p>
        </div>
      </header>
      <section className="challenge-directory" aria-label="Challenges">
        <div className="filters">
          <div className="search-field">
            <Search size={17} />
            <input
              aria-label="Search challenges"
              placeholder="Search challenges…"
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
              <span className="status-dot" /> Open for submissions
            </button>
          </div>
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
                  : "No challenges published yet."
              }
            >
              {challenges.length
                ? "Try a different field or search term."
                : "Published challenges will appear here. You can start drafting one now."}
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
                : "Create the first challenge"}
              <ArrowRight size={16} />
            </Link>
          </div>
        ) : (
          <div className="challenge-grid">
            {shown.map((c) => (
              <ChallengeCard key={c.id} challenge={c} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
function ChallengeCard({ challenge: c }: { challenge: Challenge }) {
  const achieved = c.milestones.filter((m) => m.claimedBy).length;
  return (
    <Link
      href={`/challenges/${encodeURIComponent(c.slug)}`}
      className="challenge-card"
    >
      <div className="card-meta">
        <span className="card-domain">
          {c.domain || "Computational science"}
        </span>
        <Status value={c.status} />
      </div>
      <h3>{c.title}</h3>
      <p className="card-summary">{displaySummary(c)}</p>
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
