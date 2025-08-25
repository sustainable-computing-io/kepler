# Enhancement Proposals

This directory contains Enhancement Proposals (EPs) for major features and changes to Kepler.

## Active Proposals

| ID                                           | Title                            | Status   | Author                  | Created    |
|----------------------------------------------|----------------------------------|----------|-------------------------|------------|
| [EP-000](EP_TEMPLATE.md)                     | Enhancement Proposal Template    | Accepted | Sunil Thaha             | 2025-01-18 |
| [EP-001](EP_001-redfish-support.md)          | Redfish Power Monitoring Support | Draft    | Sunil Thaha             | 2025-08-14 |
| [EP-002](EP-002-MSR-Fallback-Power-Meter.md) | MSR Fallback for CPU Power Meter | Draft    | Kepler Development Team | 2025-08-12 |

## Proposal Status

- **Draft**: Initial proposal under discussion
- **Accepted**: Proposal approved for implementation
- **Implemented**: Feature has been implemented and merged
- **Rejected**: Proposal was not accepted
- **Superseded**: Proposal replaced by a newer one

## Creating a New Proposal

To create a new enhancement proposal:

1. Copy the [EP_TEMPLATE.md](EP_TEMPLATE.md) template
2. Name your file `EP_XXX-short-title.md` where XXX is the next available number
3. Fill out all sections of the template
4. Update this index with your proposal
5. Submit a pull request for review

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
