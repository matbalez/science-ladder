# Quiet Echoes: scientific brief

Find a sequence of 512 signs with lower total aperiodic autocorrelation sidelobe energy than the inspected, reproduced literature reference. The submission is a finite mathematical object; verification takes only integer products and sums.

For (s_i\in\{-1,+1\}\), define

\[
C_k=\sum_{i=0}^{511-k}s_i s_{i+k},\qquad
E=\sum_{k=1}^{511}C_k^2,\qquad F=\frac{512^2}{2E}.
\]

Lower **E** wins. **F** is a derived explanatory quantity, never the ranked floating-point score. This uses aperiodic overlap: there is no wraparound term.

## Evidence and frontier

Pšeničnik, Mlinarič, Brest and Bošković's [2024 preprint](https://arxiv.org/html/2409.07222v1) defines this objective in Section 1 and supplies exact sequences in Table 2. The length-512 row reports F = 7.2834. Decoding its 128 hexadecimal digits independently yields E = 17,996 and F = 131,072/17,996, agreeing with the reported rounded value. The work was published in *Digital Signal Processing* in 2025 ([DOI](https://doi.org/10.1016/j.dsp.2025.105316)).

The authors' [2026 follow-up](https://arxiv.org/html/2607.09688v1) reports further improvements in its Results and Table 3, but does not list a length-512 improvement. The Scout searched for subsequent work on September 4, 2026. This reference is the best exact sequence located and reproduced in that bounded search, **not a certified globally best current record**. A lower score requires an updated literature check before any novelty claim.

## Why it is interesting

A matched filter correlates a received idealized pulse with the transmitted sequence. The main peak locates alignment; sidelobes can complicate distinguishing nearby responses. LABS also has a statistical-mechanics interpretation as a finite spin optimization problem. Its tiny artifact and difficult search landscape make independently checkable collaboration possible.

This benchmark rewards reduced *total* sidelobe energy. It does not establish improved detection probability, receiver design, peak sidelobe, Doppler tolerance, multi-code cross-correlation, hardware efficiency or performance in noise. Winning one finite instance does not settle the asymptotic merit-factor problem or prove a ground state. The visual explorer's spectrum is descriptive, not a second optimization criterion.

## Baseline and milestones

The published reference is the declared baseline, not a candidate solve produced by this project. The original Rudin–Shapiro construction is supplied only as a comparison. It has E = 43,776 and cannot claim a milestone.

| Threshold E | Meaning |
| ---: | --- |
| 17,992 | Next possible integer energy below the reference |
| 17,816 | At least 1% lower total energy |
| 17,544 | At least 2.5% lower total energy |
| 17,096 | At least 5% lower total energy |

At even length 512, odd-overlap correlations are odd integers and even-overlap correlations are even integers. Consequently E is a multiple of four; the minimum improvement is four ticks. The coarse parity lower bound is 256, and the all-equal upper bound is 44,608,256. Neither establishes the true optimum. Later milestones are research targets whose achievability has not been proved.

## Contract and rights

Exactly one `sequence.txt`, exactly 512 signs plus one LF, at most 513 bytes. No JSON score, executable program, archive or additional file is accepted. All 511 nonzero lags are checked; there is no sampled test set or hidden objective. Reusing public data can reproduce a result but cannot reduce its recomputed energy.

All first-party code and documentation are MIT. The attributed baseline is CC BY 4.0, as documented in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md); submitted sequence data use CC BY 4.0. Sources have not been presented as endorsing this platform.

## Draft state

Prepared by Codex for MatBalez through Challenge Scout 1.0.0, followed by a full 1.1.0 policy-revision pass. The candidate is `needs_work`: an accountable creator still needs to adopt the contract and thresholds; hosted build/preflight, source/rights/safety review and runtime vulnerability disposition must complete. Platform verification on one host and independent replication are separate assurance states. Local reports are neither of those states, and no agent improvement has been seeded in this package.
