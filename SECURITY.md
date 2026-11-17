# Security Policy

## Reporting a Vulnerability

Please do not report security issues in public issues, pull requests or Slack.

**[Open a private report on the Kepler repository](https://github.com/sustainable-computing-io/kepler/security/advisories/new)**
using GitHub's [Private Vulnerability Reporting]. Use this link for any
repository in the sustainable-computing-io organization, and say which one is
affected.

If you can't use GitHub, email the maintainers at
<cncf-kepler-maintainers@lists.cncf.io>.

Please include:

- the affected repository and version (or commit)
- steps to reproduce or a proof of concept
- the impact you expect

## Scope

This policy covers the actively maintained projects:

- [Kepler](https://github.com/sustainable-computing-io/kepler), including its
  Helm chart and Kubernetes manifests
- [Kepler Operator](https://github.com/sustainable-computing-io/kepler-operator)
- [Kepler documentation](https://sustainable-computing.io/)
  ([kepler-doc](https://github.com/sustainable-computing-io/kepler-doc))

Reports for other repositories in the organization are handled on a best-effort
basis.

Kepler needs host access to measure power and attribute it to workloads. By
default it uses the host PID namespace, reads `/proc` and `/sys`, and has the
`SYS_PTRACE` capability. A report that only shows Kepler can read this data is
expected behavior. A report that shows Kepler exposes this data to someone who
shouldn't have it, or can be used to escalate privileges, is in scope.

## Security Response Team

The maintainers with the Repository Oversight role triage reports, as set out
in [GOVERNANCE.md](GOVERNANCE.md#4-repository-oversight). The Project Lead is
the escalation contact.

## What to Expect

The team is made up of volunteers, so these are targets, not guarantees.

1. **Acknowledgement:** within 3 business days. If you hear nothing after a
   week, email the maintainers list above.
2. **Triage:** within 14 days we confirm the issue and rate its severity with
   [CVSS](https://www.first.org/cvss/). We may ask for more details or invite
   you to a temporary private fork to work on the fix. Low-severity reports
   may be moved to the public issue tracker, with your agreement.
3. **Updates:** at least every 14 days until the issue is fixed.
4. **Fix:** we aim to release a fix for critical and high severity issues
   within 30 days of confirming them. Lower severity fixes ship in a later
   regular release.
5. **Disclosure:** when the fix is released we publish a
   [GitHub Security Advisory](https://github.com/sustainable-computing-io/kepler/security/advisories),
   request a CVE through GitHub, and credit you unless you'd rather stay
   anonymous. Please keep the issue private until then. We aim to disclose
   within 90 days of your report. If we need longer, we'll tell you why and
   agree on a date with you.

We don't run a pre-disclosure list. Vendors who ship Kepler can ask the team
to be told before an advisory is published.

## Supported Versions

Kepler and the Kepler Operator are pre-1.0. Security fixes land on `main` and
ship in the next release of the affected project. Older releases don't get
backports, so please upgrade to the latest release.

Report anything that affects the latest release or `main`.

## Security Practices

What the Kepler repository does today to catch problems early:

- [OpenSSF Best Practices](https://www.bestpractices.dev/projects/7391) passing badge
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/sustainable-computing-io/kepler)
  runs on every pull request and push to `main`, with results in GitHub code
  scanning
- `govulncheck` scans Go dependencies in the required pre-commit check on
  every pull request, and again before every release
- Dependabot opens weekly updates for Go modules and GitHub Actions
- [Snyk](https://snyk.io/) static code scanning to find vulnerabilities
- A CI check flags pull requests that add new `golang.org/x/crypto`
  dependencies
- GitHub Actions workflows default to a read-only token, and jobs that need
  write access declare it
- Go fuzz targets exist for cgroup and command-line parsing in
  `internal/resource` and label parsing in the NVIDIA DCGM exporter. Their seed
  inputs run with the unit tests
- [FOSSA](https://app.fossa.com/projects/git%2Bgithub.com%2Fsustainable-computing-io%2Fkepler)
  license and security scanning
- `main` is protected: every pull request needs an approving review, a new
  push dismisses earlier approvals, and force pushes are blocked
- Pull requests are checked for [DCO](DCO) sign-off

[Private Vulnerability Reporting]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability
