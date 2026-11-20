# Security policy

gluper runs as root on production GitLab servers, so security reports are
taken seriously.

## Reporting a vulnerability

Please **don't open a public issue**. Report it privately through GitHub:

1. Go to the repository's **Security** tab.
2. Click **Report a vulnerability**.

You'll get an acknowledgement within 7 days. Please allow up to 90 days for a
fix before disclosing publicly; we'll agree on a date with you.

Helpful details: the gluper version (`gluper version`), the OS and GitLab
install type, what an attacker could do, and steps to reproduce.

## Scope

In scope: the gluper binary, its release artifacts and signatures, the install
script, and the published dataset.

Out of scope: vulnerabilities in GitLab itself. Report those to
[GitLab](https://about.gitlab.com/security/disclose/).

## Supported versions

Before 1.0, only the latest release gets security fixes.

## Verifying releases

Every release is signed with keyless cosign and has build provenance
attestations. See [Verify a download](README.md#verify-a-download).
