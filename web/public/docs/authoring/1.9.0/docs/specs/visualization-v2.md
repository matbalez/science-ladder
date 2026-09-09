# Science visual specification v2

**Every new challenge needs at least one visual explanation of the science.
Interactivity is optional.** A labeled diagram, annotated plot, geometric
construction or illustrated example is sufficient. It should help a reader
understand the scientific object, process or relationship involved. Decoration
and a leaderboard alone do not satisfy the requirement.

This supersedes the universal preview-package requirements in v1. Scout 1.7 uses
this standard. The existing interactive showcases remain useful; they do not set
a minimum engineering burden for every new creator.

## Default: a static visual

Supply an image plus `visualization/visual.json` in the challenge repository:

```json
{
  "version": "science-ladder/science-visual/v1",
  "kind": "static",
  "asset": "visualization/science.svg",
  "caption": "A short explanation of what the figure shows and why it is relevant.",
  "alt": "A concise accessible description of the image.",
  "description": "A fuller plain-text explanation of the scientific relationships, labels and any simplifications. This also provides context for the learning guide.",
  "sources": [],
  "credit": "Original diagram by the challenge creator; MIT."
}
```

Use real file paths and actual attribution. `sources` contains inspected source
URLs when the figure is adapted from research or depicts published data. An
original schematic may use an empty list; label it as a schematic rather than
inventing data or provenance. The proposed asset formats are SVG, PNG, JPEG and
WebP. Choose a resolution that keeps labels readable at mobile and desktop sizes.
For plots and diagrams, prefer vector geometry where practical.

The caption explains the scientific relevance. The accessible description and
longer explanation let readers learn without seeing the image, and give the inline
agent enough context to answer questions. A plot must identify quantities, units
and any relevant uncertainty; a schematic should identify its simplifications.
If the visual displays a reference score or measured result, cite the actual data
and reproduce those numbers. A conceptual illustration need not compute the metric,
reproduce a checker, depict the full frontier, or provide a verification interface.
The challenge's education and evaluation sections carry the fuller argument.

Inspect the image for scientific correctness, legibility, accessible description
and redistribution rights. The creator can record this inspection in the normal
challenge brief. No HTML preview, build command, JavaScript, control/reset design,
`reference.json`, context schema, `getContext()` method, or separate browser-test
report is required for a static visual. List the image and descriptor in the existing
candidate `repositoryPlan`; do not add unsupported candidate or manifest fields.

## Optional: an interactive visual

Use interaction when changing a parameter, selecting a step or exploring a
construction helps explain the science. Retain a static fallback and the same
caption, accessible description, scientific explanation and source attribution.
The descriptor has `kind: "interactive"`, `asset` pointing to the fallback image,
and `entrypoint` pointing to a local preview such as `visualization/index.html`.

The interactive implementation and isolation guidance in
[visualization v1](visualization-v1.md) applies only to this optional path: local
assets, bounded browser computation, keyboard/touch access, sensible reset behavior
and current display context for the learning guide. Include reference data and
numerical checks when the visualization actually computes or displays quantitative
results. A diagram with expandable explanations need not invent a numerical dataset.

A simple “Your exploration” label is sufficient when a reader changes inputs and
sees local numbers. The visual's purpose is explanation on the challenge description
page. Do not add a second verification workflow or blanket scientific disclaimers.
Actual submissions and receipts continue through the existing platform workflow.

## Review standard

Reviewers ask whether the visual helps explain the science, whether its labels and
claims are accurate, whether it is accessible, and whether its use is permitted.
Accept a good static visual without requiring controls, dynamic agent context or
an executable preview. If a visual is not supplied in the review evidence, do not
claim to have inspected it. A text-only reviewer can assess the description and
sources; actual image inspection must be recorded honestly. The visual need not
independently restate every frontier or significance argument on the page.

The current specification is an authoring/review requirement. Candidate lint still
checks the candidate/manifest contract; it does not render or certify images. Earlier
Scout proposals and existing scientific locks remain readable. The simpler standard
also applies when reviewing an older proposal now; do not force its creator to build
interaction merely because an archived prompt asked for a preview package.

## Implementation plan and current status

The current site still has manually integrated showcase visuals. Automatic
creator-asset display is not yet implemented. The next integration should start
with the static path:

1. Read and validate the descriptor and local asset from the pinned public challenge
   package. Preserve their exact bytes/digests and bind the presentation to the
   challenge version; keep this separate from the scored manifest.
2. Store and serve the reviewed asset through the existing artifact storage, with
   image type/size checks. Render it as an image with caption and accessible text in
   the standard challenge-page position. Treat SVG as image content, never inject
   arbitrary creator SVG/HTML into the application DOM.
3. Supply the caption and explanation to the learning guide automatically. Static
   visuals need no selection-state bridge or creator JavaScript.
4. Verify the full create → review → publish → display flow with a new static package
   and no challenge-specific website changes. Keep the existing interactive showcases
   working; migrating them is not a prerequisite for the simpler static path.
5. Add an isolated interactive-package host as a separate enhancement, reusing the
   same caption/fallback layout and adding a bounded state bridge when useful.

Changing the standard does not itself build this loader. Do not reject an otherwise
complete static authoring package solely because the platform's automatic hosting
work is pending; track that platform readiness separately and communicate it before
publication. Never claim an asset is already displayed when it is not.
