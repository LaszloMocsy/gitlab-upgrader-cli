// Package config loads gluper's configuration.
//
// Values are layered, highest precedence first: command-line flags, GLUPER_*
// environment variables, the config file, and built-in defaults. Every
// effective value remembers where it came from (see Source), which is what
// `gluper config show` prints.
//
// The schema is the Config struct. Its yaml tags define the key names, and the
// key registry (keys.go) is derived from it by reflection, so adding a field
// adds the key, its GLUPER_* variable and its validation in one place.
//
// Every default is "auto" or conservative. Nothing here may assume a
// particular GitLab setup.
package config

import "time"

// Config is gluper's complete configuration.
type Config struct {
	Target            Target            `yaml:"target"`
	HopsPerRun        HopsPerRun        `yaml:"hops_per_run"`
	Checkpoints       []Checkpoint      `yaml:"checkpoints"`
	Install           Install           `yaml:"install"`
	Backup            Backup            `yaml:"backup"`
	ConfigBackup      ConfigBackup      `yaml:"config_backup"`
	Dataset           Dataset           `yaml:"dataset"`
	Upgrade           Upgrade           `yaml:"upgrade"`
	Verify            Verify            `yaml:"verify"`
	Hooks             Hooks             `yaml:"hooks"`
	Notify            Notify            `yaml:"notify"`
	MaintenanceWindow MaintenanceWindow `yaml:"maintenance_window"`
}

// Install describes how GitLab is installed. "auto" means detect it.
type Install struct {
	Type           InstallType    `yaml:"type"`
	PackageManager PackageManager `yaml:"package_manager"`
	Edition        Edition        `yaml:"edition"`
	Source         PackageSource  `yaml:"source"`
}

// Backup controls the application backup taken before each hop.
type Backup struct {
	Enabled             bool           `yaml:"enabled"`
	Path                HostPath       `yaml:"path"`
	Skip                []string       `yaml:"skip"`
	Strategy            BackupStrategy `yaml:"strategy"`
	VerifyArchive       VerifyArchive  `yaml:"verify_archive"`
	MaxAge              Duration       `yaml:"max_age"`
	MinFreeSpace        Size           `yaml:"min_free_space"`
	SkipPackageDBBackup bool           `yaml:"skip_package_db_backup"`
	Target              BackupTarget   `yaml:"target"`
}

// BackupTarget holds optional assertions about where backups land.
type BackupTarget struct {
	RequireMount bool     `yaml:"require_mount"`
	ExpectedFS   []string `yaml:"expected_fs"`
}

// ConfigBackup controls the copy of gitlab.rb, gitlab-secrets.json, the
// effective config and the dataset used.
type ConfigBackup struct {
	Dir    HostPath `yaml:"dir"`
	Strict bool     `yaml:"strict"`
}

// Dataset selects where upgrade-path data comes from.
type Dataset struct {
	Source DatasetSource `yaml:"source"`
}

// Upgrade holds timeouts and behaviour of the upgrade itself.
type Upgrade struct {
	BBMWaitTimeout   Duration `yaml:"bbm_wait_timeout"`
	ReadinessTimeout Duration `yaml:"readiness_timeout"`
	HoldPackage      bool     `yaml:"hold_package"`
}

// Verify controls the checks after each hop.
type Verify struct {
	Smoke     Smoke    `yaml:"smoke"`
	Checklist []string `yaml:"checklist"`
}

// Smoke lists the opt-in smoke checks.
type Smoke struct {
	Readiness bool `yaml:"readiness"`
}

// Hooks are commands run at fixed points of an upgrade (from milestone M4).
type Hooks struct {
	PreUpgrade  []string `yaml:"pre_upgrade"`
	PostHop     []string `yaml:"post_hop"`
	PostUpgrade []string `yaml:"post_upgrade"`
	OnFailure   []string `yaml:"on_failure"`
}

// Notify configures notifications.
type Notify struct {
	Webhook WebhookURL `yaml:"webhook"`
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{
		Target:      Target("latest"),
		HopsPerRun:  HopsPerRun{N: 1},
		Checkpoints: []Checkpoint{CheckpointBeforeBackup, CheckpointBeforeInstall},
		Install: Install{
			Type:           "auto",
			PackageManager: "auto",
			Edition:        "auto",
			Source:         "repo",
		},
		Backup: Backup{
			Enabled:             true,
			Path:                "auto",
			Skip:                []string{},
			Strategy:            "default",
			VerifyArchive:       "auto",
			MaxAge:              0,
			MinFreeSpace:        Size{Auto: true},
			SkipPackageDBBackup: true,
			Target: BackupTarget{
				RequireMount: false,
				ExpectedFS:   []string{},
			},
		},
		ConfigBackup: ConfigBackup{Dir: "auto", Strict: false},
		Dataset:      Dataset{Source: DatasetSource("auto")},
		Upgrade: Upgrade{
			BBMWaitTimeout:   Duration(6 * time.Hour),
			ReadinessTimeout: Duration(10 * time.Minute),
			HoldPackage:      true,
		},
		Verify: Verify{
			Smoke:     Smoke{Readiness: true},
			Checklist: []string{},
		},
		Hooks: Hooks{
			PreUpgrade:  []string{},
			PostHop:     []string{},
			PostUpgrade: []string{},
			OnFailure:   []string{},
		},
	}
}
