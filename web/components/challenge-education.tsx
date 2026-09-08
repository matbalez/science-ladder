import type { Challenge } from "@/lib/types";
import { asRecord, asText } from "@/lib/scientific";
import { ExternalLink } from "./ui";

// Editorial notes are bound to the published sources. They do not alter locks,
// validator claims or scores; new challenges carry education in their adopted candidates.
const editions: Record<
  string,
  {
    source: string;
    frontier: string[];
    significance: string[];
    sources: [string, string][];
  }
> = {
  "matbalez/science-ladder-quiet-echoes": {
    source: "f42f527e97563b1c068a1835732c6da44f21223f",
    frontier: [
      "A binary pulse is a sequence of +1 and −1 signs. When it is compared with shifted copies of itself, the unwanted matches are called sidelobes. Finding signs that make their total squared strength as small as possible is the low-autocorrelation binary sequence problem: a compact mathematical object with a difficult search landscape.",
      "At 512 signs, this challenge targets a published sequence with energy 17,996, from Table 2 of Dual-Step Optimization for Binary Sequences with High Merit Factors. The authors combine GPU search with unrestricted refinement. Their 2026 follow-up reports further results, but no improved length-512 sequence. The September 8, 2026 literature check found no stronger comparable published construction. The community’s target is to beat this reproduced frontier reference. An improvement would receive a fresh novelty check before a world-record claim.",
    ],
    significance: [
      "The checker evaluates every nonzero shift using exact integer arithmetic. Lower energy therefore directly improves the stated mathematical objective; it cannot be achieved by reporting a favorable score or exploiting a sampled test set. A new sequence below 17,996 would be a concrete, reusable improvement over this published reference. A passing sequence above it is a valid attempt, not research progress beyond the reference.",
      "Autocorrelation helps explain pulse matching in signal processing, but total sidelobe energy is only one property. Lower energy does not guarantee a smaller worst sidelobe, better detection in noise or Doppler tolerance. One improved sequence also does not establish a better general search algorithm, the optimum at length 512, or a solution to the asymptotic merit-factor problem. This is a focused mathematical research challenge, not a demonstrated radar breakthrough.",
    ],
    sources: [
      [
        "Dual-Step Optimization · definitions and Table 2",
        "https://arxiv.org/html/2409.07222v1",
      ],
      [
        "Prioritizing Search Space Regions · 2026 results",
        "https://arxiv.org/html/2607.09688v1",
      ],
    ],
  },
  "matbalez/science-ladder-load-paths": {
    source: "f738b962986c192a6f6b986db6151f57737d2f28",
    frontier: [
      "Topology optimization asks where a limited amount of material should go. Here the left edge of each structure is fixed and three different forces act at the right. A layout that is stiff under one force may bend too much under another. Each submitted layout must cope with all three.",
      "Density-based stiffness optimization is an established method. Research extends it to uncertainty, many possible loads, manufacturing constraints and larger models; the paper on robust and stochastic compliance below studies finite loading scenarios. This challenge isolates a small, reproducible part of that problem: three fixed two-dimensional geometries and nine elastic calculations.",
      "Our reference uses 110 iterations of a standard SIMP optimizer. It is neither a converged optimum nor the strongest published method. Beating it demonstrates progress on this benchmark, not advancement beyond the field’s current frontier. The repository reports sensitivity to longer reference runs so that this distinction is inspectable.",
    ],
    significance: [
      "Compliance measures how much a structure gives under a specified force; lower is stiffer in this linear-elastic model. The checker independently solves the equilibrium equations, enforces the same material budget and checks all loads. The score is the smallest reference-to-candidate worst-load compliance ratio across the three geometries. A score of 1.01 corresponds to roughly 0.99% less worst-load compliance in even the least-improved geometry, subject to score rounding.",
      "This makes the score an interpretable measure of stiffness at fixed material use. It does not measure stress limits, buckling, fatigue, manufacturability or physical safety. The coarse meshes and density model are approximations, and offline search on these public cases is allowed. Establishing a frontier result would require stronger converged comparisons, mesh refinement, unseen cases and appropriate physical validation. For now, Load Paths is an educational optimization benchmark and a demonstration of multi-metric verification.",
    ],
    sources: [
      [
        "Robust and stochastic compliance · finite load scenarios",
        "https://arxiv.org/abs/2103.04594",
      ],
      [
        "Reference method and numerical validation",
        "https://github.com/matbalez/science-ladder-load-paths/tree/f738b962986c192a6f6b986db6151f57737d2f28",
      ],
    ],
  },
};

export function challengeEducation(c: Challenge) {
  const supplied = asRecord(c.education);
  const edition = editions[c.repository];
  return asText(supplied.frontier) && asText(supplied.significance)
    ? {
        frontier: asText(supplied.frontier).split(/\n\s*\n/),
        significance: asText(supplied.significance).split(/\n\s*\n/),
        sources: [] as [string, string][],
      }
    : edition?.source === c.sourceCommit
      ? edition
      : undefined;
}

export function ChallengeEducation({ challenge: c }: { challenge: Challenge }) {
  const context = challengeEducation(c);
  if (!context) return null;
  return (
    <div>
      <h3>Where the research stands</h3>
      {context.frontier.map((p, i) => (
        <p key={i}>{p}</p>
      ))}
      <h3>What progress would mean</h3>
      {context.significance.map((p, i) => (
        <p key={i}>{p}</p>
      ))}
      {context.sources.length > 0 && (
        <p className="subtle">
          Editorial context · September 8, 2026. Sources:{" "}
          {context.sources.map(([title, href], i) => (
            <span key={href}>
              {i > 0 && "; "}
              <ExternalLink href={href}>{title}</ExternalLink>
            </span>
          ))}
        </p>
      )}
    </div>
  );
}
