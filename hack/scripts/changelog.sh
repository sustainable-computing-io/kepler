#!/usr/bin/env bash

set -e -u -o pipefail

# Global variables
PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare FROM_TAG=""
declare TO_TAG=""
declare SHOW_HELP=false
declare EXCLUDED_KEYWORDS="chore|compose|ci|dependabot"

# Source utils.bash
source "$PROJECT_ROOT/hack/scripts/utils.bash"

# Print usage information
print_usage() {
    local scr
    scr="$(basename "$0")"

    read -r -d '' help <<-EOF_HELP || true
        Usage:
          $scr --from <tag> --to <tag>
          $scr --help | -h
         ─────────────────────────────────────────────────────────────────
        Generates a changelog between two Git tags.
EOF_HELP

    echo -e "$help"
    return 0
}

# Parse command-line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
        --help | -h)
            SHOW_HELP=true
            return 0
            ;;
        --from)
            shift
            if [[ $# -eq 0 ]]; then
                fail "missing value for --from"
            fi
            FROM_TAG="$1"
            if [[ "$FROM_TAG" == -* ]]; then
                fail "invalid value for --from: $FROM_TAG"
            fi
            shift
            ;;
        --to)
            shift
            if [[ $# -eq 0 ]]; then
                fail "missing value for --to"
            fi
            TO_TAG="$1"
            if [[ "$TO_TAG" == -* ]]; then
                fail "invalid value for --to: $TO_TAG"
            fi
            shift
            ;;
        *)
            return 1
            ;;
        esac
    done
    return 0
}

# Verify that the provided Git tags are valid and usable
verify_tags() {
    git rev-parse --verify "$FROM_TAG" >/dev/null 2>&1 || {
        err "Tag $FROM_TAG does not exist"
        return 1
    }
    git rev-parse --verify "$TO_TAG" >/dev/null 2>&1 || {
        err "Tag $TO_TAG does not exist"
        return 1
    }
    [[ "$FROM_TAG" == "$TO_TAG" ]] && {
        err "From and to tags are the same"
        return 1
    }
    return 0
}

# Generate the changelog by processing commits
generate_changelog() {
    local added_commits=()
    local fixed_commits=()
    local new_contrib_list=()
    local -A seen_authors

    while IFS=$'\t' read -r hash subject author; do
        local subject_lower="${subject,,}"

        # Categorize commits
        local section=""
        if [[ "$subject_lower" == feat* ]]; then
            section="Added"
        elif [[ "$subject_lower" == fix* ]]; then
            section="Fixed"
        else
            echo "Warning: Skipping unexpected commit type: $hash $subject" >&2
            continue
        fi

        # Store commit entry
        local commit_entry="- $hash $subject"
        if [[ "$section" == "Added" ]]; then
            added_commits+=("$commit_entry")
        elif [[ "$section" == "Fixed" ]]; then
            fixed_commits+=("$commit_entry")
        fi

        # Identify new contributors
        if [[ ! -v seen_authors["$author"] ]]; then
            seen_authors["$author"]=1
            if [[ -z "$(git log --oneline --author="$author" --before="$FROM_TAG" -n 1 --pretty=format:"%h" 2>/dev/null)" ]]; then
                new_contrib_list+=("- $author made their first contribution in this release ($hash)")
            fi
        fi
    done < <(git log --oneline --no-merges --pretty=format:"%h%x09%s%x09%an" "$FROM_TAG..$TO_TAG" |
        awk -v exclude_pattern="$EXCLUDED_KEYWORDS" '
            BEGIN { FS=OFS="\t" }
            (tolower($2) ~ /^feat(\(.*\))?:/ || tolower($2) ~ /^fix(\(.*\))?:/) && (tolower($2) !~ exclude_pattern) {
                print $1, $2, $3
            }
        ')

    # Output the changelog if there are entries
    if [[ ${#added_commits[@]} -gt 0 || ${#fixed_commits[@]} -gt 0 || ${#new_contrib_list[@]} -gt 0 ]]; then
        print_changelog
    else
        echo "No significant changes found between $FROM_TAG and $TO_TAG"
    fi
}

# Print the formatted changelog
print_changelog() {
    echo
    echo "$TO_TAG is out now, please see the changelog below for details! 🚀"
    echo
    echo "## Changelog"

    if [[ ${#added_commits[@]} -gt 0 ]]; then
        echo
        echo "### ✨ Added"
        printf '%s\n' "${added_commits[@]}"
    fi

    if [[ ${#fixed_commits[@]} -gt 0 ]]; then
        echo
        echo "### 🐛 Fixed"
        printf '%s\n' "${fixed_commits[@]}"
    fi

    if [[ ${#new_contrib_list[@]} -gt 0 ]]; then
        echo
        echo "### 🎉 New Contributors"
        printf '%s\n' "${new_contrib_list[@]}"
    fi

    echo
}

# Main execution flow
main() {
    parse_args "$@" || {
        print_usage
        fail "failed to parse arguments"
        return 1
    }

    if $SHOW_HELP; then
        print_usage
        return 0
    fi

    [[ -z "$FROM_TAG" || -z "$TO_TAG" ]] && {
        fail "from and to tags are required"
        return 1
    }

    verify_tags || return 1

    info "Generating changelog between $FROM_TAG and $TO_TAG"
    generate_changelog
    ok "Changelog generated successfully"
}

main "$@"
