#!/usr/bin/env bash
# switch-default-to-master.sh
set -euo pipefail

abort() { echo "ERROR: $*" >&2; exit 1; }
log()   { echo "▶ $*"; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || abort "Missing dependency: $1"; }
require_cmd git
require_cmd gh

# Auth & repo checks
gh auth status >/dev/null 2>&1 || abort "GitHub CLI not authenticated. Run: gh auth login"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || abort "Not inside a Git repository"
[[ -z "$(git status --porcelain)" ]] || abort "Working tree is dirty. Commit or stash first."

# Identify repo (OWNER/REPO)
name_with_owner="$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || true)"
[[ -n "${name_with_owner}" ]] || abort "Failed to infer OWNER/REPO via gh. Does 'origin' point to GitHub?"
log "Repository: ${name_with_owner}"

# Ensure main exists, sync it
git show-ref --verify --quiet refs/heads/main || abort "Local branch 'main' not found."
log "Syncing local 'main' with origin"
git fetch origin main
git checkout main
git pull --ff-only origin main

# Create or fast-forward master
if git show-ref --verify --quiet refs/heads/master; then
  log "Fast-forwarding 'master' to 'main'"
  git checkout master
  git merge --ff-only main || abort "Cannot fast-forward 'master' to 'main' (diverged). Resolve manually."
else
  log "Creating 'master' from 'main'"
  git checkout -b master main
fi

# Push master (set upstream)
log "Pushing 'master' to origin"
git push -u origin master

# --- Rewrite Actions: main -> master (targeted) ---
rewrite_workflows() {
  local file_path="$1"
  # branches: ["main"]  -> ["master"]
  # branches:\n  - main ->   - master
  sed -i.bak \
    -e 's/\(\s*branches\s*:\s*\[\s*\)\(["'\'']\{0,1\}\)main\(["'\'']\{0,1\}\)\(\s*\]\)/\1\2master\3\4/g' \
    -e 's/^\(\s*-\s*\)\(["'\'']\{0,1\}\)main\(["'\'']\{0,1\}\)\s*$/\1\2master\3/g' \
    "$file_path" || return 1
  rm -f "${file_path}.bak"
}

workflows_dir=".github/workflows"
changed_workflow_count=0
if [[ -d "${workflows_dir}" ]]; then
  log "Retargeting GitHub Actions workflows 'main' → 'master'"
  while IFS= read -r -d '' workflow_file; do
    if grep -qE '^\s*branches\s*:|^\s*-\s*["'\'']?main["'\'']?\s*$' "${workflow_file}"; then
      rewrite_workflows "${workflow_file}" || abort "Rewrite failed for ${workflow_file}"
      changed_workflow_count=$((changed_workflow_count+1))
    fi
  done < <(find "${workflows_dir}" -type f \( -name "*.yml" -o -name "*.yaml" \) -print0)
  if (( changed_workflow_count > 0 )); then
    git add .github/workflows
    git commit -m 'CI: switch workflow branch filters to master'
    git push origin master
  fi
else
  log "No workflows directory found; skipping rewrite."
fi

# --- GitHub Pages guard (preserve deployments) ---
# If Pages is enabled and its source.branch == "main" in legacy mode (build_type "legacy"),
# move the Pages source to {branch:"master", path:<existing>}.
# (If build_type is "workflow", our workflow rewrite above already retargets to master.)
log "Checking GitHub Pages configuration"
pages_json="$(gh api -X GET "repos/${name_with_owner}/pages" 2>/dev/null || true)"
if [[ -n "${pages_json}" && "${pages_json}" != "null" ]]; then
  pages_branch="$(echo "${pages_json}" | jq -r '.source.branch // empty')"
  pages_path="$(echo   "${pages_json}" | jq -r '.source.path   // "/"')"
  build_type="$(echo   "${pages_json}" | jq -r '.build_type    // "legacy"')"

  log "Pages enabled: branch=${pages_branch:-<none>} path=${pages_path} build_type=${build_type}"

  if [[ "${build_type}" == "legacy" && "${pages_branch}" == "main" ]]; then
    log "Updating GitHub Pages publishing source to master (${pages_path})"
    jq -n --arg path "${pages_path}" '{source:{branch:"master", path:$path}}' \
    | gh api -X PUT "repos/${name_with_owner}/pages" \
        --input - \
        -H "Content-Type: application/json" \
        -H "Accept: application/vnd.github+json" \
        >/dev/null
  fi
else
  log "GitHub Pages not configured; skipping."
fi
# (API: GET/PUT /repos/{owner}/{repo}/pages support reading & changing source.branch/path.) :contentReference[oaicite:2]{index=2}

# --- Flip default branch on GitHub ---
log "Setting default branch to 'master' on GitHub"
gh api -X PATCH "repos/${name_with_owner}" -f default_branch=master
# (API: PATCH /repos/{owner}/{repo} default_branch=...) :contentReference[oaicite:3]{index=3}

# --- DEFAULT: rebase local branches created off main onto master ---
log "Rebasing local branches that were created off 'main' onto 'master'"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
mapfile -t local_branches < <(git for-each-ref --format='%(refname:short)' refs/heads | grep -Ev '^(main|master)$' || true)

[[ ! -d .git/rebase-apply && ! -d .git/rebase-merge ]] || abort "A rebase is in progress; resolve/abort first."

for branch in "${local_branches[@]}"; do
  if git merge-base --is-ancestor main "${branch}"; then
    log "→ ${branch}: git rebase master"
    git checkout "${branch}"
    if ! git rebase master; then
      abort "Rebase conflict on ${branch}. Resolve (git status), then: git rebase --continue (or --abort) and re-run."
    fi
    if git rev-parse --abbrev-ref "${branch}@{upstream}" >/dev/null 2>&1; then
      git push --force-with-lease
    fi
  else
    log "→ ${branch}: skip (not based on main)"
  fi
done
if [[ "$(git rev-parse --abbrev-ref HEAD)" != "${current_branch}" ]]; then
  git checkout "${current_branch}"
fi

# --- DEFAULT: retarget open PRs base main → master ---
log "Retargeting open PRs base main → master"
for pr in $(gh pr list --state open --base main --json number --jq '.[].number'); do
  gh pr edit "${pr}" --base master
done
# (API: List/Edit PRs) :contentReference[oaicite:4]{index=4}

# --- Safety gates before deleting main ---
open_pr_count="$(gh api "repos/${name_with_owner}/pulls" -f state=open -f base=main --jq 'length' 2>/dev/null || echo 0)"
log "Open PRs targeting 'main': ${open_pr_count}"

main_protection=0
if gh api "repos/${name_with_owner}/branches/main/protection" >/dev/null 2>&1; then
  main_protection=1
fi
log "Branch protection on 'main': $([[ ${main_protection} -eq 1 ]] && echo yes || echo no)"

leftover_refs="0"
if [[ -d "${workflows_dir}" ]]; then
  if grep -R --line-number --include="*.yml" --include="*.yaml" -E '\bmain\b' "${workflows_dir}" >/dev/null 2>&1; then
    leftover_refs="1"
  fi
fi
log "Remaining 'main' mentions in workflows: $([[ ${leftover_refs} == 1 ]] && echo yes || echo no)"

if [[ "${open_pr_count}" == "0" && "${main_protection}" -eq 0 && "${leftover_refs}" == "0" ]]; then
  log "Safe to delete 'main' (remote and local)."
  git push origin --delete main || true
  git branch -D main || true
else
  log "NOT deleting 'main' (safety gates not met). You can delete it later once clean."
fi

log "Done. Default branch is 'master'."

