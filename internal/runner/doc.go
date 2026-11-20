// Package runner is the only place in gluper that starts external processes.
//
// Every command is tagged ReadOnly or Mutating and goes through a Runner
// (Local, DryRun, Simulate, and later SSH), which is what makes dry runs
// identical to real runs. Importing os/exec anywhere else fails the lint
// (depguard rule "no-exec-outside-runner" in .golangci.yml).
//
// The implementation arrives in milestone M3.
package runner
