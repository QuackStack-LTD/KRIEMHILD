#!/usr/bin/env bash
# Pull requests only build and check; no credentials or registry writes.
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/image.sh"
main() {
  local revision
  revision=$(image_revision)
  check_image "kriemhild:check-${revision:0:12}"
}
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main "$@"; fi
