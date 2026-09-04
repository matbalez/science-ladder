# Headless GitHub repository provisioning

`scripts/github-repository.py` creates a dedicated repository, pushes a reviewed source commit, and requests that **one repository ID** be added to an explicitly supplied GitHub App installation. It uses authenticated `gh` and Git locally. It never opens a browser, extracts a token, changes global Git credentials, deletes a repository, force-pushes, or expands an installation to all repositories.

```sh
python3 scripts/github-repository.py \
  --source /absolute/path/to/reviewed-source \
  --repository OWNER/REPOSITORY \
  --installation-id INSTALLATION_ID \
  --visibility public
```

Replace all three placeholders. Visibility defaults to **private** when omitted. Python 3.10+, Git and the GitHub CLI must already be installed and authenticated. A personal destination must belong to the authenticated user; this helper requires an active organization owner for an organization destination. Repository admin access is required. The installation ID is an explicit operator input, not guessed from App names.

A plain source directory becomes its own Git repository and receives an initial commit of files allowed by its `.gitignore`. An existing checkout must be its own root and have a clean, committed working tree. Review the complete source and any existing history before using the helper: a small credential-filename check is a tripwire, **not a complete secret scan**. Existing commits retain their complete reachable history. A directory nested inside another repository is rejected.

An existing destination is resumed only with matching local `origin` fetch/push URLs, exact owner identity, admin permission and unchanged visibility. The target branch defaults to the source branch, or `main` for a new directory; `--branch` overrides it. Only a normal fast-forward push of that branch is allowed. Divergent history, URL rewrites, unexpected destinations, forks and archived repositories are rejected. Git HTTP redirects are disabled so a rename or transfer cannot silently redirect a push. Inherited Git repository/index/object environment overrides are cleared, keeping operations within the explicit source. Tags and submodules are not recursively pushed. A newly created repository's origin is recorded immediately so a later failure can resume.

## Enrollment credentials and partial success

GitHub's add-repository endpoint requires a **classic personal access token with `repo` scope**, together with repository admin access. Its documented credential restriction excludes fine-grained PATs and GitHub App tokens. Ordinary GitHub CLI OAuth also failed with HTTP 403 in our deployment check. [GitHub endpoint documentation](https://docs.github.com/en/rest/apps/installations#add-a-repository-to-an-app-installation)

The helper uses the existing local GitHub CLI authentication mechanism, including an already-configured environment credential. It does not assert that every credential is in an OS keychain: GitHub CLI can fall back to its own plaintext file when secure storage is unavailable. The helper performs no login or credential-storage operation. [GitHub CLI authentication behavior](https://cli.github.com/manual/gh_auth_login)

An operator who chooses classic-PAT enrollment must configure that authorized credential locally; do not paste it into chat, a command argument, a source file or logs. The helper does not attempt to obtain one or broaden its scopes.

The helper does **not** require `/user/installations` discovery. That route has a different token boundary and failed with our ordinary CLI OAuth. Supplying an installation ID therefore does not assert that the script independently verified its App identity. GitHub authorizes or rejects the exact enrollment request. Only HTTP 204 or 304 counts as enrollment success.

It prints one JSON result. Exit `0` means the remote commit was confirmed and GitHub accepted enrollment. Exit `3` / `enrollment_blocked` means the source commit is preserved remotely but enrollment received HTTP 403. Check the credential type, installation ID, admin access and applicable organization policy, then rerun the identical command; repository creation and an already-matching push are skipped. Other failures exit `1` and retain completed work. A timeout can leave uncertain remote state; the same command inspects it on retry. The helper never deletes resources to undo a partial operation. If a process died between remote creation and recording origin, inspect the exact destination and configure its matching local origin before resuming.

## Authorization pattern for repeated challenge repositories

For a personal account, selected-repository installation grants keep access explicit. Each additional repository needs its own authorized enrollment. Repository creation through the API does not imply permission to broaden a selected installation.

For larger deployments, our recommended design is a **dedicated organization containing only Science Ladder repositories**, with an organization owner's explicit one-time approval for the App's required permissions and an all-repositories installation in that organization. This makes future repositories in that dedicated namespace part of the intended authorization boundary. All-repositories access is an installation option; this utility does not select or change it. [GitHub installation options](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)

That design requires separate operator approval to create/use the organization and choose its installation scope. No new organization, all-personal-repositories grant or production authorization is created by this script. Once an all-repositories installation is deliberately established, production provisioning should confirm installation coverage through the App's own authenticated API before reporting readiness; it should not require a classic-PAT enrollment for every new repository. The standalone helper here targets explicit per-repository enrollment.

## Tests

```sh
python3 -m unittest discover -s scripts/tests -p test_github_repository.py -v
```

Tests use mocked GitHub API responses and real disposable local Git repositories. They cover exact-ID enrollment, private default, conflict and visibility rejection, unrelated-owner denial, fast-forward-only history, no implicit tag push, idempotent resume, and preservation after a credential-scope failure. These tests are not a claim that live enrollment succeeded with the deployment's OAuth credential.
