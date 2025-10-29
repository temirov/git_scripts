# Command Failure Classification

The table below categorises the major maintenance commands into **fatal** and **non‑fatal** steps. Non‑fatal steps emit structured warnings (`FETCH-SKIP`, `PULL-SKIP`, `PAGES-SKIP`, `PR-RETARGET-SKIP`, `PROTECTION-SKIP`, `DELETE-SKIP`) while the command continues processing the remaining repositories.

| Command | Step | Classification | Behaviour |
| --- | --- | --- | --- |
| branch cd | Enumerate remotes, switch branch, create branch (when missing) | Fatal | Missing dependencies or branch creation errors abort execution. |
|  | Fetch remote (`git fetch`) | Non-fatal | Logged as `FETCH-SKIP` and the command proceeds without pulling. |
|  | Pull branch (`git pull --rebase`) | Non-fatal | Logged as `PULL-SKIP`; branch switch still succeeds. |
|  | Dry-run skip | Non-fatal | Explicit message and continue. |
|  | Remote/local deletion (branch cleanup) | Non-fatal | Errors appear as warnings; remaining branches processed. |
| branch default | Workflow rewrite, default branch update | Fatal | Required to guarantee correctness. |
|  | GitHub Pages update | Non-fatal | Logged as `PAGES-SKIP`; migration continues. |
|  | Pull request listing | Non-fatal | Logged as `PR-LIST-SKIP`; migration continues. |
|  | Pull request retarget | Non-fatal | Each failure logs `PR-RETARGET-SKIP`; other PRs still processed. |
|  | Branch protection check | Non-fatal | Logged as `PROTECTION-SKIP`; deletion guarded by safety gate. |
|  | Source branch deletion | Non-fatal | Logged as `DELETE-SKIP`; migration still reports success. |
| Repo remote/protocol/rename | Validation, remote URL construction, filesystem rename | Fatal | These steps define the primary behaviour; failures abort execution. |
| branch cleanup | Confirmation, branch deletion | Non-fatal | Deletion failures logged; other branches continue. |
| Workflow runner | Operation execution | Fatal (operation-defined) | Operations decide whether to downgrade issues; warnings bubble via environment output. |

> Note: Commands that operate on remote URLs or filesystem mutations (`repo remote update`, `repo protocol convert`, `repo folder rename`, etc.) are treated as fatal for their core steps. Their tasks either succeed or abort with the contextual error catalogue introduced in prior issues.
