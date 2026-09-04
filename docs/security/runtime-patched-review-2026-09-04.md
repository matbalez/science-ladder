# Patched runtime evidence — 2026-09-04

The exact published runtime below passes the **unsigned advisory-policy check** after package updates and an explicit retained-component applicability review. It still has **17 retained findings: 9 medium, 2 moderate and 6 low**. This is neither a vulnerability-free claim nor an official challenge-acceptance receipt. The copied scanner report explicitly records `signatureVerified: false` and `officialAcceptance: false`.

| Binding | Exact value |
| --- | --- |
| Published OCI image | `ghcr.io/matbalez/science-ladder-python@sha256:a8136bf6f5082a72776f2565f279a6c336b462da483aa54cfa81586ad20705fa` |
| Runtime inventory file | `sha256:0dbe6e677aecaf5ee204bd49d4f167f6496bcbb37c6d8348daf23dd2657bcd2a` |
| Embedded component inventory | `sha256:188492919acca87e0b11fde5b24464a2511e886a551d987a9f8583e879745226` |
| Coverage | CPython 3.13.15 plus 17 Debian binary-package coordinates; 2,134 retained-file entries |
| Advisory snapshot | `patched-debian-review-20260904T223626Z` |
| Snapshot expires | `2026-09-05T22:36:26.017165+00:00` |

The protocol agent pulled the published image, confirmed its complete embedded component inventory matched the tested local build, and reran local checker/hidden-preview/output integration checks. The inventory-file digest includes its trailing newline; the embedded component digest covers the embedded JSON bytes without the exporter's added newline. [Exact inventory](runtime-patched-2026-09-04/runtime-inventory.json), [binding and review summary](runtime-patched-2026-09-04/review-summary.json)

## What changed

The distroless build now retains glibc `2.43-4`, SQLite `3.53.4-2`, ncurses `6.6+20260608-2`, libuuid `2.42.2-4` and base-passwd `3.6.8`, alongside the unchanged coordinates listed in the inventory. These are actual retained libraries and files from the build, not edited version labels. Changed source versions were compared against the archived Debian fixed-version boundaries with `dpkg --compare-versions`; unchanged coordinates retained their previous assessment. [Exact comparison results](runtime-patched-2026-09-04/debian-version-comparisons.json)

The updated glibc crosses the published fixed-version boundaries for [CVE-2026-5435](https://security-tracker.debian.org/tracker/CVE-2026-5435), [CVE-2026-5450](https://security-tracker.debian.org/tracker/CVE-2026-5450) and [CVE-2026-5928](https://security-tracker.debian.org/tracker/CVE-2026-5928). SQLite crosses the `3.53.2-1` boundary for both [CVE-2026-11822](https://security-tracker.debian.org/tracker/CVE-2026-11822) and [CVE-2026-11824](https://security-tracker.debian.org/tracker/CVE-2026-11824). Their previous high/critical ratings are preserved in the evidence; the new versions address the findings.

Other exclusions are explicit applicability decisions. The inventory and probes support absent gconv converters, the absent optional SQLite zipfile extension, and absent mount/nsenter code where only libuuid is retained. `libc-bin` contributes locale/configuration/copyright data; executable glibc remains independently assessed as `libc6`. Eight named records use vendor non-security or disputed-vulnerability determinations. This is not a blanket conversion of Debian severity labels into a pass. Every such exclusion and its rationale appears in the [review summary](runtime-patched-2026-09-04/review-summary.json).

No published severity score was reduced, and confinement alone was not used to dismiss a finding. A later image with additional components needs a new inventory and assessment; these exclusions do not automatically transfer.

## What remains and what was tested

| Retained package | Medium | Moderate | Low |
| --- | ---: | ---: | ---: |
| CPython | 4 | 0 | 1 |
| libbz2-1.0 | 1 | 0 | 0 |
| libc6 | 2 | 0 | 0 |
| libssl3 | 1 | 2 | 4 |
| libuuid1 | 1 | 0 | 0 |
| zlib1g | 0 | 0 | 1 |

The [exact scanner output](runtime-patched-2026-09-04/scanner-report-unsigned.json) lists each advisory and its primary source. The remaining glibc CVE-2026-19499 uses a published [Red Hat assessment](https://bugzilla.redhat.com/rest/bug/2523258); it is not silently marked clean because an earlier record lacked a severity.

All 77 native extension imports enumerated by the component probe succeeded. That probe also confirmed runtime glibc 2.43, SQLite 3.53.4, unavailable SHIFT_JISX0213/EUC_JISX0213 gconv converters, and no SQLite zipfile module. The probe ran against the local build whose complete component digest matches the published image; it is not presented as an independent hardware test. [Exact component probe](runtime-patched-2026-09-04/component-probes.json)

The bundle contains the complete unsigned snapshot with 18 coverage entries and 311 primary source URL/fetch-time/content-digest records. Every recorded raw response digest was rechecked against the retained bytes before this public copy was assembled. Large raw downloads and private workstation paths are omitted. [Unsigned snapshot](runtime-patched-2026-09-04/advisory-snapshot-unsigned.json), [file digests](runtime-patched-2026-09-04/evidence-index.json)

## Earlier failures remain part of the record

The [initial 98-package review](runtime-advisory-review-2026-09-04.md) remains unchanged and blocked. The first 18-package distroless review also remained blocked, with 55 unique advisories including 9 high, 1 critical and 14 unknown; its [exact historical report](runtime-patched-2026-09-04/initial-distroless-blocked-review.json) is retained. The current result belongs to the new image and its reviewed applicability evidence, not a retroactive approval of either earlier image.

This is a repository-team assessment of the retained runtime, not independent external security review. Operator approval and signed artifact/job bindings remain separate from this unsigned policy check. The host kernel, Firecracker, platform services and deployed configuration require their own evidence; this document does not certify them or assert independent replication.
