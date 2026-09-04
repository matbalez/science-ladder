# Runtime advisory review — 2026-09-04

NOT APPROVED: published high/critical findings and unresolved severity/applicability remain. No deployment exception is issued by this report.

Runtime: `sha256:d4b4c0cea835f94c75033911451e266ea3d75d480ba808d4e03f5b0fd15aa12a`. Matched all 98 installed package coordinates (97 Debian binary packages and CPython 3.13.15); pip and vendored Python packages are absent.

Retained 108 unique advisory IDs: 3 critical, 15 high, 16 low, 30 medium, 12 moderate, 32 unknown.

Ratings use the published CNA assessment when present and published ADP CVSS otherwise. Conflicting ratings remain in the detailed report. Debian “Minor issue” and “unimportant” labels are not silently converted to clean findings.

## Retained high and critical findings

These are retained source/version matches after bounded refinement. Some still need a binary-feature or runtime applicability assessment; that uncertainty is not treated as permission to pass the gate. The PAM record is explicitly undetermined by Debian. The confirmed SQLite FTS5 feature alone is sufficient to prevent approval.

| ID | Severity | Installed packages | Primary record |
| --- | --- | --- | --- |
| CVE-2017-18018 | high | coreutils | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2017/18xxx/CVE-2017-18018.json) |
| CVE-2025-13151 | high | libtasn1-6 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2025/13xxx/CVE-2025-13151.json) |
| CVE-2025-69720 | high | ncurses-bin | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2025/69xxx/CVE-2025-69720.json) |
| CVE-2025-70873 | high | libsqlite3-0 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2025/70xxx/CVE-2025-70873.json) |
| CVE-2025-8941 | high | libpam-modules, libpam-modules-bin, libpam-runtime, libpam0g | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2025/8xxx/CVE-2025-8941.json) |
| CVE-2026-11822 | high | libsqlite3-0 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/11xxx/CVE-2026-11822.json) |
| CVE-2026-11824 | high | libsqlite3-0 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/11xxx/CVE-2026-11824.json) |
| CVE-2026-12087 | critical | perl-base | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/12xxx/CVE-2026-12087.json) |
| CVE-2026-13221 | critical | perl-base | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/13xxx/CVE-2026-13221.json) |
| CVE-2026-5435 | high | libc-bin, libc6 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/5xxx/CVE-2026-5435.json) |
| CVE-2026-54369 | high | libacl1 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/54xxx/CVE-2026-54369.json) |
| CVE-2026-5450 | critical | libc-bin, libc6 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/5xxx/CVE-2026-5450.json) |
| CVE-2026-57432 | high | perl-base | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/57xxx/CVE-2026-57432.json) |
| CVE-2026-5928 | high | libc-bin, libc6 | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/5xxx/CVE-2026-5928.json) |
| CVE-2026-76642 | high | bsdutils, libblkid1, libmount1, libsmartcols1, libuuid1, mount, util-linux, util-linux-extra | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/76xxx/CVE-2026-76642.json) |
| CVE-2026-78408 | high | bsdutils, libblkid1, libmount1, libsmartcols1, libuuid1, mount, util-linux, util-linux-extra | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/78xxx/CVE-2026-78408.json) |
| CVE-2026-78409 | high | bsdutils, libblkid1, libmount1, libsmartcols1, libuuid1, mount, util-linux, util-linux-extra | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/78xxx/CVE-2026-78409.json) |
| CVE-2026-78410 | high | bsdutils, libblkid1, libmount1, libsmartcols1, libuuid1, mount, util-linux, util-linux-extra | [record](https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/2026/78xxx/CVE-2026-78410.json) |

## Unresolved records

These retain unknown severity or applicability. Unknown is not an approval.

| ID | Installed packages |
| --- | --- |
| CVE-2005-2541 | tar |
| CVE-2007-5686 | login, passwd |
| CVE-2010-4756 | libc-bin, libc6 |
| CVE-2011-3374 | apt, libapt-pkg6.0 |
| CVE-2011-3389 | libgnutls30 |
| CVE-2011-4116 | perl-base |
| CVE-2018-20796 | libc-bin, libc6 |
| CVE-2018-6829 | libgcrypt20 |
| CVE-2019-1010022 | libc-bin, libc6 |
| CVE-2019-1010024 | libc-bin, libc6 |
| CVE-2019-1010025 | libc-bin, libc6 |
| CVE-2019-9192 | libc-bin, libc6 |
| CVE-2021-45346 | libsqlite3-0 |
| CVE-2022-27943 | gcc-12-base, libgcc-s1, libstdc++6 |
| CVE-2023-31438 | libsystemd0, libudev1 |
| CVE-2023-31439 | libsystemd0, libudev1 |
| CVE-2023-50495 | libncursesw6, libtinfo6, ncurses-base, ncurses-bin |
| CVE-2026-19499 | libc-bin, libc6 |
| CVE-2026-19542 | libc-bin, libc6 |
| CVE-2026-53613 | bsdutils, libblkid1, libmount1, libsmartcols1, libuuid1, mount, util-linux, util-linux-extra |
| CVE-2026-53615 | bsdutils, libblkid1, libmount1, libsmartcols1, libuuid1, mount, util-linux, util-linux-extra |
| CVE-2026-77117 | libc-bin, libc6 |
| CVE-2026-80489 | libc-bin, libc6 |
| TEMP-0000000-21C4F8 | libpcre2-8-0 |
| TEMP-0000000-64109B | libpcre2-8-0 |
| TEMP-0000000-8188AC | libpcre2-8-0 |
| TEMP-0000000-A5518C | libpcre2-8-0 |
| TEMP-0000000-B05303 | libpcre2-8-0 |
| TEMP-0290435-0B57B5 | tar |
| TEMP-0517018-A83CE6 | sysvinit-utils |
| TEMP-0628843-DBAD28 | login, passwd |
| TEMP-0841856-B18BAF | bash |

## Python findings

Five primary records are retained conservatively: CVE-2025-15367 (medium), CVE-2026-15806 (medium), CVE-2026-17084 (medium), CVE-2026-19672 (medium), and CVE-2026-15310 (low). An exact-commit OSV query returned only two of them; that response was not treated as exhaustive. The complete pinned PSF database contains 190 records. Git ancestry plus explicit version/platform evidence excluded 185.

## Evidence and limits

The review retains an unsigned scanner-compatible snapshot, per-package matching and exclusion decisions, all provider assessments, and 306 source responses with URL, fetch time and SHA-256 provenance. The raw collection is an operator evidence artifact. Local package source mapping, all 569 Debian version comparisons, the CPython release ancestry, and runtime component probes are hashed in the evidence report.

SQLite 3.40.1 with FTS5 enabled is confirmed in the actual pinned image. The SQLite findings require crafted databases and FTS5 queries. The runtime also contains ncurses infocmp and the Perl core/Socket module. These are not merely optional source-package matches.

No network and per-run VM isolation constrain consequences, but do not change upstream ratings. This report does not authorize bypassing the vulnerability gate. The host kernel, VMM and platform binary need their own current assessment.

The snapshot expires 24 hours after generation. Rebuilding or changing a runtime package requires regenerating the inventory and matching that exact image. Raw legacy pip/vendor query files are superseded and omitted from this final snapshot.

## Collection identity

- Review generated: `2026-09-04T22:10:23.832806Z`.
- Exact inventory SHA-256: `sha256:1ea4cd3634903d7ceda64f6ef949e0cb1be6c8d310d4d1c719d92d3b2e34c7f2`.
- CPython release commit: `4061bc4c35f7c26f25264666d4ba083b93d2f6f9`.
- Complete PSF advisory database pinned at commit `e903d0b393696d7f14f52fe9200d68c6fde0a715`.
- Package coverage: 94 fully matched to the retained database; four PAM binaries retain unresolved release applicability. This is database matching coverage, not a guarantee of security.

| Primary collection | Fetched at | SHA-256 |
| --- | --- | --- |
| [source](https://security-tracker.debian.org/tracker/data/json) | 2026-09-04T22:01:14.469178Z | `sha256:4b170c4745d92db1c3630a9660dfd5c919088dd8886749fe44baf11c9f8236a8` |
| [source](https://api.github.com/repos/psf/advisory-database/git/trees/main?recursive=1) | 2026-09-04T22:01:13.518788Z | `sha256:355f916637ab1d623a06aca6fe016f553ad38854361eb1dc7a3dd639cef6b446` |

This was a repository team review of public advisory data. It is not independent external security review, launch approval, proof of exploitability across the microVM boundary, or a claim of a vulnerability-free platform.
