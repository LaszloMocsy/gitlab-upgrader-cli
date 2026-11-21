# Reference-script parity checklist

gluper started from a single-purpose upgrade script for one install type (a
Linux package on apt, CE). That script is a **parity baseline**: gluper must
handle that scenario at least as well, on every supported setup. It isn't a
design constraint, so gluper generalizes each behaviour below instead of
copying it.

Each item names the milestone that delivers it. Milestone M3 is done only when
every item is ticked.

## Upgrade path

- [ ] Read required stops from GitLab's `config/upgrade_path.yml`. **M2** (dataset)
- [ ] Use a local copy of the path definition instead of downloading it, for offline hosts. **M2** (`dataset.source: file:`)
- [ ] Sanity-check the parsed stops, and fail if the definition looks truncated or its format changed. **M2**
- [ ] Resolve each stop to the latest patch of that minor that the package source offers. **M2**
- [ ] Skip stops below the installed minor, and stops whose latest patch is already installed. **M2**
- [ ] Drop stops at or above the target minor; the target is always the last hop. **M2**
- [ ] Validate the target version format (`MAJOR.MINOR.PATCH`). **M2**
- [ ] Refuse when already on the target, and refuse downgrades. **M2** (`no-downgrade`)
- [ ] Warn when a newer patch exists in the target minor. **M2**
- [ ] Check that the target and the next hop exist in the package source before changing anything. **M2** (`pkg-available`)
- [ ] Refuse if the computed next hop isn't newer than the installed version. **M2**
- [ ] Print the full path, marking the hop this run will do. **M2** (`plan`)

## Version-specific prerequisites

- [ ] Block a hop into a major release when the OS release no longer gets packages for it, and warn when a later hop will need an OS upgrade. **M2** (`os-support`, dataset rule)
- [ ] Block a hop when `gitlab.rb` still configures a component removed in that release (for example bundled Mattermost before 19.x), and warn ahead of it. **M2** (`config-rule`, dataset rule)

## Pre-flight

- [ ] Refuse mutating runs unless running as root. **M3** (`root`)
- [ ] Check that the package-manager and GitLab tools are present. **M1** (detection)
- [ ] Check that GitLab is installed, and read its installed version from the package manager. **M1**
- [ ] Identify the OS from `/etc/os-release`. Unlike the reference, gluper supports every distro in the dataset instead of one. **M1**
- [ ] Refresh package metadata before resolving versions. **M3** (mutating step, shown in `--dry-run`)
- [ ] Run `gitlab-ctl check-config` before starting. **M2** (`check`, `config-rule`)
- [ ] Show service status before starting. **M2** (`services-healthy`)
- [ ] Check disk space; the reference only printed `df`, gluper enforces a minimum. **M3** (`disk-space`)

## Run record

- [ ] Create a private (mode 0700) per-run directory named after the hop and the time. **M3** (journal)
- [ ] Save `gitlab.rb` and `gitlab-secrets.json`. **M3** (config backup)
- [ ] Record the installed package version, `gitlab-ctl show-config` output, the path definition used, system information and the computed plan. **M3**
- [ ] Record the result: previous and new version, remaining hops, backup location. **M3**
- [ ] Clean up temporary files on exit, including on failure. **M3**

## Backup

- [ ] Create an application backup with `gitlab-backup create`, with configurable `SKIP=` components. **M3**
- [ ] Identify the new backup. The reference used the newest file; gluper parses the backup ID. **M3**
- [ ] Fail before installing if no new backup appeared or the archive is empty, and say clearly that nothing was upgraded. **M3**
- [ ] Leave the archive where GitLab wrote it (no second full copy by default). **M3** (`backup.strategy: default`)
- [ ] Record the archive's SHA-256 checksum. gluper also checks the archive's contents. **M3**

## Checkpoints

- [ ] Checkpoint before the backup: current version, target, full path, this run's hop, suggested manual checks. **M3**
- [ ] Checkpoint before the install: backup location and checksum, and how to verify them. **M3**
- [ ] Do one hop per run by default. **M3**

## Install and verify

- [ ] Install the exact package version for the hop. **M3**
- [ ] Verify the installed version matches the expected one. **M3**
- [ ] Run `gitlab-ctl status`, `gitlab-rake gitlab:check SANITIZE=true` and `gitlab-ctl check-config` afterwards. gluper waits for readiness first. **M3** (`verify`)
- [ ] Print a summary: the remaining hops, the command for the next hop, and a manual test checklist. **M3** (`verify.checklist`)
- [ ] Remind the user to wait for background migrations before the next hop. gluper checks them instead. **M4** (`bbm-finished`)

## Gaps in the reference that gluper closes

These go beyond parity. They're tracked as guards and features, not here:
real locking (`lock`), checked background migrations (`bbm-finished`),
PostgreSQL compatibility (`postgres-compat`), a backup path read from
GitLab's effective configuration (`backup-target`), secrets copied off the
data filesystem (`secrets-safe`), package holds (`package-held`), and a
journal written after every step so runs can resume.
