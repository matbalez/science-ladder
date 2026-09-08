# Visualization briefs for the published showcases

The current [science visual v2 standard](specs/visualization-v2.md) requires a visual
explanation of the science; interactivity is optional. Each challenge below has a
static minimum and a richer optional design. Existing interactive features can
remain. Planned controls are enhancements, not publication requirements. These
notes do not change scientific source versions or verification contracts.

## One Less Multiply

**Sufficient static version.** An annotated A × B = C diagram highlighting one
worked multiplication and its contribution to the output. Explain how combining
linear forms allows products to be reused. Add a caption and accessible explanation;
no input editor, product selector or browser arithmetic is required.

**Learning goal.** Understand how additions and reuse allow all nine entries of a
3×3 matrix product to be assembled from 23 bilinear products, and why finding an
exact 22-product construction would improve the finite algebraic frontier.

**Main figure.** Three labeled 3×3 grids: input A, input B and output C. Selecting
one of the 23 reference products highlights the signed linear combinations of A
and B that are multiplied, the resulting scalar, and its signed contributions to C.
Show the complete output alongside the selected contribution so a single term is
never mistaken for the answer. Use signs/labels as well as color. A compact view
of ordinary 27-product multiplication can establish what is being reduced.

**Controls.** Select product, step forward/back, edit small bounded integer inputs,
and Reset reference. Input editing is a planned extension of the current product
selector. Each control should help explain linear combinations, cancellation or
reuse. Demonstrating the identity on chosen inputs is an illustration, not proof
for all inputs.

**Meaning and limits.** The primary metric counts products of input linear forms.
An exact 22-product formula would improve the reproduced 23-product reference.
The full verifier checks all 729 coefficient identities, rather than a handful of
numeric examples. Additions, coefficient size, stability, memory movement and actual
runtime are separate concerns. Product-count improvement alone establishes no
practical speedup or best asymptotic matrix multiplication method.

**Implementation and data.** Use responsive HTML/SVG grids with public U/V/W
coefficient arrays exported from the exact rational reference. Small-integer preview
arithmetic can use bounded exact integers; otherwise display an approximation notice.
The current first-party implementation is `web/components/multiply-explorer.tsx`,
with the source-bound reference in `web/lib/multiply-reference.ts`. The proposed
portable package must derive from those scientific source bytes, with their license
and attribution, rather than copying rounded screen values.

**Guide state.** Input matrices, selected product number, coefficient signs, the two
linear-form values, their product, contribution grid, complete output and whether
inputs have been edited. Include the distinction between a worked example and the
729-identity certificate.

**Acceptance anchors.** With A = 1…9 and B = 9…1 in row order, selecting product 23
gives 1 × 6 = 6 and contributes +6 to C23; the complete C23 is 54. Summing every
reference contribution must reproduce the ordinary product for the bounded input
fixtures. A deliberately corrupted coefficient must be described as failing the
identity check. Selection/input changes must appear in the next guide snapshot.

## Smallest Triangle

**Sufficient static version.** A labeled point configuration in a square with one
smallest triangle shaded. Explain that every triple forms a triangle and the aim is
to make the smallest one larger. A conceptual example may use fewer points if it
is labeled clearly. Dragging points and recalculating areas are optional.

**Learning goal.** Understand max–min optimization: moving one point can improve
some triangles while making another smaller. A better configuration must raise
the smallest triangle among all 364 triples of 14 points in the unit square.

**Main figure.** A square with 14 numbered points and the selected minimum-area
triangle shaded. Display its area and count of tied bottlenecks; allow inspecting
the 26 reference bottlenecks. An optional area distribution over all 364 triangles
can explain why optimizing one appealing triangle is insufficient. Provide a
reference/edited comparison on the same scale.

**Controls.** Drag a point or move it with keyboard controls; select a tied minimum;
Reset reference. Planned additions: direct coordinate inputs and an optional
reference overlay. Clamp points to the declared domain and show repeated/degenerate
points honestly; do not silently repair an invalid configuration.

**Meaning and limits.** Maximizing the minimum area would improve the constructive
lower bound for this fixed Heilbronn triangle problem. The substantiated reference
is approximately 0.0243039796209924867482…; its exact algebraic description is what
the checker compares against. Browser decimals are educational approximations.
Raising a preview number is not an official record, proof of optimality or a result
for every number of points. Significance belongs to extremal geometry and the study
of geometric configurations; do not invent engineering applications.

**Implementation and data.** SVG geometry with numbered controls and a coordinate
table. Recompute all 364 areas for local edits; preserve the exact source reference
separately from drawing coordinates. Describe the tie tolerance used in the preview
and any floating-point ambiguity. Current source-bound rendering and reference:
`web/components/smallest-triangle.tsx`, `web/lib/triangle-reference.ts`.

**Guide state.** The 14 coordinates, edited/reference state, selected triple,
minimum preview area, tie count, units and approximation notice. If describing a
movement rather than the current geometry, also supply previous coordinates or a
small explicit last-change record; the guide must not guess a movement from one
snapshot.

**Acceptance anchors.** The unedited preview must reproduce the reference minimum
within its stated drawing tolerance and show 26 ties. Test boundary points, duplicate
points, a degenerate triple and the exact reference fixture. Reset must restore the
original coordinates. Compare representative browser computations against exact
checker calculations, and keep the official comparison bound unrounded.

## Quiet Echoes

**Sufficient static version.** A binary sequence beside a shifted copy, with the
matching and opposing terms marked, plus a small annotated correlation plot or
schematic. Explain how unwanted shifted matches create sidelobes. A short teaching
sequence is sufficient when identified as illustrative; interactive sign flipping
and displaying all 512 signs are optional.

**Learning goal.** Understand aperiodic autocorrelation: the unwanted match between
a binary sequence and a shifted copy. Learn why reducing total squared sidelobe
energy is useful, and why it does not necessarily reduce the single worst sidelobe.

**Main figure.** A compact strip of 512 +1/−1 signs, a selected shifted overlap,
and the off-peak correlation plot. Highlight the selected shift and show how the
products in that overlap sum to C(k), then contribute C(k)² to the full energy.
Show E = sum over k=1…511 of C(k)² and the reference E = 17,996. Omit the zero-lag
peak from the energy and explain why. A full-range overview prevents zooming from
concealing which shifts contribute to the score.

**Controls.** Select a shift, zoom the displayed lag range, restore full range and
Reset reference. A local sign-flip mode is a proposed extension: label it Exploration
and compare total energy and peak sidelobe on shared scales. The chart's zoom must
never change which lags contribute to E. A selected overlap may be scrolled or
summarized, with its full length stated.

**Meaning and limits.** Lower E improves the exact low-autocorrelation objective for
this length-512 binary sequence. If shown, merit factor is 512²/(2E), not a different
independent result. The dated reference does not establish optimality. Total energy,
peak sidelobe, noisy receiver performance and Doppler response are distinct; the
visualization must not imply an experimentally proven radar benefit.

**Implementation and data.** Use the exact published sign sequence and integer
correlation calculations. SVG is sufficient for the lag plot; a compact canvas or
scrollable strip can show the 512 signs with a text equivalent. Keep the selected
shift decomposition separate from the full score. Existing displays live in
`web/components/binary-pulse.tsx` and the source-bound Quiet Echoes showcase page.
The selected-overlap explanation and portable package require additional work.

**Guide state.** Visible lag range, selected shift, overlap length, correlation at
that shift, its squared contribution, full energy, peak sidelobe, merit factor,
reference/edited state and any sign changes. Bound the snapshot; do not send every
pairwise product when a summary and selected entries suffice.

**Acceptance anchors.** The frozen sequence reproduces E = 17,996. A short labeled
teaching sequence must match hand-computed aperiodic correlations. The zero-lag
term must not enter E; changing the displayed lag range must leave E unchanged.
For sign edits, recompute all contributing correlations and distinguish local
preview output from any published verified artifact.

## Applying the standard to other scientific problems

For simulation or optimization, show the scientific system, constraints, reference
and observable tradeoff, with modeling assumptions and uncertainty. For proof
search, use a dependency graph, geometric construction or small worked instance,
clearly distinguishing certified statements from remaining obligations. For paired
performance measurements, show the quality gate and measured distributions with
uncertainty and hardware conditions. A racing timer or cherry-picked fastest run
is not sufficient. For every domain, explain how the visual feature supports the
actual scientific claim and what evidence would be needed to go beyond it.

Load Paths remains withdrawn because its scientific reference did not meet the
frontier bar. Improving its graphics is not grounds to republish it.
