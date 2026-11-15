#!/usr/bin/env bash
# Every main commit is checked before publishing its images and GitHub release.
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/image.sh"
main() {
  [[ ${GITHUB_REF:-} == refs/heads/main && ${GITHUB_EVENT_NAME:-} == push ]] || {
    echo 'Image publication requires a push to main.' >&2
    return 1
  }
  local revision image
  revision=$(image_revision)
  [[ ${GITHUB_REPOSITORY:-} =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || {
    echo 'GITHUB_REPOSITORY must identify the image repository.' >&2
    return 1
  }
  image="ghcr.io/${GITHUB_REPOSITORY,,}"
  check_image "kriemhild:check-${revision:0:12}"
  publish_image "$image" "$revision"
  release_image "$image" "$revision"
}
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main "$@"; fi
