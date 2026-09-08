# Educational context

Every new challenge must explain **where the research stands** and **what progress
would mean**. Scout 1.3 introduced (and current Scout 1.6 generates) `education.frontier` and
`education.significance` in the candidate YAML, beside `manifest`. These are
plain text, not HTML. Cite titles and locations from the candidate's primary
sources. Explain a concrete score improvement, preserved conditions, scientific
use, claim limits and additional evidence needed for broader impact. Distinguish
an intentionally limited reference from the strongest published result.

The candidate parser requires both sections for viable Scout 1.3 proposals.
Scientific review requires them for all new reviews, including old-prompt imports,
and assesses whether their reasoning and sources support the claim. Populated
fields are not proof of scientific merit. Archived 1.0–1.2 candidates and existing
locks remain readable and runnable. Truthful rejected/needs-work proposals need
not invent educational content.

Education is stored with the adopted candidate in PostgreSQL and returned as
`challenge.education`, including in exports. It is creator-authored context, not
an additional validation rule. No database migration or verifier update is needed;
executable manifests and their signed bytes retain their existing schema.

The two already-published showcases have dated, source-bound editorial notes in
`web/components/challenge-education.tsx`. Those notes are presentation editions in
public Git history, not modifications to their historical candidate, lock or
score. They only apply to the exact source commits reviewed. New challenges render
their own adopted educational context without a showcase-specific code change.

Load Paths uses interpolated SVG density contours by default. Its optional
Simulation cells view shows the actual finite-element density grid. Interpolation
and exaggerated displacement are explicitly illustrations, never new solver data.

Quiet Echoes meets the bar for a focused mathematical challenge: exact full-instance
validation and a published reference. A new lower-energy sequence would be a
concrete improvement over that reference, with novelty checked separately. It does
not yet support claims of better deployed radar or a general algorithmic advance.
Load Paths has physically interpretable numerical validation, but beating its
110-iteration reference alone is not a field-frontier result. The public pages now
make both distinctions explicit.

## Visualization requirement

Scout 1.6 requires the repository package in [visualization v1](specs/visualization-v1.md),
using the [brief template](templates/visualization-brief.md). The
[showcase briefs](showcase-visualizations.md) explain what each existing challenge’s
visualization should teach and distinguish current features from planned additions.
This is an authoring and scientific-review requirement; candidate lint does not
render previews, and generic creator-package hosting remains separate work.
Historical Load Paths discussion above describes its numerical demonstration;
that challenge remains withdrawn for failing the frontier-reference bar.
