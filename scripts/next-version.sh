#!/usr/bin/env bash
# Prints the next release tag (vMAJOR.MINOR.PATCH) derived from the
# Conventional Commits since the latest SemVer tag reachable from HEAD, or
# nothing when no commit warrants a release. With --notes, prints the release
# notes (feat/fix/perf subjects since that tag) instead. Never creates tags.
#
#   breaking change (type!: or a BREAKING CHANGE: footer) -> major (minor on 0.x)
#   feat                                                   -> minor
#   fix, perf                                              -> patch
#   anything else, merge commits, non-conventional commits -> no release
set -euo pipefail

mode="${1:-}"
if [[ -n "$mode" && "$mode" != "--notes" ]]; then
  echo "usage: $0 [--notes]" >&2
  exit 2
fi

base=$(git tag --merged HEAD --list 'v*' \
  | grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
  | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1 || true)
range=HEAD
[[ -z "$base" ]] || range="v$base..HEAD"

header='^([a-z]+)(\([^)]*\))?(!)?: (.+)$'
level=0
features=()
fixes=()
while IFS= read -r -d '' message; do
  subject="${message%%$'\n'*}"
  [[ "$subject" =~ $header ]] || continue
  type="${BASH_REMATCH[1]}"
  bang="${BASH_REMATCH[3]}"
  commit_level=0
  case "$type" in
    feat) commit_level=2; features+=("$subject") ;;
    fix | perf) commit_level=1; fixes+=("$subject") ;;
  esac
  if [[ -n "$bang" ]] || grep -qE '^BREAKING[ -]CHANGE: ' <<<"$message"; then
    commit_level=3
  fi
  ((commit_level > level)) && level=$commit_level
done < <(git log -z --no-merges --format=%B "$range")

if [[ "$mode" == "--notes" ]]; then
  if ((${#features[@]})); then
    printf '## Features\n\n'
    printf -- '- %s\n' "${features[@]}"
    printf '\n'
  fi
  if ((${#fixes[@]})); then
    printf '## Fixes\n\n'
    printf -- '- %s\n' "${fixes[@]}"
  fi
  exit 0
fi

((level > 0)) || exit 0
if [[ -z "$base" ]]; then
  echo "v0.0.1"
  exit 0
fi
IFS=. read -r major minor patch <<<"$base"
((level == 3 && major == 0)) && level=2
case "$level" in
  3) major=$((major + 1)); minor=0; patch=0 ;;
  2) minor=$((minor + 1)); patch=0 ;;
  1) patch=$((patch + 1)) ;;
esac
echo "v$major.$minor.$patch"
