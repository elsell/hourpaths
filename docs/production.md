# Production checklist

Deploy the API and web images by immutable digest. Run PostgreSQL migrations and
the SpiceDB datastore migration/schema jobs with separate credentials before
starting API replicas. API database credentials must not own schema or migration
state; API SpiceDB credentials go through the capability proxy and cannot change
schema.

Required preparation:

- Set `HOURPATHS_APP_ENV=production`, `HOURPATHS_API_URL`,
  `HOURPATHS_OIDC_ISSUER`, and `HOURPATHS_WEB_OIDC_CLIENT_ID` for the web container.
  The image defaults to production mode and refuses to serve with absent, local,
  credentialed, or non-HTTPS production endpoints. Local Compose explicitly
  overrides this to development.
- Set `HOURPATHS_CURRENT_POLICY_REVISION` to a positive, monotonically increasing
  number, incrementing it whenever any published policy version or location
  changes. Set `HOURPATHS_CURRENT_TERMS_VERSION`,
  `HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION`, and
  `HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION` to the exact immutable version
  identifiers users are accepting or acknowledging. Set `HOURPATHS_TERMS_URL`,
  `HOURPATHS_PRIVACY_POLICY_URL`, `HOURPATHS_COMMUNITY_GUIDELINES_URL`, and
  `HOURPATHS_SUPPORT_URL` to their published HTTPS locations; none of these API
  settings has a production default.
- After migrations, run `/policy-publish -updated-at <immutable-RFC3339-instant>`
  once with migration credentials and the desired policy settings, before rolling
  API replicas. Reusing a revision with changed metadata fails; a lower revision
  cannot overwrite shared state. API credentials retain read-only access to the
  authority, and every onboarding read resolves it from PostgreSQL.
- Keep the generated Red Hat Hardened Images for Go, the static API runtime,
  Node.js, and PostgreSQL unless a documented compatibility constraint requires
  another source. When updating a base, select an immutable catalog release,
  pin its manifest-list digest, inspect its SBOM and CVE report, and verify its
  Red Hat signature using the catalog's current instructions. Hardened Images
  are usable without a Red Hat subscription. Do not claim FIPS compliance from
  a FIPS image tag alone; validate the complete application, cryptographic
  configuration, host, and deployment boundary.
- Terminate TLS at a trusted proxy and forward only validated scheme/host data.
  Keep API, PostgreSQL, SpiceDB, OTLP, and OIDC transport encryption enabled.
  Set `HOURPATHS_TRUSTED_PROXY_CIDRS` to only that proxy network when
  client-source forwarding is required; leave it empty for direct deployments.
- Generate independent high-entropy cursor, push-token encryption, metrics,
  database, SpiceDB, and other credentials. Set
  `HOURPATHS_PUSH_TOKEN_KEY` to at least 32 bytes of dedicated secret material;
  rotating it requires a coordinated installation re-registration plan because
  existing token snapshots cannot be decrypted with a replacement key. Keep
  `HOURPATHS_PUSH_PROVIDER_ENDPOINT` on its default HTTPS Expo boundary.
  `HOURPATHS_PUSH_PROVIDER_INSECURE` exists only for controlled local acceptance
  and must remain false in production. Never deploy values from `.env.example`
  or Compose.
- Configure exact public API/web origins, CORS origins, OIDC clients and redirect
  URIs using `docs/oidc.md`.
- Size the PostgreSQL connection pool across all replicas, not per process in
  isolation. Configure request/audit limits, session lifetimes, and authorization
  retry/dead-letter policy for expected traffic.
- Set `HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES` to the positive whole
  number of minutes used for newly created ownership transfers. The documented
  default is `10080` (seven days); changing it does not alter existing transfers.
- Configure OTLP export, scrape the authenticated metrics endpoint, and alert on
  readiness, authorization dead letters, audit-write rate, database capacity,
  latency, and error probes.
- Choose and test an audit retention period. Retention is disabled by default
  and must run with its separate database role; the API runtime cannot erase
  audit events.
- Bootstrap invitation administrators before selecting invitation-only account
  provisioning, then test invitation expiry, consumption, revocation, and
  concurrent first login.
- Route liveness and readiness separately. During rolling deployment, complete
  migrations and authorization schema first, wait for readiness, and drain API
  replicas using their bounded shutdown window.
- Exercise restore procedures and a complete staging rollout even when backup
  infrastructure is managed outside this repository.

Scalar documentation can be disabled. Do so unless interactive production API
documentation is intentional and its public OIDC client is registered.


## Restricted report review

Reports are private moderation cases, not ordinary account or Path records.
Application and retention credentials cannot call the review capabilities or
read case evidence. Use a separately authenticated operational database login;
a database owner can grant that login only `EXECUTE` on
`moderation_list_cases(integer)`, `moderation_read_case(text)`, and
`moderation_set_case_state(text,text,text,text)`. Never grant these capabilities
to `app`, `app_audit_retention`, or a product user. Each read and decision records
the login's database `session_user`; use individual logins for attribution.

Review the oldest open cases with `SELECT moderation_list_cases(20)`. Open a
specific case with a bound `case_id` argument to `moderation_read_case`; the
result includes the report-time text and, for a profile photo, `pictureBase64`.
Keep this evidence in the restricted operator session rather than general logs,
issue trackers, or messages to the reported user.

Use `moderation_set_case_state(case_id, expected_state, next_state, reason)` to
move an open case to `reviewing`, or an open/reviewing case to `dismissed` with a
nonempty decision reason. A stale expected state fails instead of overwriting
another review. The functions retain the reviewer and decision history. The
reporter receives no review/outcome notification.

A separately authorized reviewer can issue a warning with
`moderation_warn_user(case_id, notice_id, policy_reason)`. Grant only its exact
`(text,text,text)` signature to that individual operator login. The function
atomically closes the open/reviewing case and creates the subject's private
Settings → Account notices entry. Repeating a closed-case action fails.

Appeal reviewers need explicit `EXECUTE` grants on
`moderation_list_appeals(integer)`, `moderation_read_appeal(text)`, and
`moderation_decide_warning_appeal(text,text,text,text)`. List pending appeals,
then read the selected notice ID and its original case in the restricted
session. Decide with `(notice_id, 'upheld' or 'reversed', decision_reason,
same_reviewer_reason)`. Use a different reviewer when practicable; the original
reviewer must record why that is unavailable. Final decisions cannot be
replaced. The subject sees the result without reviewer identity or report
contents. Pending appeals keep their case evidence until final resolution.

For a reviewed comment case, grant the operator only
`moderation_remove_comment(text,text,text)` and call it with
`(case_id, notice_id, policy_reason)`. It hides the comment and removes associated
notifications atomically with the subject notice and decision event. The private
notice identifies the original comment's posting time and reference. Ordinary
runtime credentials cannot issue or undo a removal.

Comment appeals use `moderation_decide_comment_appeal(text,text,text,text)` with
the same decision arguments and reviewer requirements as warning appeals.
Reversal lifts that restriction; it never recreates deleted content, bypasses
ordinary audience rules, or sends old notifications. A duplicate removal fails
without another notice or partial case transition.

Suspensions and bans use the image's `/account-enforce` command with a separately
controlled operational database connection. Do not grant enforcement writes to
`app`, `app_audit_retention`, or product users. This command needs the privileged
operator connection used for reviewed maintenance: it writes the decision, case,
review event, timer completion, activity derivatives and audit in one transaction.
Use an individually attributed database login; the review event records
`session_user`. Keep that credential outside API and retention workloads.

After reviewing a case, provide `--case CASE_ID --notice UNIQUE_NOTICE_ID
--action suspension --until RFC3339_INSTANT --reason-file /protected/reason.txt`.
For a ban use `--action ban` and omit `--until`. The reason file contains only the
subject-visible policy explanation. Supply database settings through the existing
configuration environment, never a command-line connection string. Reuse the
same notice ID and arguments after an uncertain result; a changed retry fails.
The operation stops the subject's server timers and saves elapsed time. It does
not stop other participants' timers. A failure rolls the transaction back.

Account appeals use `moderation_decide_account_appeal(text,text,text,text)` with
the same reviewer requirements as warning appeals. Grant only that exact
capability to an appeal reviewer. Reversal or suspension expiry restores ordinary
admission only if no other restriction remains; it never restarts timers.
Notices and appeals remain accessible with the existing application session.
Reported received nudges use `moderation_remove_nudge(text,text,text)` with
`(case_id, notice_id, policy_reason)` and the same separately granted operational
capability. The case identifies the recipient's notification. Removal hides that
nudge and suppresses queued push while preserving rate limits and retained
history. `moderation_decide_nudge_appeal(text,text,text,text)` applies the same
reviewer requirements: reversal restores only eligible retained in-app history
and never resends old push. Grant each reviewer only the required exact functions;
ordinary application and retention credentials must not receive them.
Reported social posts use `moderation_remove_feed_event(text,text,text)` with
`(case_id, notice_id, policy_reason)`. It suppresses the reported event, its
interactions and associated notifications while preserving the source recorded
activity or goal achievement. `moderation_decide_feed_event_appeal(text,text,text,text)`
uses the same reviewer and final-decision requirements. Reversal restores only
retained, currently accessible social content; it cannot recreate deleted time,
restore a separately removed comment or resend old push. Grant only these exact
capabilities to the relevant operational reviewers, never the ordinary runtime
or retention role.
Reported Path names use `moderation_remove_path_name(text,text,text,text)` with
`(case_id, notice_id, policy_reason, replacement_name)`. Supply a neutral name as
Path data; the standard “Name removed” labels are in `packages/i18n` under
`enforcement.replacementPathName` (English and Spanish). The case must still
identify the current owner and reported name. Removal preserves tracking and
membership and retires queued notifications containing the old name.
`moderation_decide_path_name_appeal(text,text,text,text)` uses the same final
reviewer rules. Reversal restores the original name only if its name revision
and owner are unchanged; later renames, transfers and deletions are preserved.
Removed names remain restricted case evidence and expire with the case.
Grant these exact capabilities only to designated operational reviewers.
Reported profile fields use
`moderation_remove_profile_fields(text,text,text,text[],text)` with
`(case_id, notice_id, policy_reason, removed_fields, replacement_name)`.
Select only `display_name`, `description`, and/or `picture`. The subject-visible
reason must name the removed fields and the policy basis, without reporter or
restricted evidence. Supply a neutral replacement name as localized profile data
when removing the display name; pass NULL otherwise. Username and account
identity are preserved. Removal refuses selected content that differs from the
report, clears removed photos from public storage and preserves profile history.
`moderation_decide_profile_fields_appeal(text,text,text,text)` follows the same
reviewer and final-decision rules. Text and picture restoration independently
preserve any subsequent edits. Grant only these exact functions to designated
reviewers; runtime and retention roles cannot invoke them or read retained fields.
A warning does not restrict the account.

The existing hourly deleted-account retention job also drains closed-case
expiry in bounded batches. It starts cleanup 89 days after closure, within the
90-day maximum; each immutable run summary includes `moderation_deleted_count`.
Open cases and recently closed cases are ineligible. Investigate failed runs
before the deadline. Case expiry removes its evidence/photo and case-specific
review history; an account-owned receipt contains only the accepted report ID
and retry hash, and is removed when that reporter deletes their account.
