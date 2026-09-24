#!/usr/bin/env bash
set -euo pipefail

fail=0
report() { printf 'structural check: %s\n' "$*" >&2; fail=1; }

for required in \
  apps/api/internal/domain/audit/event.go \
  apps/api/internal/adapters/dbmigrations/000004_create_audit_events.up.sql \
  specs/audit/audit.spec.md; do
  [[ -f "$required" ]] || report "mandatory audit primitive is missing $required"
done
grep -Fq 'DuplicateAccountRecoveryDeclines: store' apps/api/cmd/server/main.go ||
  report "server bootstrap must wire durable duplicate email recovery declines"
grep -Fq 'OnboardingActivator: sessions' apps/api/cmd/server/main.go ||
  report "server bootstrap must wire atomic onboarding activation"
grep -Fq 'PolicyAuthority: store' apps/api/cmd/server/main.go ||
  report "server bootstrap must wire the shared current policy authority"
grep -Fq 'gormstore.PolicyAuthorityHealth{Authority: store}' apps/api/cmd/server/main.go ||
  report "API readiness must fail closed until shared policy publication succeeds"
grep -Fq 'pg_advisory_xact_lock_shared(?)' apps/api/internal/adapters/gormstore/activation_repository.go ||
  report "account activation must share the policy publisher advisory lock"
if grep -Fq 'clause.Locking{Strength: "SHARE"}' apps/api/internal/adapters/gormstore/activation_repository.go; then
  report "account activation must not require policy authority UPDATE privilege for a row lock"
fi
grep -Fq 'entrypoint: [/policy-publish]' compose.yaml ||
  report "Compose must publish the shared policy authority before API startup"
app_migrate_compose="$(sed -n '/^  app-migrate:/,/^  policy-publish:/p' compose.yaml)"
grep -Fq 'depends_on: { postgres: { condition: service_healthy }, spicedb-schema: { condition: service_completed_successfully } }' <<<"$app_migrate_compose" ||
  report "application migrations must wait for the backward-compatible SpiceDB schema before publishing new authorization relationships"
grep -Fq 'depends_on: { policy-publish: { condition: service_completed_successfully }' compose.yaml ||
  report "API startup must wait for successful shared policy publication"
grep -Fq 'COPY --from=build /out/policy-publish /policy-publish' apps/api/Dockerfile ||
  report "API image must include the one-shot policy publisher"
if [[ -f apps/api/internal/adapters/dbmigrations/000004_create_audit_events.up.sql ]] &&
  ! grep -q 'BEFORE UPDATE OR DELETE ON audit_event_models' apps/api/internal/adapters/dbmigrations/000004_create_audit_events.up.sql; then
  report "audit persistence is not database-enforced append-only"
fi

if [[ ! -f .codex/agents/code-critic.toml ]] ||
  ! grep -q '^sandbox_mode = "read-only"$' .codex/agents/code-critic.toml; then
  report "project-scoped code critic must exist and remain read-only"
fi

if ! node scripts/check-i18n.mjs; then
  report "internationalization invariant failed"
fi

if ! node scripts/check-app-env.mjs; then
  report "application environment naming invariant failed"
fi

if ! python3 scripts/check_oidc_broker_contract.py; then
  report "provider-neutral OIDC broker documentation contract failed"
fi

if ! python3 scripts/check-client-api-boundary.py; then
  report "generated client API boundary failed"
fi

if ! git check-ignore -q apps/web/build/index.html; then
  report "web production output must not dirty the repository"
fi

live_acceptance="scripts/live-acceptance.sh"

grep -Fq 'latest_migration_version="$(sed -nE '\''s/^const LatestVersion uint = ([0-9]+)$/\1/p'\'' apps/api/internal/adapters/dbmigrations/migrations.go)"' "$live_acceptance" ||
  report "live migration rerun acceptance must derive the latest migration ledger from the migrator source of truth"
grep -Fq '"$ledger" == "${latest_migration_version}|f"' "$live_acceptance" ||
  report "live migration rerun acceptance must verify the mechanically derived latest migration ledger"

for policy_key in \
  HOURPATHS_CURRENT_POLICY_REVISION \
  HOURPATHS_CURRENT_TERMS_VERSION \
  HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION \
  HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION \
  HOURPATHS_TERMS_URL \
  HOURPATHS_PRIVACY_POLICY_URL \
  HOURPATHS_COMMUNITY_GUIDELINES_URL \
  HOURPATHS_SUPPORT_URL; do
  grep -Fq "$policy_key: \${$policy_key:-" compose.yaml ||
    report "Compose policy configuration must allow explicit .env overrides for $policy_key"
done
if [[ -f "$live_acceptance" ]]; then
  postgres_line="$(grep -n '^docker compose up -d postgres$' "$live_acceptance" | head -n 1 | cut -d: -f1 || true)"
  upgrade_line="$(grep -n '^run_migration_upgrade_acceptance$' "$live_acceptance" | head -n 1 | cut -d: -f1 || true)"
  adapter_line="$(grep -n '^run_postgres_adapter_acceptance$' "$live_acceptance" | head -n 1 | cut -d: -f1 || true)"
  services_line="$(grep -n '^docker compose up -d --build spicedb dex api web$' "$live_acceptance" | head -n 1 | cut -d: -f1 || true)"
  if [[ -z "$postgres_line" || -z "$upgrade_line" || -z "$adapter_line" || -z "$services_line" ]] ||
    ! (( postgres_line < upgrade_line && upgrade_line < adapter_line && adapter_line < services_line )); then
    report "migration and PostgreSQL adapter acceptance must run after PostgreSQL health and before API startup"
  fi
  grep -Fq 'docker compose run --rm --build app-migrate -target-version 14' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the frozen v14 release through the real migrator"
  grep -Fq 'docker compose run --rm app-migrate -target-version 15' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the attribution release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 16' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the resource release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 17' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the Path release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 18' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the authorization ordering release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 19' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the provisional account release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 20' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the duplicate email recovery release explicitly"
  [[ "$(grep -Fc 'docker compose run --rm app-migrate -target-version 20' "$live_acceptance")" -eq 1 ]] ||
    report "live acceptance must not ask the forward-only application migrator to downgrade to version 20"
  grep -Fq 'docker compose run --rm app-migrate -target-version 21' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the recovery decline release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 22' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the active username release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 23' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the atomic account activation release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 24' "$live_acceptance" ||
    report "migration upgrade acceptance must apply the current policy authority release explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 25' "$live_acceptance" ||
    report "migration upgrade acceptance must apply provider-email verification explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 26' "$live_acceptance" ||
    report "migration upgrade acceptance must apply Path creator membership explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 27' "$live_acceptance" ||
    report "migration upgrade acceptance must apply activity persistence explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 28' "$live_acceptance" ||
    report "migration upgrade acceptance must apply optional Path goals explicitly"
  grep -Fq 'docker compose run --rm app-migrate -target-version 31' "$live_acceptance" ||
    report "migration upgrade acceptance must apply Path archival explicitly"
  grep -Fq 'MIGRATION_PATH_ARCHIVE_SCHEMA' "$live_acceptance" ||
    report "migration upgrade acceptance must verify Path archival schema and indexes"
  grep -Fq '000031_path_archival.down.sql' "$live_acceptance" ||
    report "migration upgrade acceptance must prove Path archival rollback is fail-closed"
  grep -Fq 'docker compose run --rm app-migrate -target-version 33' "$live_acceptance" ||
    report "migration upgrade acceptance must apply push persistence explicitly"
  grep -Fq 'MIGRATION_PUSH_PERSISTENCE_SCHEMA' "$live_acceptance" ||
    report "migration upgrade acceptance must verify push persistence schema, indexes, and trigger"
  grep -Fq 'MIGRATION_PUSH_PERSISTENCE_PRIVILEGES' "$live_acceptance" ||
    report "migration upgrade acceptance must verify push persistence least privileges"
  grep -Fq '000033_push_persistence.down.sql' "$live_acceptance" ||
    report "migration upgrade acceptance must prove push persistence rollback is fail-closed"
  grep -Fq "expected clean current migration ledger 33|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean push persistence ledger"
  grep -Fq 'migration_upgrade_legacy_outbox' "$live_acceptance" ||
    report "migration upgrade acceptance must preserve a seeded legacy authorization outbox row"
  grep -Fq 'is_nullable' "$live_acceptance" ||
    report "migration upgrade acceptance must verify current attribution constraints"
  grep -Fq 'go test -count=1 ./internal/adapters/gormstore' "$live_acceptance" ||
    report "live acceptance must run the PostgreSQL GORM adapter suite"
  grep -Fq 'go test -count=1 ./internal/adapters/gormstore/path' "$live_acceptance" ||
    report "live acceptance must run the Path PostgreSQL adapter suite"
  grep -Fq 'go test -count=1 ./internal/adapters/gormstore/activity' "$live_acceptance" ||
    report "live acceptance must run the activity PostgreSQL adapter suite"
  grep -Fq -- '-migration-database-dsn "$migration_dsn"' "$live_acceptance" ||
    report "migration-ledger integration must use the separate migration role"
  grep -Fq 'go test -count=1 ./internal/adapters/ratelimit' "$live_acceptance" ||
    report "live acceptance must run the shared PostgreSQL limiter suite"
  grep -Fq 'go test -count=1 ./internal/adapters/auditretention' "$live_acceptance" ||
    report "live acceptance must run the retention-role PostgreSQL suite"
  grep -Fq "expected clean current migration ledger 17|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean Path release ledger"
  grep -Fq "expected clean current migration ledger 19|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean provisional account ledger"
  grep -Fq "expected clean current migration ledger 20|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean duplicate email recovery ledger"
  grep -Fq "expected clean current migration ledger 21|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean recovery decline ledger"
  grep -Fq "expected clean current migration ledger 22|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean active username ledger"
  grep -Fq "expected clean current migration ledger 23|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean account activation ledger"
  grep -Fq "expected clean current migration ledger 24|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean current policy authority ledger"
  grep -Fq "expected clean current migration ledger 25|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean provider-email verification ledger"
  grep -Fq "expected clean current migration ledger 26|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean Path creator membership ledger"
  grep -Fq "expected clean current migration ledger 27|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean activity persistence ledger"
  grep -Fq "expected clean current migration ledger 28|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean optional Path goals ledger"
  grep -Fq "expected clean current migration ledger 31|f" "$live_acceptance" ||
    report "migration upgrade acceptance must verify the clean Path archival ledger"
  grep -Fq 'MIGRATION_PROVISIONAL_STATUS' "$live_acceptance" ||
    report "migration upgrade acceptance must verify provisional status constraints and defaults"
  grep -Fq 'MIGRATION_DUPLICATE_EMAIL' "$live_acceptance" ||
    report "migration upgrade acceptance must accept duplicate normalized recovery email addresses"
  grep -Fq 'user_models_normalized_email_lookup_idx' "$live_acceptance" ||
    report "migration upgrade acceptance must verify the normalized email lookup index"
  grep -Fq 'MIGRATION_RECOVERY_DECLINE_PRIVILEGES' "$live_acceptance" ||
    report "migration upgrade acceptance must verify recovery decline runtime privileges"
  grep -Fq "recovery_decline_privileges\" == 't|t|f|f|f'" "$live_acceptance" ||
    report "migration upgrade acceptance must reject recovery decline update, delete, and truncate privileges"
  grep -Fq 'MIGRATION_USERNAME_SCHEMA' "$live_acceptance" ||
    report "migration upgrade acceptance must verify username nullability, constraints, and index"
  grep -Fq 'version-22 migration allowed a provisional user to reserve a username' "$live_acceptance" ||
    report "migration upgrade acceptance must reject provisional username reservations"
  grep -Fq 'version-22 migration accepted a case-only duplicate username' "$live_acceptance" ||
    report "migration upgrade acceptance must reject case-only duplicate usernames"
  grep -Fq '000022_active_usernames.down.sql' "$live_acceptance" ||
    report "migration upgrade acceptance must prove populated username rollback is guarded"
  [[ -f apps/api/internal/adapters/dbmigrations/prior-release-v21.sha256 ]] ||
    report "the exact version-21 migration release must remain frozen"
  [[ -f apps/api/internal/adapters/dbmigrations/prior-release-v22.sha256 ]] ||
    report "the exact version-22 migration release must remain frozen"
  [[ -f apps/api/internal/adapters/dbmigrations/prior-release-v23.sha256 ]] ||
    report "the exact version-23 migration release must remain frozen"
  [[ -f apps/api/internal/adapters/dbmigrations/prior-release-v24.sha256 ]] ||
    report "the exact version-24 migration release must remain frozen"
  grep -Fq 'MIGRATION_ACCOUNT_ACTIVATION_SCHEMA' "$live_acceptance" ||
    report "migration upgrade acceptance must verify atomic account activation schema"
  grep -Fq 'MIGRATION_ACCOUNT_ACTIVATION_PRIVILEGES' "$live_acceptance" ||
    report "migration upgrade acceptance must verify account activation runtime privileges"
  grep -Fq 'version-23 migration allowed an incomplete provisional account activation' "$live_acceptance" ||
    report "migration upgrade acceptance must reject incomplete account activation"
  grep -Fq '000023_atomic_account_activation.down.sql' "$live_acceptance" ||
    report "migration upgrade acceptance must prove populated account activation rollback is guarded"
  grep -Fq 'MIGRATION_CURRENT_POLICY_AUTHORITY_SCHEMA' "$live_acceptance" ||
    report "migration upgrade acceptance must verify the shared current policy authority schema"
  grep -Fq 'MIGRATION_CURRENT_POLICY_AUTHORITY_PRIVILEGES' "$live_acceptance" ||
    report "migration upgrade acceptance must verify read-only runtime policy authority privileges"
  grep -Fq 'version-24 migration allowed activation against a stale policy revision' "$live_acceptance" ||
    report "migration upgrade acceptance must reject stale policy revisions"
  grep -Fq 'version-24 migration allowed activation against stale policy versions' "$live_acceptance" ||
    report "migration upgrade acceptance must reject stale policy versions"
  grep -Fq '000024_current_policy_authority.down.sql' "$live_acceptance" ||
    report "migration upgrade acceptance must prove populated policy authority rollback is guarded"
  grep -Fq 'MIGRATION_PROVIDER_EMAIL_VERIFICATION_SCHEMA' "$live_acceptance" ||
    report "migration upgrade acceptance must verify conservative provider-email provenance"
  grep -Fq 'version-25 migration allowed verified provenance for an empty email' "$live_acceptance" ||
    report "migration upgrade acceptance must reject verified provenance for empty email"
  grep -Fq '000025_provider_email_verification.down.sql' "$live_acceptance" ||
    report "migration upgrade acceptance must prove populated provider-email provenance rollback is guarded"
  grep -Fq '_ "time/tzdata"' apps/api/internal/domain/identity/onboarding_activation.go ||
    report "account activation must embed IANA time-zone data for the static production image"
  decline_migration="apps/api/internal/adapters/dbmigrations/000021_duplicate_email_recovery_declines.up.sql"
  grep -Fq 'REVOKE UPDATE, DELETE, TRUNCATE ON duplicate_email_recovery_declines FROM app;' "$decline_migration" ||
    report "recovery decline migration must explicitly revoke runtime mutation and truncation"
  grep -Fq 'MIGRATION_PATH_PRIVILEGES' "$live_acceptance" ||
    report "migration upgrade acceptance must verify bounded Path runtime privileges"
  grep -Fq 'MIGRATION_PATH_MEMBERSHIP_PRIVILEGES' "$live_acceptance" ||
    report "migration upgrade acceptance must verify SELECT-only membership storage"
  grep -Fq "membership_privileges\" == 't|f|f|f|f'" "$live_acceptance" ||
    report "migration upgrade acceptance must reject membership write and truncate privileges"
  path_migration="apps/api/internal/adapters/dbmigrations/000017_create_paths.up.sql"
  grep -Fq 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON path_membership_models FROM app;' "$path_migration" ||
    report "Path migration must explicitly revoke runtime membership writes and truncation"
  grep -Fq 'TestPrivatePathViewPermissionMatrix|TestSeedPrivatePathHTTPAcceptanceRelationships' "$live_acceptance" ||
    report "live acceptance must exercise real private Path SpiceDB relationships"
  grep -Fq 'private-path-live-acceptance' "$live_acceptance" ||
    report "live acceptance must exercise the private Path HTTP authorization boundary"
  grep -Fq 'assert len(raw)==32' "$live_acceptance" ||
    report "live acceptance expired Path session must be a valid 32-byte opaque credential"
  grep -Fq 'expired_path_token' "$live_acceptance" ||
    report "live acceptance must reject expired application sessions at the private Path boundary"
  grep -Fq 'PATH_GOAL_CASES' "$live_acceptance" ||
    report "live Path acceptance must exercise optional goal combinations through POST, GET, and LIST"
  for goal_case in neither interval-only overall-only both; do
    grep -Fq "goal_case='$goal_case'" "$live_acceptance" ||
      report "live Path acceptance must exercise the $goal_case optional goal combination"
  done
  for recurrence in hourly daily weekly monthly yearly; do
    grep -Fq "recurrence='$recurrence'" "$live_acceptance" ||
      report "live Path acceptance must persist the default $recurrence interval alignment"
  done
  for goal_name in neither hourly overall daily weekly monthly yearly; do
    goal_get_call="assert_goal_path_get \"\$${goal_name}_path_id\" \"\$${goal_name}_expected\""
    [[ "$(grep -Fxc "$goal_get_call" "$live_acceptance")" == 1 ]] ||
      report "live Path acceptance must execute exactly one GET assertion for $goal_name"
  done
  [[ "$(grep -Fxc '# PATH_GOAL_LIST_ASSERTION verifies the complete goal-bearing Path shapes.' "$live_acceptance")" == 1 ]] ||
    report "live Path acceptance must retain the optional-goal LIST assertion marker"
  goal_list_call='curl -fsS -H "Authorization: Bearer $owner_token" '\''http://localhost:8080/v1/paths?limit=25'\'' |'
  [[ "$(grep -Fxc "$goal_list_call" "$live_acceptance")" == 1 ]] ||
    report "live Path acceptance must execute exactly one authenticated optional-goal LIST assertion"
  grep -Fq 'PATH_GOAL_PERSISTENCE_SQL' "$live_acceptance" ||
    report "live Path acceptance must prove optional goals in PostgreSQL"
  for goal_column in interval_goal_target_seconds interval_goal_recurrence interval_goal_start_minute interval_goal_start_hour interval_goal_start_weekday interval_goal_start_day interval_goal_start_month overall_target_seconds; do
    grep -Fq "$goal_column" "$live_acceptance" ||
      report "live Path acceptance must inspect persisted $goal_column values"
  done
  for role in administrator participant supporter stranger; do
    extraction="${role}_subject=\"\$(identity_subject \"\$${role}_identity_token\")\""
    binding="-v \"${role}_subject=\$${role}_subject\""
    sql_subject=":'${role}_subject', :'${role}_id'"
    user_id="${role}_id=\"\$(identity_user_id \"\$${role}_subject\")\""
    labeled_exchange="session_from_identity \"\$${role}_identity_token\" $role"
    [[ "$(grep -Fxc "$extraction" "$live_acceptance")" == 1 ]] ||
      report "live Path acceptance must derive exactly one $role subject from its OIDC token"
    grep -Fq -- "$binding" "$live_acceptance" ||
      report "live Path acceptance must bind the derived $role subject into PostgreSQL"
    grep -Fq -- "$sql_subject" "$live_acceptance" ||
      report "live Path acceptance must persist the derived $role subject for the seeded identity"
    [[ "$(grep -Fxc "$user_id" "$live_acceptance")" == 1 ]] ||
      report "live Path acceptance must derive exactly one deterministic $role user ID from its subject"
    grep -Fq -- "$labeled_exchange" "$live_acceptance" ||
      report "live Path acceptance must label the $role session exchange for actionable failures"
  done
  grep -Fq "printf '%s' \"\$1\" | python3 scripts/identity-fixture.py subject" "$live_acceptance" ||
    report "live Path acceptance must derive subjects through the tested fixture utility"
  grep -Fq 'python3 scripts/identity-fixture.py user-id "$dex_public_issuer" "$1"' "$live_acceptance" ||
    report "live Path acceptance must derive user IDs through the tested fixture utility"
  if grep -Eq "^[[:space:]]*[[:alnum:]_]+_subject=['\"]?00000000-0000-0000-0000-0000000000" "$live_acceptance"; then
    report "live Path acceptance must not assign raw provider user IDs as OIDC subjects"
  fi
else
  report "live migration upgrade acceptance harness is missing"
fi

while IFS= read -r -d '' file; do
  first="$(head -n 1 "$file")"
  lines="$(wc -l < "$file" | tr -d ' ')"
  if (( lines > 800 )) && [[ "$first" != '// Code generated '*' DO NOT EDIT.' ]]; then
    report "$file has $lines lines (maximum 800)"
  fi
  if grep -nE 'fmt\.(Print|Printf|Println)\(' "$file" >/dev/null; then
    report "$file uses ad hoc printing"
  fi
  case "$file" in
    */internal/config/*|*/cmd/*) ;;
    *) if grep -nE 'os\.(Getenv|LookupEnv)\(' "$file" >/dev/null; then report "$file reads environment outside a boundary package"; fi ;;
  esac
done < <(find apps \( -type d \( -name node_modules -o -name .svelte-kit -o -name build -o -name dist \) -prune \) -o -type f -name '*.go' -print0)

if ! python3 scripts/check-direct-sql.py; then
  report "direct SQL allowlist invariant failed"
fi

if find . \( -type d \( -name .git -o -name node_modules -o -name .svelte-kit -o -name build -o -name dist \) -prune \) -o -type f \( -iname '*mock*.go' -o -iname '*mock*.ts' -o -iname '*mock*.tsx' \) -print | grep -q .; then
  report "mock files are forbidden; use behavioral fakes"
fi

if grep -RInE '^[[:space:]]*uses:[[:space:]]+[^#[:space:]]+@(v[0-9]+|main|master)([[:space:]]|$)' .github/workflows >/dev/null 2>&1; then
  report "CI actions must use immutable commit SHAs"
fi

while IFS= read -r line; do
  [[ "$line" == *'@sha256:'* ]] || report "Compose image is not digest-pinned: $line"
done < <(grep -hE '^[[:space:]]+image:[[:space:]]+' compose*.yaml 2>/dev/null || true)

while IFS= read -r line; do
  [[ "$line" == *'@sha256:'* ]] || report "Dockerfile base is not digest-pinned: $line"
done < <(grep -hE '^FROM[[:space:]]+' apps/*/Dockerfile 2>/dev/null || true)

exit "$fail"
