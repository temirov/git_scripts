# Notes

## Rules of engagement

Review the notes.md. Make a plan for autonomously fixing every bug. Ensure no regressions. Ensure adding tests. Lean into integration tests. Fix every bug.

Fix bugs one by one. Write a nice comprehensive commit message AFTER EACH bug is fixed and tested and covered with tests. Do not work on all bugs at all. Work at one bug at a time sequntially. 

Remove a bug from the notes.md after the bug is fixed. commit and push to the remote.

Leave BugFixes section empty but don't delete the section itself.

## BugFixes

### Change root sematics and rename to roots

- [ ] Allow multiple folders to be passed under --roots. Remove --root and replace it with --roots which accepts mutiple roots. Check if one includes another and silently ignore the inclusion
```shell
11:34:36 tyemirov@Vadyms-MacBook-Pro:~/Development/git_scripts - [master] $ gix audit --roots ../
unknown flag: --roots
```
Test and document

### Add all folders under audit

- [ ] add a flag to include non git enabled folders in the audit command, --all. when added this flag will include folders without git repo in the output, leaving all git-related fileds as n/a
