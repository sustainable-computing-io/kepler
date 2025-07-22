# Pre-commit Hooks Guide

## Overview

This project uses [pre-commit](https://pre-commit.com/) to ensure code quality and consistency before commits are made. Pre-commit runs a series of checks and formatters on your code to catch issues early.

## Installation

1. Install pre-commit:

   ```bash
   # Using pip
   pip install pre-commit
   ```

2. Install the git hooks in your local repository:

   ```bash
   cd /path/to/kepler
   pre-commit install
   ```

## Available Hooks

Our pre-commit configuration includes the following hooks:

1. **trailing-whitespace**: Trims trailing whitespace from lines
2. **end-of-file-fixer**: Ensures files end with a newline
3. **check-added-large-files**: Prevents large files from being committed
4. **check-merge-conflict**: Prevents files with merge conflicts from being committed
5. **yamllint**: Validates YAML files
6. **markdownlint**: Validates Markdown files
7. **codespell**: Checks for common misspellings
8. **golangci-lint**: Runs Go linters
9. **commitlint**: Validates commit messages against Conventional Commits format
10. **reuse-lint-file**: Checks copyright and license information in files
11. **shell-fmt-go**: Formats shell scripts using `shfmt`
12. **shellcheck**: Lints shell scripts
13. **helmlint**: Lints Helm charts which requires `helm` command to be installed

## Running Manually

You can run pre-commit manually on all files:

```bash
pre-commit run --all-files
```

Or on specific files:

```bash
pre-commit run --files path/to/file1 path/to/file2
```

Or run a specific hook:

```bash
pre-commit run <hook-id> --all-files
```

## Commit Message Format

Our project follows the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Common types include:

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation changes
- **style**: Changes that don't affect the code's meaning (formatting, etc.)
- **refactor**: Code changes that neither fix a bug nor add a feature
- **test**: Adding or correcting tests
- **chore**: Changes to the build process, tools, etc.

## Updating Hooks

To update the hooks to their latest versions:

```bash
pre-commit autoupdate
```

## Configuration Files

The project includes several configuration files:

- `.pre-commit-config.yaml`: Main pre-commit configuration
- `.yamllint.yaml`: Configuration for YAML linting
- `.markdownlint.yaml`: Configuration for Markdown linting
- `.commitlintrc.yaml`: Configuration for commit message linting
- `.codespellignore`: Words to ignore in spellchecking

## Troubleshooting

For more help, run `pre-commit --help` or refer to the [official documentation](https://pre-commit.com/).
