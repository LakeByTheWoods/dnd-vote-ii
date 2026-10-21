# PR #2 access-control review — 22 October 2026

## Decision

Keep PR #2 open and draft until the actual runtime configuration is verified.
Mergeability is not evidence that the application can start or be reached after
deployment. No production service or database was changed during this review.

## Code review

Reviewed the original head `abd16f3345040e28eded5b1b8f5d70f965d685a5`,
including server authorisation, ballot replacement transactions, schema migration,
credential storage/transport, static file routing and the client actions.
At review start GitHub reported no reviews, review threads, check runs or commit
statuses. The PR was a mergeable draft.

Host checks protect discovery, creation and deletion. Ballot replacement checks
the receipt inside a serialized SQLite transaction. Public reads omit credentials
and receipt hashes; static serving is allowlisted. No additional authorisation
bypass was identified in this focused review. Account-free voting still allows
poll-link holders to create ballots under new names, as documented.

Found and fixed a client correctness issue: Unicode lowercasing made distinct
SQLite NOCASE names such as `Älice` and `älice` use one receipt storage slot.
The regression failed with HTTP 403 before the fix. Receipt keys and ballot
lookup now share ASCII-only case folding to match SQLite. Previously stored
receipts for accented uppercase names may require host-assisted recovery;
the recovery procedure is documented in README.txt.

The startup banner now prints the actual bound port, including when port 0 asks
the operating system to select a free port.

## Automated evidence

- 18 Python tests pass: the existing 15 tests plus three production-entry-point
  checks for environment/CLI listener configuration, health access, and SQLite
  ballot/receipt persistence across process restart and host-key rotation.
- One Node.js HTTP integration test passes, extended with the accented-name
  receipt regression.
- Python compilation, syntax checks for all four JavaScript files, and
  `git diff --check` pass.
- Added `.github/workflows/checks.yml` to run Python, client and syntax checks
  on PRs and pushes to master/the access-control branch, with read-only contents
  permission and no deployment or production secrets.

These checks use temporary local databases. They do not validate live HTTPS,
the actual proxy, a provider's volume durability, backups, or rendered browser
interactions. GitHub CI execution is separate from the local evidence above.

## Unresolved deployment requirements

Prior user direction excludes Railway. The remaining `railway.toml` is a legacy
file and does not establish the current host. No replacement host or effective
runtime settings were established from the available repository and prior context.

Before merging into a branch that could deploy, establish:

1. The actual host/service, deployed branch and startup command, including whether
   merging master triggers automatic deployment.
2. A privately provisioned random `DND_VOTE_ADMIN_TOKEN` of at least 32 ASCII
   characters without whitespace. Do not put its value in GitHub or this report.
   Missing/invalid configuration causes startup failure.
3. The required listener: loopback works for a same-machine reverse proxy;
   containers/external proxies may require explicit `HOST=0.0.0.0` or
   `--host 0.0.0.0`, with the correct `PORT`. The default is 127.0.0.1:8000.
4. `DND_VOTE_DATA_DIR` mapped to a private, writable, durable directory holding
   the intended existing `dnd_vote.db`, with a verified backup before migration.
   A temporary application filesystem is insufficient. Confirm an appropriate
   single-instance SQLite arrangement rather than independent replica databases.
5. HTTPS termination and a controlled verification of health access, host unlock,
   unauthorised rejection, public voting/editing, and data survival after a host
   restart/redeploy. Use a dedicated test poll and preserve existing ballots.

Do not infer any of these settings from successful local tests or stale hosting
configuration. Once the actual hosting arrangement is supplied, verify these
requirements there before completing the merge.
