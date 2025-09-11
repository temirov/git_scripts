# build a set of existing remote branch names
REMOTE_BRANCHES=$(git ls-remote --heads origin | awk '{print $2}' | sed 's|refs/heads/||')

# now delete only if branch is in that set
gh pr list --state closed --json headRefName --limit 100000 -q '.[].headRefName' \
| while read branch; do
    if echo "$REMOTE_BRANCHES" | grep -qx "$branch"; then
        echo "Deleting remote: $branch"
        git push origin --delete "$branch" || true
        echo "Deleting local: $branch"
        git branch -D "$branch" 2>/dev/null || true
    else
        echo "Skipping (already gone): $branch"
    fi
done

