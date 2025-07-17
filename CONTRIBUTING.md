# Contributing to Kepler 🌱

Thanks for your interest in contributing to Kepler! This guide will help you get started with contributing to our project.

## What Should I Know Before I Get Started? 🤔

Kepler is a Go-based project focused on sustainable computing. Before contributing, it's helpful to:

- Have basic knowledge of Go programming
- Understand container technologies and Kubernetes
- Familiarize yourself with the project structure by exploring the repository
- Read our [GOVERNANCE.md](GOVERNANCE.md) and [MAINTAINERS.md](MAINTAINERS.md) documents

## Gen AI policy

Our project adheres to the Linux Foundation's Generative AI Policy, which can be viewed at [https://www.linuxfoundation.org/legal/generative-ai](https://www.linuxfoundation.org/legal/generative-ai).

## How Do I Start Contributing? 🚀

1. Fork the repository on GitHub
2. Clone your fork locally
3. Set up your development environment (see below)
4. Create a new branch for your work
5. Make your changes
6. Submit a pull request

Don't forget to sign your commits according to our [DCO](DCO) requirements!

## How Can I Contribute? 💡

### Continuous Integration 🔄

Our project uses GitHub Actions for CI. Each PR will trigger automated builds and tests. Check the `.github/workflows` directory to understand our CI pipeline. Ensure your contributions pass all CI checks before requesting a review.

### Code Review 👀

All submissions require review. We use GitHub pull requests for this purpose:

1. Submit a pull request from your fork to our main repository
2. Ensure all CI checks pass
3. Address feedback from maintainers
4. Once approved, a maintainer will merge your changes

### Reporting Bugs 🐛

Found a bug? Please report it by creating an issue with the bug template. Include:

- A clear and descriptive title
- Steps to reproduce the issue
- Expected vs actual behavior
- Screenshots or logs if applicable
- Your environment details (OS, Go version, etc.)

### Suggesting Enhancements ✨

#### How Do I Submit A (Good) Enhancement Suggestion?

Enhancement suggestions are tracked as GitHub issues. Create an issue with the enhancement template and provide:

- A clear and descriptive title
- A detailed description of the proposed enhancement
- Any potential implementation ideas you have
- Why this enhancement would be useful to most Kepler users

#### How Do I Submit A (Good) Improvement Item?

For smaller improvements to existing functionality:

- Focus on a single, specific improvement
- Explain how your improvement makes Kepler better
- Provide context around why this improvement matters
- Link to any related issues or discussions

## Development 💻

### Set up your dev environment

1. Install Go (1.19 or later recommended)
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
