#!/usr/bin/env bash
set -euo pipefail
export GIT_TERMINAL_PROMPT=0

require_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: missing dependency: $1" >&2; exit 1; }; }

csv_escape() {
  local string_value="$1"
  if [[ "$string_value" == *","* || "$string_value" == *"\""* || "$string_value" == *$'\n'* ]]; then
    string_value="${string_value//\"/\"\"}"; printf '"%s"' "$string_value"
  else
    printf '%s' "$string_value"
  fi
}

print_csv_header() {
  echo "final_github_repo,folder_name,name_matches,remote_default_branch,local_branch,in_sync,remote_protocol,origin_matches_canonical"
}

origin_url_raw() { git remote get-url origin 2>/dev/null || true; }

# Resolve redirected/canonical repo data via GitHub API (follows renames/transfers; forks unchanged).
# Returns JSON: {"full_name":"owner/repo","id":123456789}
resolve_repo_meta_json() {
  local owner_repo_input="$1"
  [[ -n "$owner_repo_input" ]] || { echo ""; return 0; }
  gh api -H "Accept: application/vnd.github+json" "repos/${owner_repo_input}" \
    --jq '{full_name:.full_name,id:.id}' 2>/dev/null || true
}

owner_repo_from_url() {
  local url_input="$1"
  local normalized_url="$1"
  case "$url_input" in
    git@github.com:*)       normalized_url="https://github.com/${url_input#git@github.com:}" ;;
    ssh://git@github.com/*) normalized_url="https://github.com/${url_input#ssh://git@github.com/}" ;;
    https://github.com/*)   normalized_url="$url_input" ;;
    *)                      normalized_url="$url_input" ;;
  esac
  normalized_url="${normalized_url%.git}"
  local path_part="${normalized_url#https://github.com/}"
  IFS='/' read -r owner_part repo_part _ <<<"$path_part"
  if [[ -n "${owner_part:-}" && -n "${repo_part:-}" ]]; then
    echo "${owner_part}/${repo_part}"
  else
    echo ""
  fi
}

detect_remote_protocol_from_url() {
  local url_value="$1"
  if   [[ "$url_value" =~ ^git@github\.com: ]];       then echo "git"
  elif [[ "$url_value" =~ ^ssh://git@github\.com/ ]]; then echo "ssh"
  elif [[ "$url_value" =~ ^https://github\.com/ ]];   then echo "https"
  else echo "other"; fi
}

get_remote_default_branch() {
  local branch_name; branch_name="$(gh repo view --json defaultBranchRef --jq .defaultBranchRef.name 2>/dev/null || true)"
  if [[ -n "$branch_name" && "$branch_name" != "null" ]]; then echo "$branch_name"; return 0; fi
  local symref; symref="$(git ls-remote --symref origin HEAD 2>/dev/null | awk '/^ref:/ {print $2}' || true)"
  [[ -n "$symref" ]] && echo "${symref##refs/heads/}" || echo ""
}

get_local_branch() {
  local branch_value; branch_value="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  if [[ -z "$branch_value" ]]; then echo ""
  elif [[ "$branch_value" == "HEAD" ]]; then echo "DETACHED"
  else echo "$branch_value"; fi
}

compute_in_sync_flag() {
  local remote_default_branch="$1"
  local local_branch="$2"
  local remote_protocol="$3"
  if [[ -z "$remote_default_branch" || -z "$local_branch" || "$local_branch" != "$remote_default_branch" ]]; then
    echo "n/a"; return 0
  fi
  if [[ "$remote_protocol" != "git" && "$remote_protocol" != "ssh" ]]; then
    echo "n/a"; return 0
  fi
  if ! git fetch -q --no-tags --no-recurse-submodules origin "$remote_default_branch" >/dev/null 2>&1; then
    echo "n/a"; return 0
  fi
  local upstream_ref local_sha remote_sha
  upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || true)"
  local_sha="$(git rev-parse HEAD 2>/dev/null || true)"
  if [[ -n "$upstream_ref" ]]; then
    remote_sha="$(git rev-parse "$upstream_ref" 2>/dev/null || true)"
  else
    remote_sha="$(git rev-parse "refs/remotes/origin/${remote_default_branch}" 2>/dev/null || \
                  git rev-parse "origin/${remote_default_branch}" 2>/dev/null || true)"
  fi
  if [[ -z "$local_sha" || -z "$remote_sha" ]]; then echo "n/a"
  elif [[ "$local_sha" == "$remote_sha" ]]; then echo "yes"
  else echo "no"; fi
}

is_clean_worktree() { [[ -z "$(git status --porcelain 2>/dev/null || true)" ]] ; }

make_url_for_protocol() {
  local protocol_kind="$1"
  local owner_repo_value="$2"
  case "$protocol_kind" in
    git)   echo "git@github.com:${owner_repo_value}.git" ;;
    ssh)   echo "ssh://git@github.com/${owner_repo_value}.git" ;;
    https) echo "https://github.com/${owner_repo_value}.git" ;;
    *)     echo "" ;;
  esac
}

plan_validate_rename() {
  local old_abs_path="$1"; local new_abs_path="$2"; local require_clean_flag="$3"
  if [[ "$old_abs_path" == "$new_abs_path" ]]; then echo "PLAN-SKIP (already named): $old_abs_path"; return 1; fi
  if [[ "$require_clean_flag" == "true" ]]; then
    pushd "$old_abs_path" >/dev/null 2>&1 || true
    if ! is_clean_worktree; then echo "PLAN-SKIP (dirty worktree): $old_abs_path"; popd >/dev/null 2>&1 || true; return 1; fi
    popd >/dev/null 2>&1 || true
  fi
  local new_parent_dir; new_parent_dir="$(dirname "$new_abs_path")"
  if [[ ! -d "$new_parent_dir" ]]; then echo "PLAN-SKIP (target parent missing): $new_parent_dir"; return 1; fi
  if [[ -e "$new_abs_path" ]]; then echo "PLAN-SKIP (target exists): $new_abs_path"; return 1; fi
  local is_case_only="false"; shopt -s nocasematch
  if [[ "${old_abs_path,,}" == "${new_abs_path,,}" ]]; then is_case_only="true"; fi
  shopt -u nocasematch
  if [[ "$is_case_only" == "true" ]]; then
    echo "PLAN-CASE-ONLY: $old_abs_path → $new_abs_path (two-step move required)"
  else
    echo "PLAN-OK: $old_abs_path → $new_abs_path"
  fi
  return 0
}

apply_validate_rename() {
  local old_abs_path="$1"; local new_abs_path="$2"; local require_clean_flag="$3"
  if [[ "$old_abs_path" == "$new_abs_path" ]]; then echo "ERROR: already named: $old_abs_path" >&2; return 1; fi
  if [[ "$require_clean_flag" == "true" ]]; then
    pushd "$old_abs_path" >/dev/null 2>&1 || true
    if ! is_clean_worktree; then echo "ERROR: dirty worktree: $old_abs_path" >&2; popd >/dev/null 2>&1 || true; return 1; fi
    popd >/dev/null 2>&1 || true
  fi
  local new_parent_dir; new_parent_dir="$(dirname "$new_abs_path")"
  if [[ ! -d "$new_parent_dir" ]]; then echo "ERROR: target parent missing: $new_parent_dir" >&2; return 1; fi
  if [[ -e "$new_abs_path" ]]; then echo "ERROR: target exists: $new_abs_path" >&2; return 1; fi
  return 0
}

safe_mv_repo_dir() {
  local old_abs_path="$1"; local new_abs_path="$2"
  local is_case_only="false"; shopt -s nocasematch
  if [[ "${old_abs_path,,}" == "${new_abs_path,,}" ]]; then is_case_only="true"; fi
  shopt -u nocasematch
  if [[ "$is_case_only" == "true" ]]; then
    local temp_path="${old_abs_path}.rename.$$"
    mv "$old_abs_path" "$temp_path"
    mv "$temp_path" "$new_abs_path"
  else
    mv "$old_abs_path" "$new_abs_path"
  fi
  echo "Renamed $old_abs_path → $new_abs_path"
}

main() {
  require_cmd git; require_cmd gh
  gh auth status >/dev/null 2>&1 || { echo "ERROR: run gh auth login" >&2; exit 1; }

  local do_audit="false"
  local do_rename="false"
  local do_update_remote="false"
  local proto_from=""
  local proto_to=""
  local dry_run="false"
  local assume_yes="false"
  local require_clean="false"
  local debug_mode="false"

  declare -a scan_roots=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --audit)            do_audit="true"; shift ;;
      --rename)           do_rename="true"; shift ;;
      --update-remote)    do_update_remote="true"; shift ;;
      --protocol-from)    proto_from="${2:-}"; shift 2 ;;
      --protocol-to)      proto_to="${2:-}"; shift 2 ;;
      --dry-run)          dry_run="true"; shift ;;
      --yes|-y)           assume_yes="true"; shift ;;
      --require-clean)    require_clean="true"; shift ;;
      --debug)            debug_mode="true"; shift ;;
      --*)                echo "ERROR: unknown flag $1" >&2; exit 1 ;;
      *)                  scan_roots+=("$1"); shift ;;
    esac
  done
  [[ "${#scan_roots[@]}" -eq 0 ]] && scan_roots=(".")

  case "$proto_from" in ""|https|git|ssh) ;; *) echo "ERROR: --protocol-from must be one of https|git|ssh" >&2; exit 1;; esac
  case "$proto_to"   in ""|https|git|ssh) ;; *) echo "ERROR: --protocol-to must be one of https|git|ssh"   >&2; exit 1;; esac

  if [[ "$do_audit" == "false" && "$do_rename" == "false" && "$do_update_remote" == "false" && -z "$proto_from" && -z "$proto_to" ]]; then
    echo "ERROR: specify --audit, or --rename / --update-remote (with optional --dry-run/--yes), or a protocol conversion with --protocol-from/--protocol-to" >&2
    exit 1
  fi

  local do_protocol="false"
  if [[ -n "$proto_from" || -n "$proto_to" ]]; then
    if [[ -z "$proto_from" || -z "$proto_to" ]]; then
      echo "ERROR: protocol conversion requires both --protocol-from and --protocol-to" >&2
      exit 1
    fi
    if [[ "$proto_from" == "$proto_to" ]]; then
      echo "ERROR: --protocol-from and --protocol-to are the same ($proto_from)" >&2
      exit 1
    fi
    do_protocol="true"
  fi

  local audit_allowed="true"
  if [[ "$do_rename" == "true" || "$do_protocol" == "true" || "$do_update_remote" == "true" ]]; then
    audit_allowed="false"
  fi

  mapfile -d '' -t candidate_repo_dirs < <(
    {
      find "${scan_roots[@]}" -type d -name .git -print0 2>/dev/null
      find "${scan_roots[@]}" -type f -name .git -print0 2>/dev/null
    } | while IFS= read -r -d '' git_path; do
          printf '%s\0' "$(dirname "$git_path")"
        done
  )

  declare -A seen=()
  [[ "$debug_mode" == "true" ]] && echo "DEBUG: discovered ${#candidate_repo_dirs[@]} candidate repos under: ${scan_roots[*]}" >&2

  if [[ "$do_audit" == "true" && "$audit_allowed" == "true" ]]; then
    print_csv_header
  fi

  for repository_directory in "${candidate_repo_dirs[@]}"; do
    [[ -n "${seen[$repository_directory]+x}" ]] && continue
    seen["$repository_directory"]=1

    [[ "$debug_mode" == "true" ]] && echo "DEBUG: checking $repository_directory" >&2

    if ! pushd "$repository_directory" >/dev/null 2>&1; then continue; fi
    if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then popd >/dev/null 2>&1 || true; continue; fi

    origin_url="$(origin_url_raw)"
    if [[ ! "$origin_url" =~ github\.com[:/].+ ]]; then popd >/dev/null 2>&1 || true; continue; fi

    folder_name="$(basename "$repository_directory")"
    origin_owner_repo="$(owner_repo_from_url "$origin_url")"
    remote_protocol="$(detect_remote_protocol_from_url "$origin_url")"

    # Use GitHub API to resolve canonical (redirect target) – does not collapse forks.
    repo_meta_json="$(resolve_repo_meta_json "$origin_owner_repo")"
    canonical_owner_repo="$(jq -r '.full_name // empty' <<<"$repo_meta_json")"

    final_repo_name=""
    if [[ -n "$canonical_owner_repo" ]]; then
      final_repo_name="${canonical_owner_repo##*/}"
    else
      final_repo_name="${origin_owner_repo##*/}"
    fi

    if [[ -n "$final_repo_name" && "$final_repo_name" == "$folder_name" ]]; then
      name_matches="yes"
    else
      name_matches="no"
    fi

    if [[ "$do_audit" == "true" && "$audit_allowed" == "true" ]]; then
      remote_default_branch="$(get_remote_default_branch || true)"
      local_branch="$(get_local_branch || true)"
      in_sync="$(compute_in_sync_flag "$remote_default_branch" "$local_branch" "$remote_protocol" || true)"
      if [[ -z "$origin_owner_repo" || -z "$canonical_owner_repo" ]]; then
        origin_matches_canonical="n/a"
      else
        if [[ "${origin_owner_repo,,}" == "${canonical_owner_repo,,}" ]]; then origin_matches_canonical="yes"; else origin_matches_canonical="no"; fi
      fi
      printf "%s,%s,%s,%s,%s,%s,%s,%s\n" \
        "$(csv_escape "${canonical_owner_repo:-$origin_owner_repo}")" \
        "$(csv_escape "$folder_name")" \
        "$(csv_escape "$name_matches")" \
        "$(csv_escape "$remote_default_branch")" \
        "$(csv_escape "$local_branch")" \
        "$(csv_escape "$in_sync")" \
        "$(csv_escape "$remote_protocol")" \
        "$(csv_escape "$origin_matches_canonical")"
    fi

    popd >/dev/null 2>&1 || true

    # Update remote only on true rename/transfer (canonical != origin path).
    if [[ "$do_update_remote" == "true" ]]; then
      if [[ -z "$origin_owner_repo" ]]; then
        echo "UPDATE-REMOTE-SKIP: $repository_directory (error: could not parse origin owner/repo)"
      elif [[ -z "$canonical_owner_repo" ]]; then
        echo "UPDATE-REMOTE-SKIP: $repository_directory (no upstream: no canonical redirect found)"
      elif [[ "${canonical_owner_repo,,}" == "${origin_owner_repo,,}" ]]; then
        echo "UPDATE-REMOTE-SKIP: $repository_directory (already canonical)"
      else
        current_protocol_for_update="$remote_protocol"
        target_url_for_update="$(make_url_for_protocol "$current_protocol_for_update" "$canonical_owner_repo")"
        if [[ -z "$target_url_for_update" ]]; then
          echo "UPDATE-REMOTE-SKIP: $repository_directory (error: could not construct target URL)"
        else
          if [[ "$dry_run" == "true" ]]; then
            echo "PLAN-UPDATE-REMOTE: $repository_directory origin $origin_url → $target_url_for_update"
          else
            if [[ "$assume_yes" == "false" ]]; then
              read -r -p "Update 'origin' in '$repository_directory' to canonical ($origin_owner_repo → $canonical_owner_repo)? [y/N] " answer_update
              case "${answer_update,,}" in y|yes) ;; *) echo "UPDATE-REMOTE-SKIP: user declined for $repository_directory"; continue ;; esac
            fi
            if git -C "$repository_directory" remote set-url origin "$target_url_for_update" 2>/dev/null; then
              echo "UPDATE-REMOTE-DONE: $repository_directory origin now $target_url_for_update"
            else
              echo "UPDATE-REMOTE-SKIP: $repository_directory (error: failed to set origin URL)"
            fi
          fi
        fi
      fi
    fi

    if [[ "$do_rename" == "true" && -n "$final_repo_name" && "$final_repo_name" != "$folder_name" ]]; then
      old_abs_path="$(readlink -f "$repository_directory" 2>/dev/null || realpath "$repository_directory" 2>/dev/null || echo "$repository_directory")"
      parent_dir_path="$(dirname "$old_abs_path")"
      new_abs_path="${parent_dir_path}/${final_repo_name}"
      if [[ "$dry_run" == "true" ]]; then
        plan_validate_rename "$old_abs_path" "$new_abs_path" "$require_clean" || true
      else
        if ! apply_validate_rename "$old_abs_path" "$new_abs_path" "$require_clean"; then
          continue
        fi
        if [[ "$assume_yes" == "false" ]]; then
          read -r -p "Rename '$old_abs_path' → '$new_abs_path'? [y/N] " user_answer
          case "${user_answer,,}" in y|yes) ;; *) echo "SKIP: $old_abs_path"; continue ;; esac
        fi
        safe_mv_repo_dir "$old_abs_path" "$new_abs_path" || true
      fi
    fi

    if [[ "$do_protocol" == "true" ]]; then
      origin_url_now="$(git -C "$repository_directory" remote get-url origin 2>/dev/null || true)"
      current_protocol="$(detect_remote_protocol_from_url "$origin_url_now")"
      if [[ "$current_protocol" == "$proto_from" ]]; then
        # Use canonical only if redirect exists; preserve forks.
        owner_repo_for_protocol="${canonical_owner_repo:-$origin_owner_repo}"
        if [[ -z "$owner_repo_for_protocol" ]]; then
          echo "ERROR: cannot derive owner/repo for protocol conversion in $repository_directory" >&2
          continue
        fi
        target_url_value="$(make_url_for_protocol "$proto_to" "$owner_repo_for_protocol")"
        if [[ -z "$target_url_value" ]]; then
          echo "ERROR: cannot build target URL for protocol '$proto_to' in $repository_directory" >&2
          continue
        fi
        if [[ "$dry_run" == "true" ]]; then
          echo "PLAN-CONVERT: $repository_directory origin $origin_url_now → $target_url_value"
        else
          if [[ "$assume_yes" == "false" ]]; then
            read -r -p "Convert 'origin' in '$repository_directory' ($current_protocol → $proto_to)? [y/N] " convert_answer
            case "${convert_answer,,}" in y|yes) ;; *) echo "CONVERT-SKIP: user declined for $repository_directory"; continue ;; esac
          fi
          if git -C "$repository_directory" remote set-url origin "$target_url_value" 2>/dev/null; then
            echo "CONVERT-DONE: $repository_directory origin now $target_url_value"
          else
            echo "ERROR: failed to set origin to $target_url_value in $repository_directory" >&2
          fi
        fi
      fi
    fi

  done
}

main "$@"
