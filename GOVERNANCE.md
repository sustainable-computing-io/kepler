# Governance

## Leadership Election

For most roles, candidates can self-nominate or be nominated
and are approved through consensus by the existing maintainers.

### Nomination Protocol

#### Submit Nomination

The candidate or an existing maintainer creates
a [maintainer nomination issue](.github/ISSUE_TEMPLATE/maintainer_nominate.yaml)
and submits a pull request (PR) to modify the `MAINTAINERS.md`
or `ADVISORS.md` file to add the nominee.

#### Voting

The PR requires approval from **at least three maintainers**,
including the nominator if they are not self-nominating.
Maintainers indicate their approval by approving on the PR.

#### Approval and Merge

Once the required approvals are met, the PR can be merged.

If the nominee is not yet added as a maintainer to the repository,
they should be added at this point,
officially confirming their maintainer status.

## Maintainers

Maintainers are responsible for the overall health and progress of the project.

Their roles and responsibilities are organized into functional areas:

### 1. Project Lead

- Provide overall vision and strategic direction for the project
- Facilitate collaboration and communication among maintainers,
  contributors, and stakeholders
- Make final decisions on major project proposals and conflicts
- Represent the project publicly in meetings, conferences, and external communications
- Ensure the project adheres to its goals, timelines, and governance policies
- Mentor maintainers and foster a healthy, inclusive community culture
- Oversee release planning and high-level project milestones

### 2. Technical Committee

- Oversee the overall direction of the Kepler project.
- Provide guidance for the project maintainers and the onboarding process for
  new maintainers.
- Actively engage in the technical committee meetings.
- Participate in design and technical discussions
- Review and approve pull requests,
  especially those involving significant technical changes or core areas

#### Becoming a Technical Committee Member

Technical Committee membership is reserved for contributors with a strong,
demonstrated commitment to the project.
Members are elected or invited based on their ongoing,
significant contributions including:

- Core Kepler development and maintenance
- Test suite maintenance
- Deployment and integration (including
  the [Kepler Operator](https://github.com/sustainable-computing-io/kepler-operator),
  [Helm Chart](https://github.com/sustainable-computing-io/kepler-helm-chart))
- [Model server](https://github.com/sustainable-computing-io/kepler-model-server)
  and model development
- [Continuous integration](https://github.com/sustainable-computing-io/kepler-action)
- [Core documentation](https://github.com/sustainable-computing-io/kepler-doc)

### 3. Release Management

- Oversee planning and coordination of releases
- Approve release blockers and guide final cut
- Ensure changelogs, versioning, and release notes are accurate

### 4. Repository Oversight

- Monitor GitHub issues and PR queues
- Label, prioritize, and assign appropriately
- Close stale or unmaintained issues when needed
- Enforce code of conduct and contribution guidelines
- Maintain project security policies and dependencies
- Review and approve pull requests,
  especially those involving project or repository management

### 5. Community Engagement

- Promote the project through community calls, social channels, and events
- Host or rotate as facilitator for community calls
- Respond to contributor questions in forums, Slack, GitHub Discussions
- Connect contributors with the appropriate maintainers or domain experts
- Welcome new contributors and guide them toward first issues
- Review and approve pull requests, especially those involving community engagement

The current list of maintainers is published and updated in
[MAINTAINERS.md](./MAINTAINERS.md).

## Reviewer

A Reviewer has responsibility for specific code, documentation, test,
or other project areas.
They are collectively responsible, with other Reviewers,
for reviewing all changes to those areas and indicating whether
those changes are ready to merge.
They have a track record of contribution and review in the project.

Reviewers are responsible for a "specific area."
This can be a specific code directory, driver, chapter of the docs,
test job, event, or other clearly-defined project component
that is smaller than an entire repository or subproject.
Most often it is one or a set of directories in one or more Git repositories.

The "specific area" below refers to this area of responsibility:

### Responsibilities

- Following the reviewing guide
- Reviewing most Pull Requests against their specific areas of responsibility
- Helping other contributors become reviewers

#### Becoming a Reviewer

The contributor is nominated by opening a PR against the appropriate repository,
which adds their GitHub username to
one or more [OWNERS file](https://www.kubernetes.dev/docs/guide/owners/).

At least one member of the maintainer or the team
that owns that repository or main directory,
who are already Approvers, approve the PR.

## Advisory Committee

Advisory committee provides guidance for the overall direction of the Kepler
project, including but not limited to:

- Project roadmap
- Project governance
- Project releases
- Project marketing

Members of the advisory committee are expected to be active in the Kepler
community and attend the advisory committee meetings. Members are expected to
serve for a term of one year, with the option to renew for additional terms.

Members are invited and approved by the Kepler maintainers.

The current list of advisory committee is published and updated in
[ADVISORS.md](./ADVISORS.md).
