#!/bin/bash

base_dir="$HOME/Development"

for repo in "$base_dir"/*; do
  if [ -d "$repo/.git" ]; then
    cd "$repo" || continue
    hits=$(git rev-list --all -- AGENTS.md 2>/dev/null)
    if [ -n "$hits" ]; then
      echo "=== Repository: $repo ==="
      git log --all --pretty=format:'%h %d %s' -- AGENTS.md
      echo
    fi
  fi
done
