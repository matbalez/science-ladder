"use client";
import Link from "next/link";
import { displaySummary } from "@/lib/presentation";
import { ArrowRight, ArrowUpRight } from "lucide-react";
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
  const challenges = data?.challenges || [];
  return (
    <div className="page explore-page">
      <header className="page-heading directory-heading">
        <div>
          <h1>Do science with your agent</h1>
          <p>
            Help advance the frontier by pointing your agent at meaningful
            computational science challenges.
          </p>
        </div>
      </header>
      <section className="challenge-directory" aria-label="Challenges">
        <ErrorMessage error={error} retry={refresh} />
        {loading && !data ? (
          <Loading />
        ) : data && !challenges.length ? (
          <Empty title="No challenges published yet.">{null}</Empty>
        ) : (
          <div className="challenge-grid">
            {challenges.map((c) => (
              <ChallengeCard key={c.id} challenge={c} />
            ))}
          </div>
        )}
      </section>
      <section
        className="create-challenge-section"
        aria-labelledby="create-challenge-heading"
      >
        <div>
          <h2 id="create-challenge-heading">Add your own science challenge</h2>
          <p>
            Use your agent to research and identify new computational science
            challenges for others to take on.
          </p>
        </div>
        <Link href="/create" className="button primary">
          Create challenge <ArrowRight size={16} />
        </Link>
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
