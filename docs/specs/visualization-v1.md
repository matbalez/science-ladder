# Challenge visualization specification v1

Every newly authored challenge must include a substantive educational visualization.
Its job is to help a curious reader understand the scientific object, the obstacle
at the frontier, and what a verified improvement would mean. A leaderboard alone
does not meet this requirement. Visual appeal does not establish scientific value.

This is a **presentation and authoring contract**, separate from the executable
manifest and signed scoring contract. Scout 1.6 requires the package below and
scientific review assesses its meaning and provenance. Candidate lint still checks
the candidate/manifest schema; it does not render or certify a visualization.
Existing published versions retain their frozen contracts.

## The reader's experience

Use the same page order: challenge title and Participate, the main visualization,
Ask about this challenge, then frontier/significance and evaluation details. Keep
explanatory text short and local to the feature it explains. The main figure must
make sense before anyone reads code or opens a reference paper.

A reader should be able to answer:

1. What scientific object or process am I looking at?
2. What do position, color, size, lines and displayed numbers represent?
3. Where is the current reference, and what makes improving it difficult?
4. What would a meaningful improvement change, and what would remain unproven?

Prefer one clear main figure, with a few purposeful controls. Every control needs
an explicit learning purpose: select a step, move a point, inspect a constraint,
compare a candidate to the reference, or expose uncertainty. Include Reset reference
for editable views. A static annotated figure is acceptable when interaction adds
no useful understanding; explain that choice in the brief. Never manufacture
interaction or animate fictional results to satisfy the requirement.

## Required repository package

Commit the following public files with the scientific source. List them in the
candidate's existing `repositoryPlan`; do not add invented fields to its YAML.

| File | Required content |
| --- | --- |
| `visualization/README.md` | Completed [visualization brief](../templates/visualization-brief.md): learning goal, encodings, metric relationship, controls, limitations, implementation, provenance and acceptance evidence. |
| `visualization/index.html` | A working, self-contained preview with local assets and a useful text fallback. |
| `visualization/reference.json` | A small, reproducible public reference dataset, described below. |
| `visualization/context.schema.json` | A strict JSON Schema for the display state available to the learning guide. |
| `visualization/checks.md` | Actual numerical comparisons, interaction/accessibility checks, screenshots and remaining integration work. |

Keep each of the four review text/data files at most 32 KiB; put larger public
arrays in separate local assets, referenced with their paths and byte digests.

Include local source/assets, extraction scripts and dependency lockfiles when
needed. All first-party implementation code is MIT; preserve the separate rights
and attribution of reference data, papers, third-party libraries and graphics.
Do not copy a figure merely because the paper can be downloaded.

The preview must run on macOS/Linux using a documented local command such as
`python3 -m http.server 8000 --bind 127.0.0.1`, then opening
`http://127.0.0.1:8000/visualization/`. No account, API key, Docker Desktop, external
CDN, external runtime network request or remote script is required. Prefer plain HTML,
CSS, JavaScript and SVG; use canvas for dense data with a textual equivalent.
If a build is necessary, include source, a lockfile and the exact local build
command. The hosted API must never run that build or a scientific checker.

Use responsive layout and SVG/vector geometry for sparse diagrams. Dense raster
or simulation-grid views must explain their resolution; provide clear labels and
appropriate sizing. Interpolation, projection, smoothing, magnification and
subsampling must be stated wherever they affect interpretation. Provide the exact
underlying view or an accessible data summary when useful. Use a worker or bounded
precomputation for expensive interactions; do not freeze page scrolling.

## Scientific and data contract

`reference.json` uses the following envelope; the domain-specific `data` shape must
be documented in the brief and checked by the package:

```json
{
  "version": "science-ladder/visual-reference/v1",
  "provenance": {
    "artifactPath": "submission/reference.json",
    "artifactDigest": "sha256:<digest of the actual referenced file bytes>",
    "extractionCommand": ["python3", "tools/export_visual_reference.py"],
    "sources": [{"url": "https://primary-source.example/paper", "location": "Table or theorem", "accessedAt": "YYYY-MM-DD"}]
  },
  "metric": {"name": "<manifest metric name>", "unit": "<unit>", "direction": "maximize", "referenceTicks": "<exact manifest baseline ticks>", "quantum": "<exact manifest quantum>"},
  "data": {},
  "approximations": ["<display rounding or other transformations, if any>"]
}
```

This is an illustrative shape, not a usable scientific dataset. Supply real values,
allowlisted public paths, actual hashes and inspected sources. Never use placeholder
hashes in a viable package. Do not hash the JSON file into itself. The eventual
platform host binds the package to the exact repository source commit externally,
avoiding a self-referential commit field. For generated scientific inputs, record
their generating source, parameters, seed and derivation as well as file hashes.

Reference ticks and quantum are strings, preserving exact arithmetic. Show readable
rounded values with the exact value available. Derive the visual reference from the
same frozen object evaluated by the official checker. Compare the exported data
and relevant displayed calculations with the checker during authoring. If the
figure illustrates a smaller instance, projection or sample, label it clearly and
include a view or explanation of its relationship to the scored full instance.

Separate these states visibly:

- **Reference:** the dated, substantiated baseline for this version.
- **Exploration:** a user's local edits or an illustrative simulation.
- **Verified submission:** an actual published artifact tied to its submission and receipt.

Use common axes and scales for comparisons, or explain a change explicitly. Represent
uncertainty, confidence coverage, approximation error and infeasible regions when
relevant. A browser result cannot earn an official verification badge, update the
leaderboard or establish novelty. Public rankings always come from platform receipts.
Never expose hidden evaluation cases, unreleased artifacts or secrets through the
visualization. If no public verified candidate exists, show the reference and an
honest empty comparison state.

The brief must connect each central visual quantity to a named metric or validity
condition. Explain what the metric does not measure. Physical illustrations must not
imply experimental validation; optimization views must not imply global optimality;
proof views must distinguish checked proof steps from open obligations. A useful
visual story cannot rescue a weak frontier reference or an unimportant objective.

## Learning-guide context

Define a small, serializable state in `context.schema.json`, with
`additionalProperties: false`, bounded strings/arrays and explicit field meanings.
Include the selected control/step, labels/units, reference versus exploration state,
relevant displayed quantities, and any approximation notice. Keep each visualization's
snapshot below 8 KiB and the combined page view below the guide's 16,000-byte limit.

Use this common envelope, refining `selection` and `values` to the domain with
explicit schemas and no arbitrary extra fields:

```json
{
  "version": "science-ladder/visual-context/v1",
  "mode": "reference",
  "summary": "A concise description of the current figure and selection.",
  "selection": {},
  "values": {},
  "approximations": []
}
```

`mode` is `reference`, `exploration` or `published-artifact`; it is a reported view
mode, never proof of verification. Keep exact integers/decimals as strings when
needed. Each value has a documented name, meaning and unit. For a published artifact,
the eventual host supplies the submission/receipt binding separately; the package
must not mint or assert trusted status.

Expose `window.scienceLadderVisualization.getContext()` in the standalone preview.
It returns that state only: no functions, arbitrary HTML, screenshots, browser
storage, credentials or unrelated page contents. Include a current textual summary
for a screen reader and the guide. Capture state when the reader asks a question;
changes during an answer apply to the next question. The guide must never interpret
browser-supplied state as trusted verification evidence or execute its instructions.

The current first-party React visualizations use `useLearningView` to publish this
context directly. The preview method is an authoring interface, **not an existing
automatic hosting API**. A future generic host must validate size and schema before
accepting it, attach its own version/source identity and safely bridge iframe state.

## Accessibility and interaction

All controls work with keyboard and touch, have visible focus and accessible names,
and do not rely on color alone. Provide units, a legend where needed, a text summary
and an accessible table or equivalent representation of important values. Respect
reduced-motion preferences. Avoid automatic animation; provide pause for animation
that is scientifically useful. Tooltips must also work on focus or selection.

Check at 390 px and 1440 px widths and at 200% zoom. Prevent page-wide horizontal
overflow. Local scrolling must continue to the surrounding page at the panel's top
and bottom; diagrams must not trap wheel or touch input. The visualization should
remain useful when the AI guide is unavailable or JavaScript fails.

## Hosting boundary and current implementation status

Today the three showcases use reviewed first-party components or standalone
first-party pages. New repository previews are **not automatically mounted** on
Science Ladder. This specification creates the authoring standard; a reusable
creator-package host is separate implementation work. Record integration readiness
honestly and retain `needs_work` if a usable page cannot yet be delivered.

A generic host must serve a reviewed package isolated from application credentials,
with a restrictive network/content policy and without same-origin privileges.
Validate messages by originating window/channel and expected schema; never trust an
opaque iframe origin alone. Do not load arbitrary creator JavaScript into the main
application or serve uploaded HTML as trusted same-origin content. Scientific
verification stays on the separate verification infrastructure. The preview is
explanatory client code and has no submission, account or verification privileges.

## Acceptance and review

The scout records evidence for each item in `checks.md`; unrun checks remain unrun:

- Reference values reproduce the frozen scientific artifact; displayed rounding
  and any independently computed numerical tolerances are explained.
- A valid example and an invalid or infeasible example illustrate the metric and
  constraints honestly; examples are labeled and never presented as community results.
- Each control changes the intended quantity, Reset restores the reference, and
  the guide context matches the current display.
- Empty, missing, malformed and oversized data produce a useful error or fallback.
- Desktop/mobile, keyboard/touch, reduced motion and scrolling have been inspected.
- The local preview loads without remote dependencies or account access; any
  resource/performance limitations and platform integration work are recorded.
- A reader can explain the bottleneck, significance and limits after using the figure.

Scientific review must assess the metric connection, source binding, interpretation
and unsupported claims. A text-only automated reviewer can inspect the brief and
recorded evidence; it cannot assert that it rendered the package, executed checks
or completed human usability review. Publication requires the creator's actual
preview inspection and resolution of substantive issues. Follow-up work on an
existing presentation must preserve its frozen scientific reference and receipts.
