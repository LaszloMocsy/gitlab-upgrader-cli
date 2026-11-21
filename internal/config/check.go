package config

import (
	"fmt"
	"slices"
	"strings"
)

// knownBackupSkips lists the SKIP= values of gitlab-backup create that gluper
// knows. Others produce a warning, not an error, because GitLab adds new ones.
var knownBackupSkips = []string{
	"db", "uploads", "builds", "artifacts", "lfs", "terraform_state",
	"registry", "pages", "repositories", "packages", "ci_secure_files",
	"remote", "tar",
}

// check runs the validations that span keys or depend on meaning rather than
// syntax. Syntax errors were already reported while loading.
func check(l *Loaded) []Problem {
	c := &l.Config
	var ps []Problem
	add := func(sev Severity, key, format string, args ...any) {
		ps = append(ps, Problem{Severity: sev, Source: l.Sources[key], Key: key, Message: fmt.Sprintf(format, args...)})
	}

	for _, k := range []struct {
		key string
		d   Duration
	}{
		{"upgrade.bbm_wait_timeout", c.Upgrade.BBMWaitTimeout},
		{"upgrade.readiness_timeout", c.Upgrade.ReadinessTimeout},
	} {
		if k.d == 0 {
			add(SeverityError, k.key, "must be longer than 0")
		}
	}

	if !c.Backup.Enabled {
		add(SeverityWarning, "backup.enabled",
			"backups are off: every hop is blocked unless a recent backup exists or you pass --skip-backup")
	}
	if c.Backup.VerifyArchive == "off" {
		add(SeverityWarning, "backup.verify_archive", "new backups are not verified before the install")
	}
	var unknown []string
	for _, s := range c.Backup.Skip {
		switch {
		case s == "tar":
			add(SeverityError, "backup.skip", `"tar" is not allowed: gluper needs the backup archive to verify it`)
		case s == "db":
			add(SeverityWarning, "backup.skip", `"db" leaves the database out of the backup, so it cannot restore GitLab`)
		case !slices.Contains(knownBackupSkips, s):
			unknown = append(unknown, s)
		}
	}
	if len(unknown) > 0 {
		add(SeverityWarning, "backup.skip", "unknown component(s) %s, passed to gitlab-backup unchanged (known: %s)",
			quoteList(unknown), strings.Join(knownBackupSkips, ", "))
	}
	if slices.Contains(c.Backup.Skip, "remote") && c.Backup.VerifyArchive == "upload" {
		add(SeverityError, "backup.verify_archive", `"upload" cannot verify an upload that backup.skip ("remote") turns off`)
	}
	if len(c.Backup.Target.ExpectedFS) > 0 && !c.Backup.Target.RequireMount {
		add(SeverityWarning, "backup.target.expected_fs", "is only checked when backup.target.require_mount is true")
	}

	if c.ConfigBackup.Strict && c.ConfigBackup.Dir == "auto" {
		add(SeverityWarning, "config_backup.dir",
			`"auto" keeps the copy in the run directory, usually on GitLab's data filesystem, so strict mode will block; point it at another filesystem`)
	}
	if strings.HasPrefix(string(c.Notify.Webhook), "http://") {
		add(SeverityWarning, "notify.webhook", "uses plain http: notifications and any token in the URL travel unencrypted")
	}
	return ps
}
