# gluper

**Plan and run safe upgrades of self-managed GitLab.**

gluper inspects a GitLab installation, works out the correct upgrade path
(required stops, minimum patch versions, PostgreSQL and OS cut-offs), refuses
unsafe upgrades, and runs upgrades that can be resumed, with dry runs at every
level. Planning also works from macOS and Windows.

> [!WARNING]
> **Status: pre-alpha.** gluper doesn't inspect or upgrade anything yet. The
> repository currently holds the project skeleton and the release pipeline.
> Don't run it against production. Follow the
> [roadmap](#roadmap) for progress.

> [!NOTE]
> gluper is an **unofficial** community tool. It is not affiliated with,
> endorsed by, or supported by GitLab Inc. It automates GitLab's documented
> upgrade procedure and never tries to be smarter than the official docs.

## Support matrix (planned for v1)

### Where gluper runs

| OS / arch                 | What it can do                                                                 |
| ------------------------- | ------------------------------------------------------------------------------ |
| Linux amd64, arm64        | Everything: inspect, plan, guards, backup, upgrade, verify                     |
| macOS amd64, arm64        | Read-only: `plan`, `versions`, `config`; facts come from an exported `gluper info --json` file |
| Windows amd64, arm64      | Same read-only set as macOS                                                    |

Commands that change the GitLab host (`info`, `check`, `upgrade`, `backup`,
`verify`, …) are compiled only into Linux builds. On other systems they exit
with code 5 and say so.

### GitLab installations

| Install type                                         | Upgrade execution          |
| ---------------------------------------------------- | -------------------------- |
| Linux package (Omnibus) with apt                     | v1                         |
| Linux package with dnf / yum                         | v1                         |
| Linux package with zypper                            | v1.x                       |
| CE and EE (detected automatically)                   | v1                         |
| Docker / Docker Compose                              | later; detected and refused until then |
| Helm chart / Operator                                | plan only, later           |
| Multi-node / zero-downtime, source installs          | not supported; refused     |

### GitLab versions

| Installed version                                  | `plan` | `upgrade` |
| -------------------------------------------------- | ------ | --------- |
| 16.0.0 or later                                    | yes    | yes       |
| Older, but listed in GitLab's `upgrade_path.yml`   | yes    | no: do the hops below 16.0 manually, then gluper takes over |
| Not in the dataset                                 | no     | no        |

The minimum starting version (16.0.0, set at launch) is a dataset value. It is
reviewed once a year after each GitLab major release and only raised with a
deprecation notice in an earlier gluper release, never in a patch release.

## Install

Releases are published on
[GitHub Releases](https://github.com/LaszloMocsy/gluper/releases) for Linux,
macOS and Windows (amd64 and arm64), as archives and as `.deb`/`.rpm`
packages.

```sh
# Example: Linux amd64
VERSION=0.1.0
curl -fsSLO "https://github.com/LaszloMocsy/gluper/releases/download/v${VERSION}/gluper_${VERSION}_linux_amd64.tar.gz"
tar -xzf "gluper_${VERSION}_linux_amd64.tar.gz" gluper
sudo install -m 0755 gluper /usr/local/bin/gluper
gluper version
```

Or with Go: `go install github.com/LaszloMocsy/gluper/cmd/gluper@latest`.

### Verify a download

Every release artifact has a build provenance attestation, and `checksums.txt`
is signed with keyless [cosign](https://github.com/sigstore/cosign).

```sh
# Either: GitHub CLI
gh attestation verify gluper_0.1.0_linux_amd64.tar.gz --repo LaszloMocsy/gluper

# Or: cosign + checksums
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/LaszloMocsy/gluper/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
sha256sum --ignore-missing -c checksums.txt
```

## Exit codes

Exit codes are part of gluper's contract and don't change meaning:

| Code | Meaning                                                      |
| ---- | ------------------------------------------------------------ |
| 0    | Success                                                      |
| 1    | Error                                                        |
| 2    | A guard blocked the operation                                |
| 3    | Update available (`versions --check`)                        |
| 4    | Post-upgrade verification failed                             |
| 5    | Unsupported platform, install type or starting version       |

## Roadmap

| Milestone | Scope                                                           |
| --------- | --------------------------------------------------------------- |
| M0        | Skeleton, cross-platform build and signed releases              |
| M1        | Inspect (read-only): `info`, `config`                           |
| M2        | Plan and guards on any OS: `versions`, `plan`, `check`          |
| M3        | Execute a single hop on Linux package installs                  |
| M4        | Multi-hop, resume, zypper, end-to-end test matrix               |
| M5        | Release notes and TUI                                           |
| M6        | Docker/Compose, live API planning, air-gapped, SSH, and more    |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). To report a security problem, see
[SECURITY.md](SECURITY.md) and please don't open a public issue.

## License

[MIT](LICENSE)
