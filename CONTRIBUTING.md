# Contributing to Kepler 🌱

Thanks for your interest in contributing to Kepler! This guide will help you get started with contributing to our project.
This guide focuses primarily on technical contributions to the core Kepler project.

For general contribution guidelines, community engagement, and contributor growth, visit the
[Kepler Community Repository](https://github.com/sustainable-computing-io/community).

## What Should I Know Before I Get Started? 🤔

Kepler is a Go-based project focused on sustainable computing. Before contributing, it's helpful to:

- Have basic knowledge of Go programming
- Understand container technologies and Kubernetes
- Familiarize yourself with the project structure by exploring the repository
- Read our [GOVERNANCE.md](GOVERNANCE.md) and [MAINTAINERS.md](MAINTAINERS.md) documents

## Gen AI policy

Our project adheres to the Linux Foundation's Generative AI Policy, which can be viewed at [shared Gen AI Policy](https://github.com/sustainable-computing-io/community/blob/main/CONTRIBUTING.md#gen-ai-policy).

## How Do I Start Contributing? 🚀

1. Find or open an issue, and wait for a maintainer to triage it (see [Pull Request Policy](#pull-request-policy))
2. Fork the repository on GitHub
3. Clone your fork locally
4. Set up your development environment (see below)
5. Create a new branch for your work
6. Make your changes
7. Submit a pull request that links the triaged issue

Don't forget to sign your commits according to our [DCO](DCO) requirements!

### Keep Your Company Affiliation Up to Date 🏢

See the [shared company affiliation guidelines](https://github.com/sustainable-computing-io/community/blob/main/CONTRIBUTING.md#keep-your-company-affiliation-up-to-date-).

## How Can I Contribute? 💡

### Continuous Integration 🔄

Our project uses GitHub Actions for CI. Each PR will trigger automated builds and tests. Check the `.github/workflows` directory to understand our CI pipeline. Ensure your contributions pass all CI checks before requesting a review.

In workflows and composite actions, pin actions from other repositories to a full commit SHA and add the release tag in a comment on the same line (`owner/action@<full-commit-sha> # vX.Y.Z`). Dependabot proposes updates to these pins.

### Pull Request Policy

Agree on the direction of a change in an issue before you write the code. A large
diff for a direction the project has not agreed on costs reviewers time and costs
you work.

Every pull request links an issue that a maintainer has triaged. New issues get
the `needs-triage` label automatically. An issue counts as triaged when a
maintainer gives it one of these labels:

- `triage/accepted`
- `help wanted`
- `good first issue`

A `good first issue` also needs to be assigned to you before you open a pull
request. Comment on the issue to ask for it. Opening an issue and a pull request
together does not count as triage.

Link the issue in the pull request description, for example `Closes #123` or
`Part of #123`. Features and design changes also need an accepted
[enhancement proposal](docs/developer/proposal/EP-TEMPLATE.md) first.

These pull requests skip the triage requirement:

- Pull requests from maintainers
- Pull requests from bots, such as Dependabot
- Small fixes, such as typos or broken links

Fill in every section of the pull request template. Write the description in
your own words, and give your AI Influence Level (AIL) in the AI Disclosure
section along with what you checked yourself. You are responsible for every line
you submit, whatever tools you used.

Maintainers may close a pull request that does not link a triaged issue. A close
is not a judgment of the work: once the issue is triaged, open a new pull request
or ask a maintainer to reopen it.

### Code Review 👀

All submissions require review. We use GitHub pull requests for this purpose:

1. Submit a pull request from your fork to our main repository
2. Ensure all CI checks pass
3. Address feedback from maintainers
4. Once approved, a maintainer will merge your changes

### Commit Hygiene

Kepler uses merge commits, so the individual commits in a PR remain in the
project history. A coherent commit sequence helps future contributors understand
changes and makes debugging and reverting easier.

- Aim for each commit to represent a coherent change with a meaningful message.
- Keep separate commits for distinct, useful logical steps within a PR.
- Generally fold intermediate fixes, such as link corrections and review
  follow-ups, into the commits they belong to before merging.
- Order commits so the progression of the change is easy to follow.

These are commit hygiene principles, not strict rules or a requirement to squash
every PR into a single commit.

### Reporting Bugs 🐛

Before reporting a bug, review the
[shared bug reporting guidelines](https://github.com/sustainable-computing-io/community/blob/main/CONTRIBUTING.md#reporting-bugs-).

To report a bug specific to Kepler, select the **Bug Report**
template when [creating a new issue](https://github.com/sustainable-computing-io/kepler/issues/new/choose).

Include your Go version where applicable.

### Suggesting Enhancements ✨

#### How Do I Submit A (Good) Enhancement Suggestion?

Review the
[shared enhancement guidelines](https://github.com/sustainable-computing-io/community/blob/main/CONTRIBUTING.md#suggesting-enhancements-).

To propose a new feature or enhancement for Kepler, select the
**Feature Request** template when
[creating a new issue](https://github.com/sustainable-computing-io/kepler/issues/new/choose).

#### How Do I Submit A (Good) Improvement Item?

For smaller improvements to existing functionality:

- Focus on a single, specific improvement
- Explain how your improvement makes Kepler better
- Provide context around why this improvement matters
- Link to any related issues or discussions

#### Do I Need An Enhancement Proposal (EP) Before My PR? 📝

Kepler follows the [Kubernetes Enhancement Proposal (KEP) process](https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/0000-kep-process):
a proposal is a design document reviewed and agreed on **before** implementation,
so that large changes are not first discussed in a big PR.
Not every change needs one. A KEP is reserved for changes that are significant,
user-visible, or hard to reverse,
and routine fixes go straight to a PR.

**Open an issue first for every enhancement.** Maintainers will tell you
in the issue whether an EP is required.
If you are unsure, ask there before writing code or a proposal.

**An EP is expected when the change:**

- Adds or removes a major feature or a new component, such as a new power meter,
hardware source, or workload type.
- Changes the power attribution model or how energy and power are calculated, split,
or allocated
- Introduces breaking or hard-to-reverse changes to user-facing behavior:
metric names, labels, or units, configuration schema or CLI flags,
or deployment layout
- Is cross-cutting, spanning several packages or components, or needs
new external dependencies or elevated privileges
- Needs a phased rollout, a migration path, or a deprecation period
- Has several credible designs and needs maintainers to agree before work starts

**An EP is not needed when the change:**

- Fixes a bug, including race conditions and linter errors,
without changing documented behavior
- Refactors, cleans up, or optimizes internals without changing behavior
- Adds or improves tests, documentation, examples, or CI
- Updates dependencies or makes other routine maintenance changes
- Is a small, self-contained, backward-compatible improvement,
such as a minor option or additional metric that follows an existing pattern,
and a short issue discussion is enough

When in doubt, start with an issue. A maintainer can ask for an EP
if the discussion shows the change is larger than it first looked.

**Working toward the same goal as someone else?** If more than one contributor
or organization is working on the same goal, we encourage you to communicate
in the issue or proposal PR. On request, a maintainer can help organize a
separate meeting with a small group to divide the tasks and align on the design.
We encourage recording the meeting notes in a shared document on
[CNCF Notes](https://notes.cncf.io/) and linking them in the issue or proposal
PR, so that decisions stay visible to the rest of the community.

**How to submit an EP:** copy [EP-TEMPLATE.md](docs/developer/proposal/EP-TEMPLATE.md),
submit it as its own PR (see [Enhancement Proposals](docs/developer/proposal/index.md)
for naming, roles, and status), and get it accepted before
opening implementation PRs. Reference the EP in those PRs.

## Development 💻

### Set up your dev environment

1. Install the Go version specified in the repository's [`go.mod`](go.mod) file
2. Install pre-commit hooks:

```bash
pre-commit install
```

See [docs/developer/pre-commit.md](docs/developer/pre-commit.md) for more information.

### Testing

Run tests with:

```bash
make test
```

Generate coverage reports with:

```bash
make coverage
```

Our CI automatically runs tests and uploads coverage to Codecov.

## Your First Code Contribution 🎉

Looking for a place to start?

1. Look for issues labeled `good first issue` or `help wanted`
2. Introduce yourself in a GitHub issue before starting work
3. Read the code in the area you want to work on to understand patterns
4. Ask questions in issues or on Slack if you need help

Remember that even small contributions like fixing typos or improving documentation are valuable!

Thanks for contributing to Kepler! 💚
