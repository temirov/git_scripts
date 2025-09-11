#!/usr/bin/env bash
set -euo pipefail

# ---- CONFIG ----
# One of these must be set:
GITHUB_OWNER="temirov"            # user or org name that owns the package
PACKAGE_NAME="llm-proxy"          # container package name (as shown in GHCR)
OWNER_TYPE="user"                 # "user" or "org"
# Token must have: read:packages, write:packages, delete:packages
TOKEN="${GITHUB_PACKAGES_TOKEN}"

# Optional: preview without deleting
DRY_RUN="${DRY_RUN:-0}"           # set to 1 to preview

# ---- ENDPOINT ROOT ----
if [[ "$OWNER_TYPE" == "org" ]]; then
  BASE="https://api.github.com/orgs/${GITHUB_OWNER}/packages/container/${PACKAGE_NAME}"
else
  BASE="https://api.github.com/users/${GITHUB_OWNER}/packages/container/${PACKAGE_NAME}"
fi

page=1
total_deleted=0
while :; do
  versions_json="$(curl -sS \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "${BASE}/versions?per_page=100&page=${page}")"

  count="$(echo "${versions_json}" | jq 'length')"
  [[ "${count}" -eq 0 ]] && break

  # Select versions with NO tags (handle null tags via // [])
  echo "${versions_json}" \
  | jq -r '.[] | select((.metadata.container.tags // []) | length == 0) | .id' \
  | while read -r version_id; do
      [[ -z "${version_id}" ]] && continue
      echo "Deleting UNTAGGED version ${version_id}"
      if [[ "${DRY_RUN}" -ne 1 ]]; then
        curl -sS -X DELETE \
          -H "Authorization: Bearer ${TOKEN}" \
          -H "Accept: application/vnd.github+json" \
          "${BASE}/versions/${version_id}" >/dev/null
        total_deleted=$((total_deleted+1))
      fi
    done

  page=$((page+1))
done

echo "Done. Deleted ${total_deleted} untagged version(s). (Tagged versions were preserved.)"

