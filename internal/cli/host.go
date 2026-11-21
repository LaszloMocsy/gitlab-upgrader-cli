package cli

// hostCommandNames lists the commands that act on the GitLab host itself
// (inspect, back up, upgrade, verify). Their implementations are compiled into
// Linux builds only (host_linux.go). On macOS and Windows each name is
// registered as a stub that explains why it can't run there and exits with
// exitcode.Unsupported (host_other.go).
//
// Filled in from M1 on: info, check, upgrade, status, resume, backup, verify,
// restore. A test on Linux keeps this list and the implementations in sync.
var hostCommandNames = []string{}
