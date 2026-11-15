#!/usr/bin/env bash
# Shared build functions. Publication is invoked only by the main workflow.
set -euo pipefail
ci_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)

image_revision() {
  local revision=${GITHUB_SHA:-$(git -C "$ci_root" rev-parse HEAD)}
  [[ "$revision" =~ ^[0-9a-f]{40}$ ]] || { echo 'Invalid commit SHA' >&2; return 1; }
  printf '%s' "$revision"
}

build_image() {
  local mode=$1 metadata=$2
  shift 2
  local revision
  revision=$(image_revision)
  local args=(--file "$ci_root/Dockerfile" --target runtime --metadata-file "$metadata"
    --label org.opencontainers.image.title=KRIEMHILD
    --label "org.opencontainers.image.source=https://github.com/${GITHUB_REPOSITORY:-QuackStack-LTD/KRIEMHILD}"
    --label "org.opencontainers.image.revision=$revision")
  local tag
  for tag in "$@"; do args+=(--tag "$tag"); done
  if [[ -n ${KRIEMHILD_BUILDX_CACHE:-} ]]; then
    if [[ -f "$KRIEMHILD_BUILDX_CACHE/index.json" ]]; then
      args+=(--cache-from "type=local,src=$KRIEMHILD_BUILDX_CACHE")
    fi
    args+=(--cache-to "type=local,dest=$KRIEMHILD_BUILDX_CACHE,mode=max")
  fi
  case "$mode" in
    check) args+=(--platform linux/amd64 --load --provenance=false) ;;
    publish) args+=(--platform linux/amd64,linux/arm64 --push --provenance=mode=max --sbom=true) ;;
    *) echo 'Unknown image build mode' >&2; return 1 ;;
  esac
  docker buildx build "${args[@]}" "$ci_root"
}

check_image() {
  local image=$1 metadata
  metadata=$(mktemp)
  build_image check "$metadata" "$image"
  rm -f -- "$metadata"
  bash "$ci_root/scripts/container-smoke.sh" "$image"
}

publish_image() {
  local image=$1 revision=$2 metadata digest
  printf '%s' "${GH_TOKEN:?GH_TOKEN is required to publish}" |
    docker login ghcr.io --username "${GITHUB_ACTOR:?GITHUB_ACTOR is required}" --password-stdin
  metadata=$(mktemp)
  build_image publish "$metadata" "$image:sha-$revision" "$image:main" "$image:latest"
  digest=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["containerimage.digest"])' "$metadata")
  rm -f -- "$metadata"
  [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'Published image digest missing' >&2; return 1; }
  export KRIEMHILD_IMAGE_DIGEST=$digest
}

release_image() {
  local image=$1 revision=$2 tag="main-$2" notes
  notes=$(mktemp)
  printf 'Container: `%s:sha-%s`\n\nDigest: `%s`\n\nMoving tags: `main`, `latest`.\n\nPlatforms: linux/amd64, linux/arm64.\n' \
    "$image" "$revision" "$KRIEMHILD_IMAGE_DIGEST" > "$notes"
  if gh release view "$tag" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
    gh release edit "$tag" --repo "$GITHUB_REPOSITORY" --notes-file "$notes" --latest
  else
    gh release create "$tag" --repo "$GITHUB_REPOSITORY" --target "$revision" \
      --title "Main ${revision:0:12}" --notes-file "$notes" --generate-notes --latest
  fi
  rm -f -- "$notes"
}
