# Enhancement Proposals

This directory contains Enhancement Proposals (EPs) for major features and changes to Kepler.

## Active Proposals

| ID                                           | Title                            | Status   | Author                  | Created    |
|----------------------------------------------|----------------------------------|----------|-------------------------|------------|
| [EP-000](EP_TEMPLATE.md)                     | Enhancement Proposal Template    | Accepted | Sunil Thaha             | 2025-01-18 |
| [EP-001](EP_001-redfish-support.md)          | Redfish Power Monitoring Support | Draft    | Sunil Thaha             | 2025-08-14 |
| [EP-002](EP-002-MSR-Fallback-Power-Meter.md) | MSR Fallback for CPU Power Meter | Draft    | Kepler Development Team | 2025-08-12 |
| [EP-003](EP-003-GPU-Power-Monitoring.md)     | GPU Power Monitoring             | Draft    | Vimal Kumar             | 2025-12-10 |

## Proposal Status

- **Draft**: Initial proposal under discussion
- **Accepted**: Proposal approved for implementation
- **Implemented**: Feature has been implemented and merged
- **Rejected**: Proposal was not accepted
- **Superseded**: Proposal replaced by a newer one

## Creating a New EP

To create a new EP, start with a draft:

1. Copy the [EP_TEMPLATE.md](EP_TEMPLATE.md) template
2. Name your file `draft-YYYYMMDD-short-title.md` (e.g., `draft-20260408-vm-power-models.md`), following the [KEP naming convention](https://github.com/kubernetes/enhancements/blob/master/keps/sig-architecture/0000-kep-process/README.md#git-and-github-implementation)
3. Optionally, drafts can be checked into Git.
4. Fill out all sections of the template
5. Submit a pull request for review

A proposal starts as a **draft** and does not receive a number until the PR is approved.
If a new EP supersedes a draft, reference the closed PR in the new proposal for context.

Once it is approved, assign the next available `EP-XXX` number, rename the file prefix to `EP-XXX-<title>`, and update the index table above.

## Proposal Template

Use the [EP_TEMPLATE.md](EP_TEMPLATE.md) file as your starting point. The template includes comprehensive sections for:

- **Summary and Problem Statement**: Clear description of the enhancement and motivation
- **Goals and Non-Goals**: Scope definition and boundaries
- **Requirements**: Functional and non-functional requirements
- **Architecture**: Technical design and implementation details
- **Configuration**: New configuration options and deployment examples
- **Testing Strategy**: Comprehensive testing approach
- **Migration**: Backward compatibility and upgrade path
- **Implementation Plan**: Phased development approach with risk mitigation
