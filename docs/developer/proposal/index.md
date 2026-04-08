# Enhancement Proposals

This directory contains Enhancement Proposals (EPs) for major features and changes to Kepler. This follows the [Kubernetes Enhancement Proposal process](https://github.com/kubernetes/enhancements/tree/master/keps/sig-architecture/0000-kep-process).

## Active Proposals

| ID                                           | Title                            | Status      | Authors                 | Created    |
|----------------------------------------------|----------------------------------|-------------|-------------------------|------------|
| [EP-000](EP-TEMPLATE.md)                     | Enhancement Proposal Template    | Accepted    | Sunil Thaha             | 2025-01-18 |
| [EP-001](EP-001-redfish-support.md)          | Redfish Power Monitoring Support | Implemented | Sunil Thaha             | 2025-08-14 |
| [EP-002](EP-002-MSR-Fallback-Power-Meter.md) | MSR Fallback for CPU Power Meter | Draft       | Kepler Development Team | 2025-08-12 |
| [EP-003](EP-003-GPU-Power-Monitoring.md)     | GPU Power Monitoring             | Draft       | Vimal Kumar             | 2025-12-10 |

## Proposal Status

- **Draft**: Initial proposal under discussion
- **Accepted**: Proposal approved for implementation
- **Implemented**: Feature has been implemented and merged
- **Rejected**: Proposal was not accepted
- **Superseded**: Proposal replaced by a newer one

## Creating a New EP

To create a new EP, copy the template: [EP-TEMPLATE.md](EP-TEMPLATE.md).

Follow the [KEP naming convention](https://github.com/kubernetes/enhancements/blob/master/keps/sig-architecture/0000-kep-process/README.md#git-and-github-implementation), which is roughly the following:

New EPs can be checked in with a file name in the form of `draft-YYYYMMDD-my-title.md`. The authors can assign an EP number as significant work is done on the proposal. An EP number can be assigned as part of the initial submission if the PR is likely to be uncontested and merged quickly. Once it is approved, assign the next available `EP-XXX` number, rename the file prefix to `EP-XXX-<title>`, and update the index table above.

If a new EP supersedes a draft, reference the closed PR in the new proposal for context.

## Roles

Each Kepler EP tracks the following contributors, based on the
[KEP metadata](https://github.com/kubernetes/enhancements/blob/master/keps/sig-architecture/0000-kep-process/README.md#kep-metadata):

- **Authors**: Write and own the proposal. Identified by GitHub ID.
- **Reviewers**: Assess technical soundness. Must be distinct from authors.
- **Approvers**: Decide when an EP advances. Drawn from maintainers.
- **Editor** (optional): Shepherds the proposal forward without judging merit.

## Proposal Template

Use the [EP-TEMPLATE.md](EP-TEMPLATE.md) file as your starting point. The template includes comprehensive sections for:

- **Summary and Problem Statement**: Clear description of the enhancement and motivation
- **Goals and Non-Goals**: Scope definition and boundaries
- **Requirements**: Functional and non-functional requirements
- **Architecture**: Technical design and implementation details
- **Configuration**: New configuration options and deployment examples
- **Testing Strategy**: Comprehensive testing approach
- **Migration**: Backward compatibility and upgrade path
- **Implementation Plan**: Phased development approach with risk mitigation
