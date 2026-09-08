# Quiet Echoes frontier check — September 8, 2026

Retain the published challenge. Its baseline is the best published comparable
length-512 result located and reproduced in this check: energy **17,996**.
This is a frontier construction, not an intentionally limited solver run.

Exact scope: 512 unrestricted binary signs, aperiodic autocorrelation, summed
squared sidelobes over all 511 positive lags. The checker uses integer arithmetic.
The merit factor is 131072/17996 ≈ 7.28339631. Peak sidelobe is a separate property.

Primary sources inspected:

- [Dual-Step Optimization, Table 2](https://arxiv.org/html/2409.07222v1):
  provides the length-512 construction and merit factor 7.2834. The challenge's
  pinned source reproduces its full sequence and energy.
- [Prioritizing Search Space Regions, 2026](https://arxiv.org/html/2607.09688v1):
  subsequent improvements in the surrounding length range; no improved 512 row.
- [Massively Parallelizable Memetic Tabu Search](https://arxiv.org/html/2504.00987v1):
  improved smaller instances, not a superseding length-512 construction.
- [Quantum-enhanced Memetic Tabu Search](https://arxiv.org/html/2511.04553v1):
  no superseding length-512 construction located.
- [Discrete optimization of binary signals, 2026](https://lib.physcon.ru/doc?id=f2418d1fcdc9):
  experiments at other lengths, not a stronger comparable 512 reference.

The frozen challenge source remains
f42f527e97563b1c068a1835732c6da44f21223f. No manifest, score, lock or signed receipt
is changed by this editorial check. A lower-energy sequence would improve this
substantive research reference. Refresh current public results before claiming
an unqualified world record. Single-host platform verification remains the
agreed acceptance policy; a second physical server is not required.
