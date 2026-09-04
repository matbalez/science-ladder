# Attribution and data rights

All newly written checker, tests, tools, visual explorer and documentation are MIT licensed. Mathematical definitions are implemented independently; no published solver code is copied or executed.

The 512-bit baseline in `fixtures/baseline/sequence.txt`, the extracted hexadecimal row in `literature/reference.json`, and the corresponding embedded reference in `visualize.html` are adapted from:

Blaž Pšeničnik, Rene Mlinarič, Janez Brest and Borko Bošković, *Dual-Step Optimization for Binary Sequences with High Merit Factors*, arXiv:2409.07222v1, Table 2, length 512 row. [Source](https://arxiv.org/html/2409.07222v1), [CC BY 4.0 license](https://creativecommons.org/licenses/by/4.0/).

Changes: extracted one mathematical sequence; converted each hexadecimal digit to four binary digits and mapped 1 to `+`, 0 to `-`, with one final LF. Recomputed energy and peak sidelobe, and added original visualization. The authors do not endorse this challenge. Attribution does not imply a claim that their reference is globally optimal or remains the best published sequence indefinitely.

The 2026 follow-up paper and circle-packing/Golomb sources are cited as evidence. Only short attributed exact evidence excerpts are included in the candidate and manifest. Figures and solver implementations are not included. Source response hashes are retained in `docs/scout-provenance.json`; source bodies remain separate research intermediates.

Submission data use CC BY 4.0 so a properly attributed reproduction or derivative can be shared consistently. Solvers must retain applicable third-party attribution and accurately identify whether their sequence is new, reproduced, or modified from a reference. A data license is not evidence of novelty.
