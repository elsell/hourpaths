#!/usr/bin/env bash
set -euo pipefail
app_dir="${1:?generated app directory is required}"
cd "$app_dir"
root="$(pwd)"
export COMPOSE_FILE="$root/compose.yaml:$root/compose.acceptance.yaml"
export COMPOSE_PROFILES=acceptance
# Fixed host ports belong to the Docker host, so the lock must live in a
# host-global location rather than a runner-specific TMPDIR.
lock_dir="/tmp/hourpaths-live-acceptance.lock"
for _ in $(seq 1 1200); do
  if mkdir "$lock_dir" 2>/dev/null; then printf '%s\n' "$$" > "$lock_dir/pid"; acquired=1; break; fi
  lock_pid="$(cat "$lock_dir/pid" 2>/dev/null || true)"
  if [[ -n "$lock_pid" ]] && ! kill -0 "$lock_pid" 2>/dev/null; then rm -rf "$lock_dir"; continue; fi
  sleep 1
done
[[ -n "${acquired:-}" ]] || { echo "timed out waiting for the live acceptance port lock" >&2; exit 1; }
export MAKE_APP_UID="${MAKE_APP_UID:-$(id -u)}" MAKE_APP_GID="${MAKE_APP_GID:-$(id -g)}"
export COMPOSE_PROJECT_NAME="make-app-acceptance-${MAKE_APP_ACCEPTANCE_RUN_ID:-$$}"
export HOURPATHS_ACCOUNT_PROVISIONING_MODE=existing
export HOURPATHS_ACCOUNT_INVITED_EMAILS=developer@example.com
api_host_port="${HOURPATHS_API_HOST_PORT:-28080}"
postgres_host_port="${HOURPATHS_POSTGRES_HOST_PORT:-25432}"
spicedb_host_port="${HOURPATHS_SPICEDB_HOST_PORT:-25051}"
dex_host_port="${HOURPATHS_OIDC_HOST_PORT:-${HOURPATHS_DEX_HOST_PORT:-25556}}"
web_host_port="${HOURPATHS_WEB_HOST_PORT:-25173}"
for port_name in api_host_port postgres_host_port spicedb_host_port dex_host_port web_host_port; do
  port_value="${!port_name}"
  if [[ ! "$port_value" =~ ^[1-9][0-9]{0,4}$ ]] || (( 10#$port_value > 65535 )); then
    echo "$port_name must be a canonical TCP port from 1 through 65535" >&2
    exit 1
  fi
done
api_base_url="http://localhost:${api_host_port}"
web_base_url="${HOURPATHS_WEB_PUBLIC_BASE_URL:-http://localhost:${web_host_port}}"
dex_public_issuer="http://localhost:${dex_host_port}/dex"
dex_config_path="$(mktemp /tmp/hourpaths-dex-config.XXXXXX.yaml)"
sed "s|^issuer: .*$|issuer: ${dex_public_issuer}|" deploy/dex/config.yaml >"$dex_config_path"
sed -i.bak "s|http://localhost:8080|${api_base_url}|g" "$dex_config_path"
sed -i.bak "s|http://localhost:5173|${web_base_url}|g" "$dex_config_path"
rm -f "${dex_config_path}.bak"
chmod 0644 "$dex_config_path"
export HOURPATHS_API_HOST_PORT="$api_host_port"
export HOURPATHS_API_PUBLIC_BASE_URL="$api_base_url"
export HOURPATHS_WEB_HOST_PORT="$web_host_port"
export HOURPATHS_WEB_PUBLIC_BASE_URL="$web_base_url"
export HOURPATHS_POSTGRES_HOST_PORT="$postgres_host_port"
export HOURPATHS_TERMS_URL="$web_base_url/legal/terms"
export HOURPATHS_PRIVACY_POLICY_URL="$web_base_url/legal/privacy"
export HOURPATHS_COMMUNITY_GUIDELINES_URL="$web_base_url/community-guidelines"
export HOURPATHS_SUPPORT_URL="$web_base_url/support"
export HOURPATHS_SPICEDB_HOST_PORT="$spicedb_host_port"
export HOURPATHS_DEX_HOST_PORT="$dex_host_port"
export HOURPATHS_DEX_PUBLIC_ISSUER="$dex_public_issuer"
export HOURPATHS_DEX_CONFIG_PATH="$dex_config_path"
push_provider_log="$(mktemp /tmp/hourpaths-push-provider.XXXXXX.jsonl)"
export HOURPATHS_PUSH_PROVIDER_LOG_PATH="$push_provider_log"
export HOURPATHS_PUSH_PROVIDER_ENDPOINT="http://push-provider:19090"
export HOURPATHS_PUSH_PROVIDER_INSECURE=true
web_config_container=""
cleanup() { status=$?; if [[ -n "$web_config_container" ]]; then docker rm --force "$web_config_container" >/dev/null 2>&1 || true; fi; if [[ "$status" -ne 0 ]]; then docker compose ps -a >&2 || true; docker compose logs --tail=200 >&2 || true; fi; docker compose down --volumes --remove-orphans --rmi local >/dev/null 2>&1 || true; rm -f "$dex_config_path" "$push_provider_log"; rm -rf "$lock_dir"; return "$status"; }
trap cleanup EXIT

# Keep the long acceptance scenario readable while routing its reviewed API
# requests to the isolated host port.
curl() {
  local argument rewritten=()
  for argument in "$@"; do
    argument="${argument//http:\/\/localhost:8080/${api_base_url}}"
    rewritten+=("${argument//http:\/\/localhost:5173/${web_base_url}}")
  done
  command curl "${rewritten[@]}"
}

wait_for_push() {
  local notification_id="$1" match
  for _ in $(seq 1 200); do
    match="$(PUSH_NOTIFICATION_ID="$notification_id" python3 -c \
      'import json,os,sys;p=os.environ["PUSH_NOTIFICATION_ID"];lines=open(sys.argv[1],encoding="utf-8").readlines();rows=[json.loads(line) for line in lines if line.endswith("\n") and line.strip()];matches=[row for row in rows if row.get("data",{}).get("notificationId")==p];print(json.dumps(matches[-1],separators=(",",":")) if matches else "")' \
      "$push_provider_log")"
    if [[ -n "$match" ]]; then
      printf '%s' "$match"
      return 0
    fi
    sleep 0.1
  done
  echo "timed out waiting for push $notification_id" >&2
  docker compose exec -T postgres psql -At -F '|' -U app -d app \
    -v "notification_id=$notification_id" <<'PUSH_TIMEOUT_DIAGNOSTICS' >&2 || true
SELECT
  CURRENT_TIMESTAMP,
  d.notification_id,
  d.installation_id,
  d.recipient_user_id,
  d.available_at,
  d.created_at,
  d.attempts,
  COALESCE(d.locked_by, ''),
  COALESCE(d.locked_until::text, ''),
  COALESCE(d.delivered_at::text, ''),
  COALESCE(d.suppressed_at::text, ''),
  COALESCE(d.permanently_failed_at::text, ''),
  COALESCE(d.failure_code, ''),
  (i.id IS NOT NULL),
  COALESCE(i.owner_user_id, ''),
  (i.deleted_at IS NULL)
FROM notification_push_delivery_models d
LEFT JOIN push_installation_models i ON i.id = d.installation_id
WHERE d.notification_id = :'notification_id';
PUSH_TIMEOUT_DIAGNOSTICS
  docker compose logs --tail=100 api >&2 || true
  return 1
}

run_migration_upgrade_acceptance() {
  local ledger columns resource_columns runtime_privileges path_columns membership_columns path_privileges membership_privileges ordering_objects email_index recovery_decline_columns recovery_decline_privileges username_schema activation_schema activation_privileges provider_email_schema activity_schema activity_privileges goal_schema path_archive_schema path_invitation_schema path_invitation_privileges provider_email_before_rerun provider_email_after_rerun before_rerun after_rerun duplicate_before_rerun duplicate_after_rerun decline_before_rerun decline_after_rerun username_before_rerun username_after_rerun activation_before_rerun activation_after_rerun latest_migration_version
  latest_migration_version="$(sed -nE 's/^const LatestVersion uint = ([0-9]+)$/\1/p' apps/api/internal/adapters/dbmigrations/migrations.go)"
  [[ -n "$latest_migration_version" ]] || { echo "could not resolve latest migration version" >&2; return 1; }

  docker compose run --rm --build app-migrate -target-version 14
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '14|f' ]] || { echo "expected clean prior-release migration ledger 14|f, got $ledger" >&2; return 1; }

  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_UPGRADE_SEED'
INSERT INTO user_models (id, email, display_name, created_at, updated_at)
VALUES (
  'migration_upgrade_user',
  'migration-upgrade@example.com',
  'Migration Upgrade',
  '2026-01-01 00:00:00+00',
  '2026-01-02 00:00:00+00'
);
INSERT INTO authorization_outbox_models (
  id,
  resource_type,
  resource_id,
  relation,
  subject_type,
  subject_id,
  operation,
  attempts,
  completed_at,
  created_at,
  failure_code
)
VALUES (
  'migration_upgrade_legacy_outbox',
  'user',
  'migration_upgrade_user',
  'owner',
  'user',
  'migration_upgrade_user',
  'touch',
  0,
  '2026-01-03 00:00:00+00',
  '2026-01-03 00:00:00+00',
  ''
);
MIGRATION_UPGRADE_SEED

  docker compose run --rm app-migrate -target-version 15
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '15|f' ]] || { echo "expected clean frozen prior-release migration ledger 15|f, got $ledger" >&2; return 1; }

  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_UPGRADE_ASSERT'
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM user_models
    WHERE id = 'migration_upgrade_user'
      AND email = 'migration-upgrade@example.com'
      AND display_name = 'Migration Upgrade'
      AND created_at = '2026-01-01 00:00:00+00'
      AND updated_at = '2026-01-02 00:00:00+00'
  ) THEN
    RAISE EXCEPTION 'migration upgrade did not preserve the baseline user';
  END IF;
  IF NOT EXISTS (
    SELECT 1
    FROM authorization_outbox_models
    WHERE id = 'migration_upgrade_legacy_outbox'
      AND resource_type = 'user'
      AND resource_id = 'migration_upgrade_user'
      AND relation = 'owner'
      AND subject_type = 'user'
      AND subject_id = 'migration_upgrade_user'
      AND operation = 'touch'
      AND attempts = 0
      AND completed_at = '2026-01-03 00:00:00+00'
      AND created_at = '2026-01-03 00:00:00+00'
      AND failure_code = ''
      AND owner_user_id = 'migration_upgrade_user'
      AND actor_user_id = 'migration_upgrade_user'
  ) THEN
    RAISE EXCEPTION 'migration upgrade did not preserve and backfill the legacy outbox row';
  END IF;
END $$;
MIGRATION_UPGRADE_ASSERT

  columns="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_UPGRADE_COLUMNS'
SELECT column_name, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'authorization_outbox_models'
  AND column_name IN ('actor_user_id', 'owner_user_id')
ORDER BY column_name;
MIGRATION_UPGRADE_COLUMNS
)"
  [[ "$columns" == $'actor_user_id|NO\nowner_user_id|NO' ]] || {
    echo "expected non-null owner/actor attribution columns, got: $columns" >&2
    return 1
  }

  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO authorization_outbox_models (id, resource_type, resource_id, relation, subject_type, subject_id, operation, attempts, created_at, failure_code, owner_user_id) VALUES ('migration_upgrade_missing_actor', 'user', 'migration_upgrade_user', 'owner', 'user', 'migration_upgrade_user', 'touch', 0, CURRENT_TIMESTAMP, '', 'migration_upgrade_user')"; then
    echo "current migration accepted an outbox row without actor attribution" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO authorization_outbox_models (id, resource_type, resource_id, relation, subject_type, subject_id, operation, attempts, created_at, failure_code, actor_user_id) VALUES ('migration_upgrade_missing_owner', 'user', 'migration_upgrade_user', 'owner', 'user', 'migration_upgrade_user', 'touch', 0, CURRENT_TIMESTAMP, '', 'migration_upgrade_user')"; then
    echo "current migration accepted an outbox row without owner attribution" >&2
    return 1
  fi

  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT to_regclass('public.resource_models') IS NULL")" == 't' ]] || {
    echo "prior-release migration set unexpectedly contains resource persistence" >&2
    return 1
  }
  docker compose run --rm app-migrate -target-version 16
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '16|f' ]] || { echo "expected clean current migration ledger 16|f, got $ledger" >&2; return 1; }
  resource_columns="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_RESOURCE_COLUMNS'
SELECT column_name, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'resource_models'
ORDER BY ordinal_position;
MIGRATION_RESOURCE_COLUMNS
)"
  [[ "$resource_columns" == $'id|NO\ndomain|NO\nowner_user_id|NO\nname|NO\ncreated_at|NO' ]] || {
    echo "expected complete resource persistence columns, got: $resource_columns" >&2
    return 1
  }
  runtime_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_RESOURCE_PRIVILEGES'
SELECT
  has_table_privilege('app', 'resource_models', 'SELECT'),
  has_table_privilege('app', 'resource_models', 'INSERT'),
  has_table_privilege('app', 'resource_models', 'UPDATE'),
  has_table_privilege('app', 'resource_models', 'DELETE'),
  has_table_privilege('app', 'resource_models', 'TRUNCATE');
MIGRATION_RESOURCE_PRIVILEGES
)"
  [[ "$runtime_privileges" == 't|t|t|t|f' ]] || {
    echo "runtime resource privileges are not bounded DML: $runtime_privileges" >&2
    return 1
  }

  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT to_regclass('public.path_models') IS NULL, to_regclass('public.path_membership_models') IS NULL")" == 't|t' ]] || {
    echo "version-16 prior release unexpectedly contains Path persistence" >&2
    return 1
  }
  docker compose run --rm app-migrate -target-version 17
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '17|f' ]] || { echo "expected clean current migration ledger 17|f, got $ledger" >&2; return 1; }
  path_columns="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_COLUMNS'
SELECT column_name, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'path_models'
ORDER BY ordinal_position;
MIGRATION_PATH_COLUMNS
)"
  [[ "$path_columns" == $'id|NO\nowner_user_id|NO\nname|NO\nvisibility|NO\ncreated_at|NO\nupdated_at|NO' ]] || {
    echo "expected complete Path persistence columns, got: $path_columns" >&2
    return 1
  }
  membership_columns="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_MEMBERSHIP_COLUMNS'
SELECT column_name, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'path_membership_models'
ORDER BY ordinal_position;
MIGRATION_PATH_MEMBERSHIP_COLUMNS
)"
  [[ "$membership_columns" == $'path_id|NO\nuser_id|NO\nrole|NO' ]] || {
    echo "expected complete Path membership columns, got: $membership_columns" >&2
    return 1
  }
  path_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_PRIVILEGES'
SELECT
  has_table_privilege('app', 'path_models', 'SELECT'),
  has_table_privilege('app', 'path_models', 'INSERT'),
  has_table_privilege('app', 'path_models', 'UPDATE'),
  has_table_privilege('app', 'path_models', 'DELETE'),
  has_table_privilege('app', 'path_models', 'TRUNCATE');
MIGRATION_PATH_PRIVILEGES
)"
  [[ "$path_privileges" == 't|t|t|t|f' ]] || {
    echo "runtime Path privileges are not bounded DML: $path_privileges" >&2
    return 1
  }
  membership_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_MEMBERSHIP_PRIVILEGES'
SELECT
  has_table_privilege('app', 'path_membership_models', 'SELECT'),
  has_table_privilege('app', 'path_membership_models', 'INSERT'),
  has_table_privilege('app', 'path_membership_models', 'UPDATE'),
  has_table_privilege('app', 'path_membership_models', 'DELETE'),
  has_table_privilege('app', 'path_membership_models', 'TRUNCATE');
MIGRATION_PATH_MEMBERSHIP_PRIVILEGES
)"
  [[ "$membership_privileges" == 't|f|f|f|f' ]] || {
    echo "runtime Path membership storage is not SELECT-only: $membership_privileges" >&2
    return 1
  }

  ordering_objects="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ORDERING_ABSENT'
SELECT
  to_regprocedure('public.order_authorization_outbox_change()') IS NULL,
  to_regclass('public.authorization_outbox_owner_dead_letter_idx') IS NULL,
  to_regclass('public.authorization_outbox_resource_created_idx') IS NULL;
MIGRATION_ORDERING_ABSENT
)"
  [[ "$ordering_objects" == 't|t|t' ]] || {
    echo "version-17 prior release unexpectedly contains authorization ordering objects: $ordering_objects" >&2
    return 1
  }

  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PENDING_ORDER_FIXTURE'
INSERT INTO authorization_outbox_models (
  id, resource_type, resource_id, relation, subject_type, subject_id, operation,
  attempts, created_at, failure_code, owner_user_id, actor_user_id
)
VALUES
  (
    'migration_upgrade_pending_touch', 'resource', 'path/migration-upgrade-order',
    'owner', 'user', 'migration_upgrade_user', 'touch', 0,
    '2026-01-05 01:00:00+00', '', 'migration_upgrade_user',
    'migration_upgrade_user'
  ),
  (
    'migration_upgrade_pending_delete', 'resource', 'path/migration-upgrade-order',
    'owner', 'user', 'migration_upgrade_user', 'delete', 0,
    '2026-01-04 23:00:00+00', '', 'migration_upgrade_user',
    'migration_upgrade_user'
  );
MIGRATION_PENDING_ORDER_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000018_authorization_outbox_ordering.up.sql; then
    echo "version-18 SQL accepted authorization work whose producer order cannot be recovered" >&2
    return 1
  fi
  if docker compose run --rm app-migrate -target-version 18; then
    echo "version-18 migration accepted authorization work whose producer order cannot be recovered" >&2
    return 1
  fi
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '17|f' ]] || {
    echo "authorization drain preflight dirtied the version-17 migration ledger: $ledger" >&2
    return 1
  }
  ordering_objects="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ORDERING_STILL_ABSENT'
SELECT
  to_regprocedure('public.order_authorization_outbox_change()') IS NULL,
  to_regclass('public.authorization_outbox_owner_dead_letter_idx') IS NULL,
  to_regclass('public.authorization_outbox_resource_created_idx') IS NULL;
MIGRATION_ORDERING_STILL_ABSENT
)"
  [[ "$ordering_objects" == 't|t|t' ]] || {
    echo "refused version-18 migration partially created authorization ordering objects: $ordering_objects" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE authorization_outbox_models SET completed_at = '2026-01-05 02:00:00+00' WHERE id IN ('migration_upgrade_pending_touch', 'migration_upgrade_pending_delete')"

  docker compose run --rm app-migrate -target-version 18
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '18|f' ]] || { echo "expected clean current migration ledger 18|f, got $ledger" >&2; return 1; }
  ordering_objects="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ORDERING_PRESENT'
SELECT
  to_regclass('public.authorization_resource_lock_models') IS NOT NULL,
  to_regprocedure('public.order_authorization_outbox_change()') IS NOT NULL,
  to_regclass('public.authorization_outbox_owner_dead_letter_idx') IS NOT NULL,
  to_regclass('public.authorization_outbox_resource_created_idx') IS NOT NULL,
  EXISTS (
    SELECT 1
    FROM pg_catalog.pg_trigger
    WHERE tgname = 'authorization_outbox_ordering'
      AND tgrelid = 'public.authorization_outbox_models'::regclass
      AND NOT tgisinternal
  );
MIGRATION_ORDERING_PRESENT
)"
  [[ "$ordering_objects" == 't|t|t|t|t' ]] || {
    echo "current migration is missing authorization ordering objects: $ordering_objects" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 19
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '19|f' ]] || { echo "expected clean current migration ledger 19|f, got $ledger" >&2; return 1; }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PROVISIONAL_STATUS'
INSERT INTO user_models (id, email, display_name, created_at, updated_at)
VALUES (
  'migration_upgrade_provisional',
  'shared-recovery@example.com',
  'Provisional Upgrade',
  '2026-01-06 00:00:00+00',
  '2026-01-06 00:00:00+00'
);
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM user_models
    WHERE id = 'migration_upgrade_provisional'
      AND status = 'provisional'
  ) THEN
    RAISE EXCEPTION 'version-19 migration did not apply the provisional status default';
  END IF;
END $$;
MIGRATION_PROVISIONAL_STATUS
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO user_models (id, status, created_at, updated_at) VALUES ('migration_upgrade_invalid_status', 'unknown', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"; then
    echo "version-19 migration accepted an unknown user status" >&2
    return 1
  fi

  docker compose run --rm app-migrate -target-version 20
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '20|f' ]] || { echo "expected clean current migration ledger 20|f, got $ledger" >&2; return 1; }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_DUPLICATE_EMAIL'
INSERT INTO user_models (id, email, display_name, created_at, updated_at)
VALUES (
  'migration_upgrade_duplicate_email',
  'SHARED-RECOVERY@EXAMPLE.COM',
  'Duplicate Recovery Email',
  '2026-01-07 00:00:00+00',
  '2026-01-07 00:00:00+00'
);
DO $$
BEGIN
  IF (SELECT count(*) FROM user_models WHERE lower(email) = 'shared-recovery@example.com') <> 2 THEN
    RAISE EXCEPTION 'version-20 migration did not allow duplicate normalized recovery emails';
  END IF;
END $$;
MIGRATION_DUPLICATE_EMAIL
  email_index="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_EMAIL_LOOKUP_INDEX'
SELECT
  indexrelid::regclass::text,
  NOT indisunique
FROM pg_catalog.pg_index
WHERE indexrelid = 'public.user_models_normalized_email_lookup_idx'::regclass;
MIGRATION_EMAIL_LOOKUP_INDEX
)"
  [[ "$email_index" == 'user_models_normalized_email_lookup_idx|t' ]] || {
    echo "version-20 migration is missing the nonunique normalized email lookup index: $email_index" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 21
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '21|f' ]] || { echo "expected clean current migration ledger 21|f, got $ledger" >&2; return 1; }
  recovery_decline_columns="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_RECOVERY_DECLINE_COLUMNS'
SELECT column_name, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'duplicate_email_recovery_declines'
ORDER BY ordinal_position;
MIGRATION_RECOVERY_DECLINE_COLUMNS
)"
  [[ "$recovery_decline_columns" == $'provisional_user_id|NO\nnormalized_email|NO\ncreated_at|NO' ]] || {
    echo "expected complete recovery decline persistence columns, got: $recovery_decline_columns" >&2
    return 1
  }
  recovery_decline_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_RECOVERY_DECLINE_PRIVILEGES'
SELECT
  has_table_privilege('app', 'duplicate_email_recovery_declines', 'SELECT'),
  has_table_privilege('app', 'duplicate_email_recovery_declines', 'INSERT'),
  has_table_privilege('app', 'duplicate_email_recovery_declines', 'UPDATE'),
  has_table_privilege('app', 'duplicate_email_recovery_declines', 'DELETE'),
  has_table_privilege('app', 'duplicate_email_recovery_declines', 'TRUNCATE');
MIGRATION_RECOVERY_DECLINE_PRIVILEGES
)"
  [[ "$recovery_decline_privileges" == 't|t|f|f|f' ]] || {
    echo "runtime recovery decline privileges are not append-only: $recovery_decline_privileges" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_RECOVERY_DECLINE_FIXTURE'
INSERT INTO duplicate_email_recovery_declines (provisional_user_id, normalized_email, created_at)
VALUES ('migration_upgrade_provisional', 'shared-recovery@example.com', '2026-01-08 00:00:00+00');
MIGRATION_RECOVERY_DECLINE_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO duplicate_email_recovery_declines (provisional_user_id, normalized_email, created_at) VALUES ('migration_upgrade_provisional', 'NOT-NORMALIZED@example.com', CURRENT_TIMESTAMP)"; then
    echo "version-21 migration accepted a non-normalized recovery decline email" >&2
    return 1
  fi

  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'user_models' AND column_name = 'username'")" == '0' ]] || {
    echo "version-21 prior release unexpectedly contains username persistence" >&2
    return 1
  }
  docker compose run --rm app-migrate -target-version 22
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '22|f' ]] || { echo "expected clean current migration ledger 22|f, got $ledger" >&2; return 1; }
  username_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_USERNAME_SCHEMA'
SELECT
  (SELECT is_nullable
   FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'user_models' AND column_name = 'username'),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_constraint
    WHERE conrelid = 'public.user_models'::regclass
      AND conname = 'user_models_provisional_username_check'
      AND convalidated
  ),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_constraint
    WHERE conrelid = 'public.user_models'::regclass
      AND conname = 'user_models_username_format_check'
      AND convalidated
  ),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_index
    WHERE indexrelid = 'public.user_models_normalized_username_unique_idx'::regclass
      AND indisunique
      AND position('translate(username' in pg_get_indexdef(indexrelid)) > 0
      AND pg_get_expr(indpred, indrelid) = '(username IS NOT NULL)'
  );
MIGRATION_USERNAME_SCHEMA
)"
  [[ "$username_schema" == 'YES|t|t|t' ]] || {
    echo "version-22 migration is missing nullable username constraints or its case-insensitive partial unique index: $username_schema" >&2
    return 1
  }
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE user_models SET username = 'provisional_user' WHERE id = 'migration_upgrade_provisional'"; then
    echo "version-22 migration allowed a provisional user to reserve a username" >&2
    return 1
  fi
  for invalid_username in ab "$(printf 'a%.0s' $(seq 1 65))" 'not-valid' 'not valid' 'naïve'; do
    if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
      -v "username=$invalid_username" -c "UPDATE user_models SET status = 'active', username = :'username' WHERE id = 'migration_upgrade_duplicate_email'"; then
      echo "version-22 migration accepted invalid username: $invalid_username" >&2
      return 1
    fi
  done
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE user_models SET status = 'active', username = 'Identity.Paths_1' WHERE id = 'migration_upgrade_duplicate_email'"
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE user_models SET status = 'active', username = 'identity.paths_1' WHERE id = 'migration_upgrade_user'"; then
    echo "version-22 migration accepted a case-only duplicate username" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000022_active_usernames.down.sql; then
    echo "version-22 down migration removed populated username data" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT username FROM user_models WHERE id = 'migration_upgrade_duplicate_email'")" == 'Identity.Paths_1' ]] || {
    echo "refused version-22 rollback changed the populated username" >&2
    return 1
  }

  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT status, username IS NULL FROM user_models WHERE id = 'migration_upgrade_user'")" == 'active|t' ]] || {
    echo "version-22 fixture no longer proves compatibility with a legacy active account lacking a username" >&2
    return 1
  }
  docker compose run --rm app-migrate -target-version 23
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '23|f' ]] || { echo "expected clean current migration ledger 23|f, got $ledger" >&2; return 1; }
  activation_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ACCOUNT_ACTIVATION_SCHEMA'
SELECT
  (SELECT is_nullable
   FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'user_models' AND column_name = 'profile_visibility'),
  to_regclass('public.user_account_activation_models') IS NOT NULL,
  to_regclass('public.user_policy_acceptance_models') IS NOT NULL,
  to_regclass('public.user_preference_models') IS NOT NULL,
  to_regclass('public.user_time_zone_history_models') IS NOT NULL,
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_trigger
    WHERE tgrelid = 'public.user_models'::regclass
      AND tgname = 'user_models_complete_account_activation'
      AND NOT tgisinternal
  ),
  (SELECT count(*) = 4
   FROM pg_catalog.pg_trigger
   WHERE tgname IN (
       'user_account_activation_owner_active',
       'user_policy_acceptance_owner_active',
       'user_preference_owner_active',
       'user_time_zone_history_owner_active'
     )
     AND tgdeferrable AND tginitdeferred
     AND NOT tgisinternal);
MIGRATION_ACCOUNT_ACTIVATION_SCHEMA
)"
  [[ "$activation_schema" == 'YES|t|t|t|t|t|t' ]] || {
    echo "version-23 migration is missing account activation schema or transaction constraints: $activation_schema" >&2
    return 1
  }
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT status, username IS NULL, profile_visibility IS NULL FROM user_models WHERE id = 'migration_upgrade_user'")" == 'active|t|t' ]] || {
    echo "version-23 migration invalidated the existing active account fixture" >&2
    return 1
  }
  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM user_account_activation_models WHERE user_id = 'migration_upgrade_user'")" == '0' ]] || {
    echo "version-23 migration invented activation evidence for an existing active account" >&2
    return 1
  }
  activation_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ACCOUNT_ACTIVATION_PRIVILEGES'
SELECT table_name,
  has_table_privilege('app', table_name, 'SELECT'),
  has_table_privilege('app', table_name, 'INSERT'),
  has_table_privilege('app', table_name, 'UPDATE'),
  has_table_privilege('app', table_name, 'DELETE'),
  has_table_privilege('app', table_name, 'TRUNCATE')
FROM (VALUES
  ('user_account_activation_models'),
  ('user_policy_acceptance_models'),
  ('user_preference_models'),
  ('user_time_zone_history_models')
) AS activation_tables(table_name)
ORDER BY table_name;
MIGRATION_ACCOUNT_ACTIVATION_PRIVILEGES
)"
  [[ "$activation_privileges" == $'user_account_activation_models|t|t|f|f|f\nuser_policy_acceptance_models|t|t|f|f|f\nuser_preference_models|t|t|f|f|f\nuser_time_zone_history_models|t|t|f|f|f' ]] || {
    echo "runtime account activation privileges are not least privilege: $activation_privileges" >&2
    return 1
  }
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at) VALUES ('migration_upgrade_provisional', 16, '2026-01-09 00:00:00+00', '2026-01-09 00:00:00+00')"; then
    echo "version-23 migration allowed activation evidence to commit for a provisional owner" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at) VALUES ('migration_upgrade_provisional', 'terms', 'partial-v1', 'accepted', '2026-01-09 00:00:00+00')"; then
    echo "version-23 migration allowed policy acceptance to commit for a provisional owner" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at) VALUES ('migration_upgrade_provisional', 1, 'America/New_York', '2026-01-09 00:00:00+00', '2026-01-09 00:00:00+00')"; then
    echo "version-23 migration allowed preferences to commit for a provisional owner" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone) VALUES ('migration_upgrade_provisional', '2026-01-09 00:00:00+00', 'America/New_York')"; then
    echo "version-23 migration allowed time-zone history to commit for a provisional owner" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE user_models SET status = 'active', username = 'migration.provisional', profile_visibility = 'private' WHERE id = 'migration_upgrade_provisional'"; then
    echo "version-23 migration allowed an incomplete provisional account activation" >&2
    return 1
  fi
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_ACCOUNT_ACTIVATION_FIXTURE'
BEGIN;
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at)
VALUES ('migration_upgrade_provisional', 16, '2026-01-09 00:00:00+00', '2026-01-09 00:00:00+00');
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at) VALUES
  ('migration_upgrade_provisional', 'terms', 'migration-v1', 'accepted', '2026-01-09 00:00:00+00'),
  ('migration_upgrade_provisional', 'privacy', 'migration-v1', 'acknowledged', '2026-01-09 00:00:00+00'),
  ('migration_upgrade_provisional', 'community_guidelines', 'migration-v1', 'accepted', '2026-01-09 00:00:00+00');
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at)
VALUES ('migration_upgrade_provisional', 1, 'America/New_York', '2026-01-09 00:00:00+00', '2026-01-09 00:00:00+00');
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone)
VALUES ('migration_upgrade_provisional', '2026-01-09 00:00:00+00', 'America/New_York');
UPDATE user_models
SET status = 'active', username = 'migration.provisional', display_name = 'Migration Provisional', profile_visibility = 'private'
WHERE id = 'migration_upgrade_provisional';
COMMIT;
MIGRATION_ACCOUNT_ACTIVATION_FIXTURE
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT status, username, profile_visibility FROM user_models WHERE id = 'migration_upgrade_provisional'")" == 'active|migration.provisional|private' ]] || {
    echo "version-23 migration rejected a complete atomic account activation fixture" >&2
    return 1
  }
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d app -c \
    "UPDATE user_account_activation_models SET completed_at = completed_at + interval '1 second' WHERE user_id = 'migration_upgrade_provisional'"; then
    echo "runtime role mutated immutable account activation evidence" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d app -c \
    "UPDATE user_preference_models SET current_time_zone = 'Europe/London' WHERE user_id = 'migration_upgrade_provisional'"; then
    echo "runtime role changed preferences without effective-dated history" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000023_atomic_account_activation.down.sql; then
    echo "version-23 down migration removed populated account activation data" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM user_account_activation_models WHERE user_id = 'migration_upgrade_provisional'")" == '1' ]] || {
    echo "refused version-23 rollback changed populated account activation data" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 24
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '24|f' ]] || { echo "expected clean current migration ledger 24|f, got $ledger" >&2; return 1; }
  policy_authority_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_CURRENT_POLICY_AUTHORITY_SCHEMA'
SELECT
  to_regclass('public.current_policy_set_models') IS NOT NULL,
  (SELECT is_nullable
   FROM information_schema.columns
   WHERE table_schema = 'public'
     AND table_name = 'user_account_activation_models'
     AND column_name = 'policy_set_revision'),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_constraint
    WHERE conrelid = 'public.current_policy_set_models'::regclass
      AND conname = 'current_policy_set_models_pkey'
      AND convalidated
  ),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_trigger
    WHERE tgrelid = 'public.user_models'::regclass
      AND tgname = 'user_models_complete_account_activation'
      AND NOT tgisinternal
      AND position('current_policy_set_models' in pg_get_functiondef(tgfoid)) > 0
  );
MIGRATION_CURRENT_POLICY_AUTHORITY_SCHEMA
)"
  [[ "$policy_authority_schema" == 't|YES|t|t' ]] || {
    echo "version-24 migration is missing the current policy authority or activation linkage: $policy_authority_schema" >&2
    return 1
  }
  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c 'SELECT count(*) FROM current_policy_set_models')" == '0' ]] || {
    echo "version-24 migration invented a current policy authority" >&2
    return 1
  }
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT status, policy_set_revision IS NULL FROM user_models JOIN user_account_activation_models ON user_account_activation_models.user_id = user_models.id WHERE user_models.id = 'migration_upgrade_provisional'")" == 'active|t' ]] || {
    echo "version-24 migration invalidated or invented revision evidence for the existing active fixture" >&2
    return 1
  }
  policy_authority_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_CURRENT_POLICY_AUTHORITY_PRIVILEGES'
SELECT
  has_table_privilege('app', 'current_policy_set_models', 'SELECT'),
  has_table_privilege('app', 'current_policy_set_models', 'INSERT'),
  has_table_privilege('app', 'current_policy_set_models', 'UPDATE'),
  has_table_privilege('app', 'current_policy_set_models', 'DELETE'),
  has_table_privilege('app', 'current_policy_set_models', 'TRUNCATE');
MIGRATION_CURRENT_POLICY_AUTHORITY_PRIVILEGES
)"
  [[ "$policy_authority_privileges" == 't|f|f|f|f' ]] || {
    echo "runtime current policy authority privileges are not read-only: $policy_authority_privileges" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO user_models (id, email, display_name, created_at, updated_at) VALUES ('migration_policy_authority_provisional', 'policy-authority@example.com', 'Policy Authority Provisional', '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00')"
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_EMPTY_POLICY_AUTHORITY_REJECTION'
BEGIN;
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at, policy_set_revision)
VALUES ('migration_policy_authority_provisional', 16, '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00', 1);
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at) VALUES
  ('migration_policy_authority_provisional', 'terms', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'privacy', 'local-dev-v1', 'acknowledged', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'community_guidelines', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00');
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at)
VALUES ('migration_policy_authority_provisional', 1, 'America/New_York', '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00');
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone)
VALUES ('migration_policy_authority_provisional', '2026-01-10 00:00:00+00', 'America/New_York');
UPDATE user_models
SET status = 'active', username = 'migration.policy.authority', profile_visibility = 'private'
WHERE id = 'migration_policy_authority_provisional';
COMMIT;
MIGRATION_EMPTY_POLICY_AUTHORITY_REJECTION
  then
    echo "version-24 migration allowed activation without a published policy authority" >&2
    return 1
  fi
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_CURRENT_POLICY_AUTHORITY_FIXTURE'
INSERT INTO current_policy_set_models (
  singleton, revision, terms_version, privacy_policy_version, community_guidelines_version,
  terms_url, privacy_policy_url, community_guidelines_url, support_url, updated_at
) VALUES (
  true, 1, 'local-dev-v1', 'local-dev-v1', 'local-dev-v1',
  'http://localhost:5173/legal/terms', 'http://localhost:5173/legal/privacy',
  'http://localhost:5173/community-guidelines', 'http://localhost:5173/support',
  '2026-01-10 00:00:00+00'
);
MIGRATION_CURRENT_POLICY_AUTHORITY_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_STALE_POLICY_REVISION_REJECTION'
BEGIN;
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at, policy_set_revision)
VALUES ('migration_policy_authority_provisional', 16, '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00', 2);
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at) VALUES
  ('migration_policy_authority_provisional', 'terms', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'privacy', 'local-dev-v1', 'acknowledged', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'community_guidelines', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00');
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at)
VALUES ('migration_policy_authority_provisional', 1, 'America/New_York', '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00');
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone)
VALUES ('migration_policy_authority_provisional', '2026-01-10 00:00:00+00', 'America/New_York');
UPDATE user_models
SET status = 'active', username = 'migration.policy.authority', profile_visibility = 'private'
WHERE id = 'migration_policy_authority_provisional';
COMMIT;
MIGRATION_STALE_POLICY_REVISION_REJECTION
  then
    echo "version-24 migration allowed activation against a stale policy revision" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_STALE_POLICY_VERSION_REJECTION'
BEGIN;
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at, policy_set_revision)
VALUES ('migration_policy_authority_provisional', 16, '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00', 1);
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at) VALUES
  ('migration_policy_authority_provisional', 'terms', 'stale-terms', 'accepted', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'privacy', 'local-dev-v1', 'acknowledged', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'community_guidelines', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00');
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at)
VALUES ('migration_policy_authority_provisional', 1, 'America/New_York', '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00');
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone)
VALUES ('migration_policy_authority_provisional', '2026-01-10 00:00:00+00', 'America/New_York');
UPDATE user_models
SET status = 'active', username = 'migration.policy.authority', profile_visibility = 'private'
WHERE id = 'migration_policy_authority_provisional';
COMMIT;
MIGRATION_STALE_POLICY_VERSION_REJECTION
  then
    echo "version-24 migration allowed activation against stale policy versions" >&2
    return 1
  fi
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_MATCHING_POLICY_AUTHORITY_ACTIVATION'
BEGIN;
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at, policy_set_revision)
VALUES ('migration_policy_authority_provisional', 16, '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00', 1);
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at) VALUES
  ('migration_policy_authority_provisional', 'terms', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'privacy', 'local-dev-v1', 'acknowledged', '2026-01-10 00:00:00+00'),
  ('migration_policy_authority_provisional', 'community_guidelines', 'local-dev-v1', 'accepted', '2026-01-10 00:00:00+00');
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at)
VALUES ('migration_policy_authority_provisional', 1, 'America/New_York', '2026-01-10 00:00:00+00', '2026-01-10 00:00:00+00');
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone)
VALUES ('migration_policy_authority_provisional', '2026-01-10 00:00:00+00', 'America/New_York');
UPDATE user_models
SET status = 'active', username = 'migration.policy.authority', profile_visibility = 'private'
WHERE id = 'migration_policy_authority_provisional';
COMMIT;
MIGRATION_MATCHING_POLICY_AUTHORITY_ACTIVATION
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT status, policy_set_revision FROM user_models JOIN user_account_activation_models ON user_account_activation_models.user_id = user_models.id WHERE user_models.id = 'migration_policy_authority_provisional'")" == 'active|1' ]] || {
    echo "version-24 migration rejected activation matching the shared current policy authority" >&2
    return 1
  }
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d app -c \
    "UPDATE current_policy_set_models SET revision = 2 WHERE singleton"; then
    echo "runtime role mutated the current policy authority" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000024_current_policy_authority.down.sql; then
    echo "version-24 down migration removed populated policy authority or revision evidence" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT revision, terms_version FROM current_policy_set_models WHERE singleton')" == '1|local-dev-v1' ]] || {
    echo "refused version-24 rollback changed the current policy authority" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 25
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '25|f' ]] || { echo "expected clean current migration ledger 25|f, got $ledger" >&2; return 1; }
  provider_email_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PROVIDER_EMAIL_VERIFICATION_SCHEMA'
SELECT
  (SELECT is_nullable
   FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'user_models' AND column_name = 'provider_email_verified'),
  (SELECT column_default
   FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'user_models' AND column_name = 'provider_email_verified'),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_constraint
    WHERE conrelid = 'public.user_models'::regclass
      AND conname = 'user_models_verified_provider_email_check'
      AND convalidated
  ),
  (SELECT count(*) FROM public.user_models WHERE provider_email_verified);
MIGRATION_PROVIDER_EMAIL_VERIFICATION_SCHEMA
)"
  [[ "$provider_email_schema" == 'NO|false|t|0' ]] || {
    echo "version-25 migration trusted legacy email or has incomplete provenance schema: $provider_email_schema" >&2
    return 1
  }
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE user_models SET email = '', provider_email_verified = true WHERE id = 'migration_upgrade_user'"; then
    echo "version-25 migration allowed verified provenance for an empty email" >&2
    return 1
  fi
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "UPDATE user_models SET provider_email_verified = true WHERE id = 'migration_upgrade_duplicate_email'"
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000025_provider_email_verification.down.sql; then
    echo "version-25 down migration removed populated provider-email provenance" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT email, provider_email_verified FROM user_models WHERE id = 'migration_upgrade_duplicate_email'")" == 'SHARED-RECOVERY@EXAMPLE.COM|t' ]] || {
    echo "refused version-25 rollback changed provider-email provenance" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 26
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '26|f' ]] || { echo "expected clean current migration ledger 26|f, got $ledger" >&2; return 1; }

  docker compose run --rm app-migrate -target-version 27
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '27|f' ]] || { echo "expected clean current migration ledger 27|f, got $ledger" >&2; return 1; }
  activity_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ACTIVITY_SCHEMA'
SELECT
  to_regclass('public.running_timer_models') IS NOT NULL,
  to_regclass('public.recorded_activity_models') IS NOT NULL,
  to_regclass('public.activity_mutation_models') IS NOT NULL,
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'recorded_activity_models' AND column_name = 'duration'
  ),
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_constraint
    WHERE conrelid = 'public.running_timer_models'::regclass
      AND contype = 'u'
      AND pg_get_constraintdef(oid) = 'UNIQUE (participant_id, path_id)'
  );
MIGRATION_ACTIVITY_SCHEMA
)"
  [[ "$activity_schema" == 't|t|t|t|t' ]] || {
    echo "version-27 activity schema is incomplete: $activity_schema" >&2
    return 1
  }
  activity_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ACTIVITY_PRIVILEGES'
SELECT
  has_table_privilege('app', 'public.running_timer_models', 'SELECT'),
  has_table_privilege('app', 'public.running_timer_models', 'INSERT'),
  has_table_privilege('app', 'public.running_timer_models', 'DELETE'),
  has_table_privilege('app', 'public.running_timer_models', 'UPDATE'),
  has_table_privilege('app', 'public.recorded_activity_models', 'SELECT'),
  has_table_privilege('app', 'public.recorded_activity_models', 'INSERT'),
  has_table_privilege('app', 'public.recorded_activity_models', 'UPDATE'),
  has_table_privilege('app', 'public.recorded_activity_models', 'DELETE'),
  has_table_privilege('app', 'public.recorded_activity_models', 'TRUNCATE'),
  has_table_privilege('app', 'public.activity_mutation_models', 'SELECT'),
  has_table_privilege('app', 'public.activity_mutation_models', 'INSERT'),
  has_table_privilege('app', 'public.activity_mutation_models', 'UPDATE'),
  has_table_privilege('app', 'public.activity_mutation_models', 'DELETE');
MIGRATION_ACTIVITY_PRIVILEGES
)"
  [[ "$activity_privileges" == 't|t|t|f|t|t|f|f|f|t|t|t|f' ]] || {
    echo "version-27 activity runtime privileges are not least privilege: $activity_privileges" >&2
    return 1
  }

  # An empty rollback is reversible, but any durable timer state must make the
  # down migration fail before it revokes privileges or drops tables.
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000027_create_activities.down.sql
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT to_regclass('public.running_timer_models') IS NULL, to_regclass('public.recorded_activity_models') IS NULL, to_regclass('public.activity_mutation_models') IS NULL")" == 't|t|t' ]] || {
    echo "empty version-27 rollback retained activity tables" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000027_create_activities.up.sql
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_ACTIVITY_ROLLBACK_FIXTURE'
INSERT INTO path_models (id, owner_user_id, name, visibility, created_at, updated_at)
VALUES ('migration-activity-path', 'migration_upgrade_user', 'Migration activity', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO running_timer_models (id, path_id, participant_id, started_at, occurrence_time_zone)
VALUES ('migration-running-timer', 'migration-activity-path', 'migration_upgrade_user', CURRENT_TIMESTAMP, 'Etc/UTC');
MIGRATION_ACTIVITY_ROLLBACK_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000027_create_activities.down.sql; then
    echo "version-27 down migration destroyed durable timer state" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT to_regclass('public.running_timer_models') IS NOT NULL, (SELECT count(*) FROM running_timer_models WHERE id = 'migration-running-timer')")" == 't|1' ]] || {
    echo "refused version-27 rollback changed durable timer state" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "DELETE FROM path_models WHERE id = 'migration-activity-path'"

  docker compose run --rm app-migrate -target-version 28
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '28|f' ]] || { echo "expected clean current migration ledger 28|f, got $ledger" >&2; return 1; }
  goal_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_GOAL_SCHEMA'
SELECT
  string_agg(column_name || ':' || is_nullable, ',' ORDER BY ordinal_position),
  (SELECT count(*)
   FROM pg_catalog.pg_constraint
   WHERE conrelid = 'public.path_models'::regclass
     AND conname IN ('path_models_interval_goal_check', 'path_models_overall_target_check')
     AND convalidated)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'path_models'
  AND column_name IN (
    'interval_goal_target_seconds', 'interval_goal_recurrence',
    'interval_goal_start_minute', 'interval_goal_start_hour',
    'interval_goal_start_weekday', 'interval_goal_start_day',
    'interval_goal_start_month', 'overall_target_seconds'
  );
MIGRATION_PATH_GOAL_SCHEMA
)"
  [[ "$goal_schema" == 'interval_goal_target_seconds:YES,interval_goal_recurrence:YES,interval_goal_start_minute:YES,interval_goal_start_hour:YES,interval_goal_start_weekday:YES,interval_goal_start_day:YES,interval_goal_start_month:YES,overall_target_seconds:YES|2' ]] || {
    echo "version-28 optional Path goal schema is incomplete: $goal_schema" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 29
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '29|f' ]] || { echo "expected clean current migration ledger 29|f, got $ledger" >&2; return 1; }
  manual_activity_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_MANUAL_ACTIVITY_SCHEMA'
SELECT
  (SELECT is_nullable FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'recorded_activity_models' AND column_name = 'note'),
  to_regclass('public.recorded_activity_models_path_created_idx') IS NOT NULL,
  to_regclass('public.recorded_activity_revision_models') IS NOT NULL,
  to_regclass('public.recorded_activity_revision_models_activity_replaced_idx') IS NOT NULL,
  has_table_privilege('app', 'recorded_activity_revision_models', 'SELECT'),
  has_table_privilege('app', 'recorded_activity_revision_models', 'INSERT'),
  has_table_privilege('app', 'recorded_activity_revision_models', 'UPDATE'),
  has_table_privilege('app', 'recorded_activity_revision_models', 'DELETE'),
  has_column_privilege('app', 'recorded_activity_models', 'note', 'UPDATE'),
  has_table_privilege('app', 'recorded_activity_models', 'DELETE');
MIGRATION_MANUAL_ACTIVITY_SCHEMA
)"
  [[ "$manual_activity_schema" == 'YES|t|t|t|t|t|f|f|t|f' ]] || {
    echo "version-29 manual activity schema or least-privilege grants are incomplete: $manual_activity_schema" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 30
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '30|f' ]] || { echo "expected clean current migration ledger 30|f, got $ledger" >&2; return 1; }
  activity_deletion_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_ACTIVITY_DELETION_SCHEMA'
SELECT
  (SELECT is_nullable FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'activity_mutation_models' AND column_name = 'result_activity_deleted'),
  (SELECT is_nullable FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'activity_mutation_models' AND column_name = 'result_accumulated_seconds'),
  (SELECT count(*)
   FROM pg_catalog.pg_constraint
   WHERE conrelid = 'public.activity_mutation_models'::regclass
     AND conname IN (
       'activity_mutation_models_deleted_result_check',
       'activity_mutation_models_accumulated_seconds_check'
     )
     AND convalidated),
  has_table_privilege('app', 'recorded_activity_models', 'DELETE'),
  has_table_privilege('app', 'recorded_activity_models', 'TRUNCATE'),
  has_table_privilege('app', 'recorded_activity_revision_models', 'DELETE');
MIGRATION_ACTIVITY_DELETION_SCHEMA
)"
  [[ "$activity_deletion_schema" == 'NO|YES|2|t|f|f' ]] || {
    echo "activity deletion schema or least-privilege grants are incomplete: $activity_deletion_schema" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 31
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '31|f' ]] || { echo "expected clean current migration ledger 31|f, got $ledger" >&2; return 1; }
  path_archive_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_ARCHIVE_SCHEMA'
SELECT
  (SELECT is_nullable FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'path_models' AND column_name = 'archived_at'),
  (SELECT count(*) FROM pg_catalog.pg_constraint
   WHERE conrelid = 'public.path_models'::regclass
     AND conname = 'path_models_archive_time_check' AND convalidated),
  to_regclass('public.path_models_active_page_idx') IS NOT NULL,
  to_regclass('public.path_models_archived_page_idx') IS NOT NULL,
  has_table_privilege('app', 'path_models', 'UPDATE'),
  has_table_privilege('app', 'path_models', 'TRUNCATE');
MIGRATION_PATH_ARCHIVE_SCHEMA
)"
  [[ "$path_archive_schema" == 'YES|1|t|t|t|f' ]] || {
    echo "version-31 Path archive schema or least-privilege grants are incomplete: $path_archive_schema" >&2
    return 1
  }

  # Empty rollback is reversible, while durable archive state must make the
  # down migration fail before it drops the column, constraint, or indexes.
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000031_path_archival.down.sql
  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'path_models' AND column_name = 'archived_at'")" == '0' ]] || {
    echo "empty version-31 rollback retained Path archive state" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000031_path_archival.up.sql
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PATH_ARCHIVE_ROLLBACK_FIXTURE'
INSERT INTO path_models (id, owner_user_id, name, visibility, created_at, updated_at, archived_at)
VALUES ('migration-archived-path', 'migration_upgrade_user', 'Migration archived Path', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
MIGRATION_PATH_ARCHIVE_ROLLBACK_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000031_path_archival.down.sql; then
    echo "version-31 down migration destroyed durable Path archive state" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT (SELECT count(*) FROM path_models WHERE id = 'migration-archived-path' AND archived_at IS NOT NULL), (SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'path_models' AND column_name = 'archived_at')")" == '1|1' ]] || {
    echo "refused version-31 rollback changed durable Path archive state" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "DELETE FROM path_models WHERE id = 'migration-archived-path'"

  docker compose run --rm app-migrate -target-version 32
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '32|f' ]] || { echo "expected clean current migration ledger 32|f, got $ledger" >&2; return 1; }
  path_invitation_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_INVITATION_SCHEMA'
SELECT
  (SELECT count(*) FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'path_invitation_models'),
  (SELECT count(*) FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'notification_models'),
  (SELECT count(*) FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'notification_push_outbox_models'),
  (SELECT count(*) FROM pg_catalog.pg_constraint
   WHERE conrelid IN (
     'public.path_invitation_models'::regclass,
     'public.notification_models'::regclass,
     'public.notification_push_outbox_models'::regclass
   ) AND contype = 'c' AND convalidated),
  (SELECT count(*) FROM pg_catalog.pg_constraint
   WHERE conrelid IN (
     'public.path_invitation_models'::regclass,
     'public.notification_models'::regclass,
     'public.notification_push_outbox_models'::regclass
   ) AND contype = 'f' AND convalidated),
  to_regclass('public.path_invitation_models_one_pending_idx') IS NOT NULL,
  to_regclass('public.path_invitation_models_recipient_pending_idx') IS NOT NULL,
  to_regclass('public.notification_models_invitation_kind_recipient_idx') IS NOT NULL,
  to_regclass('public.notification_models_recipient_section_idx') IS NOT NULL,
  to_regclass('public.notification_push_outbox_models_claim_idx') IS NOT NULL,
  (SELECT position('path_invitation.created' in pg_get_constraintdef(oid)) > 0
      AND position('path_invitation.listed' in pg_get_constraintdef(oid)) > 0
      AND position('path_invitation.accepted' in pg_get_constraintdef(oid)) > 0
   FROM pg_catalog.pg_constraint
   WHERE conrelid = 'public.audit_event_models'::regclass
     AND conname = 'audit_event_models_action_check'
     AND convalidated);
MIGRATION_PATH_INVITATION_SCHEMA
)"
  [[ "$path_invitation_schema" == '10|12|9|20|9|t|t|t|t|t|t' ]] || {
    echo "version-32 Path invitation schema, constraints, indexes, or audit actions are incomplete: $path_invitation_schema" >&2
    return 1
  }
  path_invitation_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PATH_INVITATION_PRIVILEGES'
SELECT
  has_table_privilege('app', 'path_invitation_models', 'SELECT'),
  has_table_privilege('app', 'path_invitation_models', 'INSERT'),
  has_table_privilege('app', 'path_invitation_models', 'UPDATE'),
  has_table_privilege('app', 'path_invitation_models', 'DELETE'),
  has_table_privilege('app', 'path_invitation_models', 'TRUNCATE'),
  has_table_privilege('app', 'notification_models', 'SELECT'),
  has_table_privilege('app', 'notification_models', 'INSERT'),
  has_table_privilege('app', 'notification_models', 'UPDATE'),
  has_table_privilege('app', 'notification_models', 'DELETE'),
  has_table_privilege('app', 'notification_models', 'TRUNCATE'),
  has_table_privilege('app', 'notification_push_outbox_models', 'SELECT'),
  has_table_privilege('app', 'notification_push_outbox_models', 'INSERT'),
  has_table_privilege('app', 'notification_push_outbox_models', 'UPDATE'),
  has_table_privilege('app', 'notification_push_outbox_models', 'DELETE'),
  has_table_privilege('app', 'notification_push_outbox_models', 'TRUNCATE');
MIGRATION_PATH_INVITATION_PRIVILEGES
)"
  [[ "$path_invitation_privileges" == 't|t|t|f|f|t|t|t|f|f|t|t|t|f|f' ]] || {
    echo "version-32 Path invitation runtime privileges are not least privilege: $path_invitation_privileges" >&2
    return 1
  }

  # Empty invitation state is reversible, including the audit action expansion.
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000032_path_invitations.down.sql
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT to_regclass('public.path_invitation_models') IS NULL, to_regclass('public.notification_models') IS NULL, to_regclass('public.notification_push_outbox_models') IS NULL")" == 't|t|t' ]] || {
    echo "empty version-32 rollback retained Path invitation state" >&2
    return 1
  }
  [[ "$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT position('path_invitation.created' in pg_get_constraintdef(oid)) = 0 FROM pg_catalog.pg_constraint WHERE conrelid = 'public.audit_event_models'::regclass AND conname = 'audit_event_models_action_check'")" == 't' ]] || {
    echo "empty version-32 rollback retained Path invitation audit actions" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000032_path_invitations.up.sql

  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PATH_INVITATION_ROLLBACK_FIXTURE'
INSERT INTO path_models (id, owner_user_id, name, visibility, created_at, updated_at)
VALUES ('migration-invitation-path', 'migration_upgrade_user', 'Migration invitation Path', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO path_invitation_models (
  id, path_id, inviter_user_id, recipient_user_id, offered_role, created_at
)
VALUES (
  'migration-path-invitation', 'migration-invitation-path', 'migration_upgrade_user',
  'migration_upgrade_duplicate_email', 'participant', CURRENT_TIMESTAMP
);
INSERT INTO notification_models (
  id, recipient_user_id, actor_user_id, path_id, path_invitation_id, kind,
  presentation_class, channel, offered_role, created_at
)
VALUES (
  'migration-path-invitation-notification', 'migration_upgrade_duplicate_email',
  'migration_upgrade_user', 'migration-invitation-path', 'migration-path-invitation',
  'path_invitation_received', 'actionable', 'path_access', 'participant', CURRENT_TIMESTAMP
);
INSERT INTO notification_push_outbox_models (notification_id, created_at)
VALUES ('migration-path-invitation-notification', CURRENT_TIMESTAMP);
MIGRATION_PATH_INVITATION_ROLLBACK_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO path_invitation_models (id, path_id, inviter_user_id, recipient_user_id, offered_role, created_at) VALUES ('migration-duplicate-path-invitation', 'migration-invitation-path', 'migration_upgrade_user', 'migration_upgrade_duplicate_email', 'supporter', CURRENT_TIMESTAMP)"; then
    echo "version-32 migration allowed duplicate pending invitations for one Path recipient" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO path_invitation_models (id, path_id, inviter_user_id, recipient_user_id, offered_role, created_at) VALUES ('migration-invalid-path-invitation', 'migration-invitation-path', 'migration_upgrade_user', 'migration_upgrade_provisional', 'administrator', CURRENT_TIMESTAMP)"; then
    echo "version-32 migration accepted an invalid offered role" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000032_path_invitations.down.sql; then
    echo "version-32 down migration destroyed Path invitation delivery evidence" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT (SELECT count(*) FROM path_invitation_models WHERE id = 'migration-path-invitation'), (SELECT count(*) FROM notification_models WHERE id = 'migration-path-invitation-notification'), (SELECT count(*) FROM notification_push_outbox_models WHERE notification_id = 'migration-path-invitation-notification')")" == '1|1|1' ]] || {
    echo "refused version-32 rollback changed Path invitation delivery evidence" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 33
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '33|f' ]] || { echo "expected clean current migration ledger 33|f, got $ledger" >&2; return 1; }
  push_persistence_schema="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PUSH_PERSISTENCE_SCHEMA'
SELECT
  (SELECT count(*) FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'push_installation_models'),
  (SELECT count(*) FROM information_schema.columns
   WHERE table_schema = 'public' AND table_name = 'notification_push_delivery_models'),
  to_regclass('public.push_installation_models_active_token_idx') IS NOT NULL,
  to_regclass('public.push_installation_models_owner_active_idx') IS NOT NULL,
  to_regclass('public.notification_push_delivery_models_claim_idx') IS NOT NULL,
  to_regprocedure('public.validate_push_delivery_installation()') IS NOT NULL,
  EXISTS (
    SELECT 1 FROM pg_catalog.pg_trigger
    WHERE tgrelid = 'public.notification_push_delivery_models'::regclass
      AND tgname = 'notification_push_delivery_installation_check'
      AND tgenabled <> 'D'
      AND NOT tgisinternal
  ),
  (SELECT bool_and(convalidated) FROM pg_catalog.pg_constraint
   WHERE conrelid IN (
     'public.push_installation_models'::regclass,
     'public.notification_push_delivery_models'::regclass
   ));
MIGRATION_PUSH_PERSISTENCE_SCHEMA
)"
  [[ "$push_persistence_schema" == '11|19|t|t|t|t|t|t' ]] || {
    echo "version-33 push persistence schema, constraints, indexes, function, or trigger are incomplete: $push_persistence_schema" >&2
    return 1
  }
  push_persistence_privileges="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app <<'MIGRATION_PUSH_PERSISTENCE_PRIVILEGES'
SELECT
  has_table_privilege('app', 'push_installation_models', 'SELECT'),
  has_table_privilege('app', 'push_installation_models', 'INSERT'),
  has_table_privilege('app', 'push_installation_models', 'UPDATE'),
  has_table_privilege('app', 'push_installation_models', 'DELETE'),
  has_table_privilege('app', 'push_installation_models', 'TRUNCATE'),
  has_table_privilege('app', 'notification_push_delivery_models', 'SELECT'),
  has_table_privilege('app', 'notification_push_delivery_models', 'INSERT'),
  has_table_privilege('app', 'notification_push_delivery_models', 'UPDATE'),
  has_table_privilege('app', 'notification_push_delivery_models', 'DELETE'),
  has_table_privilege('app', 'notification_push_delivery_models', 'TRUNCATE');
MIGRATION_PUSH_PERSISTENCE_PRIVILEGES
)"
  [[ "$push_persistence_privileges" == 't|t|t|f|f|t|t|t|f|f' ]] || {
    echo "version-33 push persistence runtime privileges are not least privilege: $push_persistence_privileges" >&2
    return 1
  }

  # Empty push persistence is reversible without changing the migration ledger.
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000033_push_persistence.down.sql
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT to_regclass('public.push_installation_models') IS NULL, to_regclass('public.notification_push_delivery_models') IS NULL, to_regprocedure('public.validate_push_delivery_installation()') IS NULL")" == 't|t|t' ]] || {
    echo "empty version-33 rollback retained push persistence state" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000033_push_persistence.up.sql

  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PUSH_INSTALLATION_ROLLBACK_FIXTURE'
INSERT INTO push_installation_models (
  id, owner_user_id, provider, platform, locale,
  token_ciphertext, token_nonce, token_hash, created_at, updated_at
)
VALUES (
  'migration-push-installation', 'migration_upgrade_duplicate_email', 'expo', 'ios', 'es',
  decode(repeat('01', 17), 'hex'), decode(repeat('02', 12), 'hex'),
  decode(repeat('03', 32), 'hex'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
);
MIGRATION_PUSH_INSTALLATION_ROLLBACK_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "INSERT INTO notification_push_delivery_models (notification_id, installation_id, recipient_user_id, provider, platform, locale, token_ciphertext, token_nonce, token_hash, available_at, created_at) VALUES ('migration-path-invitation-notification', 'migration-push-installation', 'migration_upgrade_user', 'expo', 'ios', 'es', decode(repeat('01', 17), 'hex'), decode(repeat('02', 12), 'hex'), decode(repeat('03', 32), 'hex'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"; then
    echo "version-33 migration allowed a push installation to cross recipient boundaries" >&2
    return 1
  fi
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000033_push_persistence.down.sql; then
    echo "version-33 down migration destroyed durable push installation state" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT (SELECT count(*) FROM push_installation_models WHERE id = 'migration-push-installation'), to_regclass('public.push_installation_models') IS NOT NULL, to_regprocedure('public.validate_push_delivery_installation()') IS NOT NULL")" == '1|t|t' ]] || {
    echo "refused version-33 rollback changed durable push installation state" >&2
    return 1
  }

  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PUSH_DELIVERY_ROLLBACK_FIXTURE'
INSERT INTO notification_push_delivery_models (
  notification_id, installation_id, recipient_user_id, provider, platform, locale,
  token_ciphertext, token_nonce, token_hash, available_at, created_at
)
VALUES (
  'migration-path-invitation-notification', 'migration-push-installation',
  'migration_upgrade_duplicate_email', 'expo', 'ios', 'es',
  decode(repeat('01', 17), 'hex'), decode(repeat('02', 12), 'hex'),
  decode(repeat('03', 32), 'hex'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
);
DELETE FROM push_installation_models WHERE id = 'migration-push-installation';
MIGRATION_PUSH_DELIVERY_ROLLBACK_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000033_push_persistence.down.sql; then
    echo "version-33 down migration destroyed durable push delivery evidence" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT (SELECT count(*) FROM notification_push_delivery_models WHERE notification_id = 'migration-path-invitation-notification' AND installation_id = 'migration-push-installation'), (SELECT count(*) FROM push_installation_models WHERE id = 'migration-push-installation'), to_regclass('public.notification_push_delivery_models') IS NOT NULL")" == '1|0|t' ]] || {
    echo "refused version-33 rollback changed durable push delivery evidence" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "DELETE FROM notification_push_delivery_models WHERE notification_id = 'migration-path-invitation-notification' AND installation_id = 'migration-push-installation'"
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "DELETE FROM path_models WHERE id = 'migration-invitation-path'"

  docker compose run --rm app-migrate -target-version 34
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '34|f' ]] || { echo "expected clean current migration ledger 34|f, got $ledger" >&2; return 1; }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PATH_REJECTION_AUDIT_FIXTURE'
INSERT INTO audit_event_models (
  id, owner_user_id, actor_user_id, action, target_type, target_id,
  outcome, correlation_id, occurred_at
)
VALUES (
  'migration-path-rejection-audit', 'migration_upgrade_user',
  'migration_upgrade_duplicate_email', 'path_invitation.rejected',
  'path_invitation', 'migration-rejected-invitation', 'succeeded',
  'migration-path-rejection-correlation', CURRENT_TIMESTAMP
);
MIGRATION_PATH_REJECTION_AUDIT_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000034_path_invitation_rejection_audit.down.sql; then
    echo "version-34 down migration removed retained Path rejection audit evidence" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT (SELECT count(*) FROM audit_event_models WHERE id = 'migration-path-rejection-audit'), position('path_invitation.rejected' in pg_get_constraintdef(oid)) > 0 FROM pg_constraint WHERE conname = 'audit_event_models_action_check'")" == '1|t' ]] || {
    echo "refused version-34 rollback changed rejection audit evidence or taxonomy" >&2
    return 1
  }

  docker compose run --rm app-migrate -target-version 35
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$ledger" == '35|f' ]] || { echo "expected clean current migration ledger 35|f, got $ledger" >&2; return 1; }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app <<'MIGRATION_PATH_REJECTION_REPLAY_FIXTURE'
INSERT INTO path_models (id, owner_user_id, name, visibility, created_at, updated_at)
VALUES ('migration-rejection-replay-path', 'migration_upgrade_user', 'Migration replay Path', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO path_invitation_models (
  id, path_id, inviter_user_id, recipient_user_id, offered_role,
  created_at, rejected_at, rejection_unread_count
)
VALUES (
  'migration-rejection-replay-invitation', 'migration-rejection-replay-path',
  'migration_upgrade_user', 'migration_upgrade_duplicate_email', 'supporter',
  CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2
);
MIGRATION_PATH_REJECTION_REPLAY_FIXTURE
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
    < apps/api/internal/adapters/dbmigrations/000035_path_invitation_rejection_replay.down.sql; then
    echo "version-35 down migration removed retained Path rejection replay evidence" >&2
    return 1
  fi
  [[ "$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT (SELECT rejection_unread_count FROM path_invitation_models WHERE id = 'migration-rejection-replay-invitation'), EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'path_invitation_models' AND column_name = 'rejection_unread_count')")" == '2|t' ]] || {
    echo "refused version-35 rollback changed rejection replay evidence or schema" >&2
    return 1
  }
  docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c \
    "DELETE FROM path_models WHERE id = 'migration-rejection-replay-path'"

  before_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM authorization_outbox_models WHERE id = 'migration_upgrade_legacy_outbox'")"
  duplicate_before_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM user_models WHERE lower(email) = 'shared-recovery@example.com'")"
  decline_before_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM duplicate_email_recovery_declines WHERE provisional_user_id = 'migration_upgrade_provisional' AND normalized_email = 'shared-recovery@example.com'")"
  username_before_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT username FROM user_models WHERE id = 'migration_upgrade_duplicate_email'")"
  activation_before_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM user_account_activation_models WHERE user_id = 'migration_upgrade_provisional'")"
  policy_authority_before_rerun="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT revision, terms_version, privacy_policy_version, community_guidelines_version FROM current_policy_set_models WHERE singleton')"
  provider_email_before_rerun="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT email, provider_email_verified FROM user_models WHERE id = 'migration_upgrade_duplicate_email'")"
  docker compose run --rm app-migrate
  after_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM authorization_outbox_models WHERE id = 'migration_upgrade_legacy_outbox'")"
  duplicate_after_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM user_models WHERE lower(email) = 'shared-recovery@example.com'")"
  decline_after_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM duplicate_email_recovery_declines WHERE provisional_user_id = 'migration_upgrade_provisional' AND normalized_email = 'shared-recovery@example.com'")"
  username_after_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT username FROM user_models WHERE id = 'migration_upgrade_duplicate_email'")"
  activation_after_rerun="$(docker compose exec -T postgres psql -At -U app_migrator -d app -c "SELECT count(*) FROM user_account_activation_models WHERE user_id = 'migration_upgrade_provisional'")"
  policy_authority_after_rerun="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT revision, terms_version, privacy_policy_version, community_guidelines_version FROM current_policy_set_models WHERE singleton')"
  provider_email_after_rerun="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c "SELECT email, provider_email_verified FROM user_models WHERE id = 'migration_upgrade_duplicate_email'")"
  ledger="$(docker compose exec -T postgres psql -At -F '|' -U app_migrator -d app -c 'SELECT version, dirty FROM schema_migrations')"
  [[ "$before_rerun" == 1 && "$after_rerun" == 1 && "$duplicate_before_rerun" == 2 && "$duplicate_after_rerun" == 2 && "$decline_before_rerun" == 1 && "$decline_after_rerun" == 1 && "$username_before_rerun" == 'Identity.Paths_1' && "$username_after_rerun" == 'Identity.Paths_1' && "$activation_before_rerun" == 1 && "$activation_after_rerun" == 1 && "$policy_authority_before_rerun" == '1|local-dev-v1|local-dev-v1|local-dev-v1' && "$policy_authority_after_rerun" == "$policy_authority_before_rerun" && "$provider_email_before_rerun" == 'SHARED-RECOVERY@EXAMPLE.COM|t' && "$provider_email_after_rerun" == "$provider_email_before_rerun" && "$ledger" == "${latest_migration_version}|f" ]] || {
    echo "current migrator rerun changed the upgraded fixture or ledger" >&2
    return 1
  }

}

run_postgres_adapter_acceptance() {
  local postgres_host_port runtime_dsn migration_dsn retention_dsn
  postgres_host_port="${HOURPATHS_POSTGRES_HOST_PORT:-25432}"
  if [[ ! "$postgres_host_port" =~ ^[1-9][0-9]{0,4}$ ]] || (( 10#$postgres_host_port > 65535 )); then
    echo "HOURPATHS_POSTGRES_HOST_PORT must be a canonical TCP port from 1 through 65535" >&2
    return 1
  fi
  runtime_dsn="postgres://app:app@127.0.0.1:${postgres_host_port}/app?sslmode=disable"
  migration_dsn="postgres://app_migrator:app_migrator@127.0.0.1:${postgres_host_port}/app?sslmode=disable"
  retention_dsn="postgres://app_audit_retention:app_audit_retention@127.0.0.1:${postgres_host_port}/app?sslmode=disable"
  (
    cd apps/api
    GOWORK=off go test -count=1 ./internal/adapters/gormstore -args \
      -database-dsn "$runtime_dsn" -migration-database-dsn "$migration_dsn"
    GOWORK=off go test -count=1 ./internal/adapters/gormstore/path -args \
      -database-dsn "$runtime_dsn" -migration-database-dsn "$migration_dsn"
    GOWORK=off go test -count=1 ./internal/adapters/gormstore/activity -args \
      -database-dsn "$runtime_dsn" -migration-database-dsn "$migration_dsn"
    GOWORK=off go test -count=1 ./internal/adapters/ratelimit -args \
      -database-dsn "$runtime_dsn"
    GOWORK=off go test -count=1 ./internal/adapters/auditretention -args \
      -database-dsn "$migration_dsn" -retention-dsn "$retention_dsn"
  )
}

docker compose down --volumes --remove-orphans >/dev/null 2>&1 || true
docker compose up -d postgres
for _ in $(seq 1 60); do
  id="$(docker compose ps -q postgres)"
  [[ -n "$id" && "$(docker inspect -f '{{.State.Health.Status}}' "$id")" == healthy ]] && break
  sleep 1
done
run_migration_upgrade_acceptance
run_postgres_adapter_acceptance
# The full live suite deliberately sends far more traffic than one interactive
# client while retaining focused limiter behavior and Postgres boundary tests.
export HOURPATHS_REQUESTS_PER_MINUTE=10000
export HOURPATHS_AUDIT_EVENTS_PER_MINUTE=10000
docker compose up -d --build spicedb dex api web
unset HOURPATHS_REQUESTS_PER_MINUTE
unset HOURPATHS_AUDIT_EVENTS_PER_MINUTE
for _ in $(seq 1 180); do curl -fsS http://localhost:8080/readyz >/dev/null 2>&1 && curl -fsS "${dex_public_issuer}/.well-known/openid-configuration" >/dev/null 2>&1 && break; sleep 1; done
curl -fsS http://localhost:8080/healthz >/dev/null
curl -fsS http://localhost:8080/readyz >/dev/null
for _ in $(seq 1 180); do curl -fsS http://localhost:5173 >/dev/null 2>&1 && break; sleep 1; done
curl -fsS http://localhost:5173 >/dev/null

scalar_docs="$lock_dir/scalar-docs.html"
scalar_runtime="$lock_dir/scalar-api-reference-1.44.20.js"
curl -fsS http://localhost:8080/docs -o "$scalar_docs"
grep -Fq 'src="/docs/assets/scalar-api-reference-1.44.20.js"' "$scalar_docs"
if grep -Fq 'unpkg.com' "$scalar_docs"; then
  echo "Scalar documentation retained a third-party CDN dependency" >&2
  exit 1
fi
curl -fsS http://localhost:8080/docs/assets/scalar-api-reference-1.44.20.js -o "$scalar_runtime"
[[ "$(wc -c < "$scalar_runtime" | tr -d ' ')" == '3544608' ]] || { echo "self-hosted Scalar runtime size drifted" >&2; exit 1; }
python3 -c 'import hashlib,pathlib,sys; assert hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest() == "f349c815d31be09d11e386726da989e3af50c2f1885910764b51f5b0fae9e28e"' "$scalar_runtime"

SCALAR_ACCEPTANCE_BASE_URL="$api_base_url" node scripts/scalar-browser-acceptance.mjs
WEB_ACCEPTANCE_BASE_URL="$web_base_url" WEB_ACCEPTANCE_API_URL="$api_base_url" WEB_ACCEPTANCE_DEX_ORIGIN="${dex_public_issuer%/dex}" node scripts/web-browser-acceptance.mjs

web_image="$(docker compose images -q web)"
[[ -n "$web_image" ]] || { echo "Compose did not produce a web image" >&2; exit 1; }
assert_web_image_rejects_config() {
  local case_name="$1" container_name running exit_code
  shift
  container_name="${COMPOSE_PROJECT_NAME}-web-config-${case_name}"
  web_config_container="$container_name"
  docker rm --force "$container_name" >/dev/null 2>&1 || true
  docker run --detach --name "$container_name" "$@" "$web_image" >/dev/null
  running=true
  for _ in $(seq 1 15); do
    running="$(docker inspect --format '{{.State.Running}}' "$container_name")"
    [[ "$running" == false ]] && break
    sleep 1
  done
  if [[ "$running" != false ]]; then
    docker logs "$container_name" >&2 || true
    echo "production web image served with rejected configuration: $case_name" >&2
    return 1
  fi
  exit_code="$(docker inspect --format '{{.State.ExitCode}}' "$container_name")"
  if [[ "$exit_code" == 0 ]]; then
    docker logs "$container_name" >&2 || true
    echo "production web image exited successfully with rejected configuration: $case_name" >&2
    return 1
  fi
  docker rm --force "$container_name" >/dev/null
  web_config_container=""
}

valid_api='https://api.hourpaths.example'
valid_issuer='https://identity.hourpaths.example'
valid_client_id='hourpaths-web'
assert_web_image_rejects_config missing
assert_web_image_rejects_config malformed-environment \
  --env HOURPATHS_APP_ENV=staging --env HOURPATHS_API_URL="$valid_api" --env HOURPATHS_OIDC_ISSUER="$valid_issuer" --env HOURPATHS_WEB_OIDC_CLIENT_ID="$valid_client_id"
assert_web_image_rejects_config malformed-api-url \
  --env HOURPATHS_API_URL=not-a-url --env HOURPATHS_OIDC_ISSUER="$valid_issuer" --env HOURPATHS_WEB_OIDC_CLIENT_ID="$valid_client_id"
assert_web_image_rejects_config local-api-url \
  --env HOURPATHS_API_URL=http://127.0.0.1:8080 --env HOURPATHS_OIDC_ISSUER="$valid_issuer" --env HOURPATHS_WEB_OIDC_CLIENT_ID="$valid_client_id"
assert_web_image_rejects_config credentialed-issuer \
  --env HOURPATHS_API_URL="$valid_api" --env HOURPATHS_OIDC_ISSUER=https://user:password@identity.hourpaths.example --env HOURPATHS_WEB_OIDC_CLIENT_ID="$valid_client_id"
assert_web_image_rejects_config non-https-issuer \
  --env HOURPATHS_API_URL="$valid_api" --env HOURPATHS_OIDC_ISSUER=http://identity.hourpaths.example --env HOURPATHS_WEB_OIDC_CLIENT_ID="$valid_client_id"
assert_web_image_rejects_config blank-client-id \
  --env HOURPATHS_API_URL="$valid_api" --env HOURPATHS_OIDC_ISSUER="$valid_issuer" --env HOURPATHS_WEB_OIDC_CLIENT_ID=

token() {
  curl -fsS -X POST "${dex_public_issuer}/token" -d grant_type=password -d "client_id=$1" -d "username=$2" -d password=password -d scope='openid profile email' |
    python3 -c 'import json,sys;print(json.load(sys.stdin)["id_token"])'
}
identity_subject() {
  printf '%s' "$1" | python3 scripts/identity-fixture.py subject
}
identity_user_id() {
  python3 scripts/identity-fixture.py user-id "$dex_public_issuer" "$1"
}
session_from_identity() {
  identity_token="$1"
  identity_label="${2:-identity}"
  request_body="$(IDENTITY_TOKEN="$identity_token" python3 -c 'import json,os;print(json.dumps({"identityToken":os.environ["IDENTITY_TOKEN"]}))')"
  exchange_response="$(printf '%s' "$request_body" | curl -sS -X POST -H 'Content-Type: application/json' --data-binary @- -w $'\n%{http_code}' http://localhost:8080/v1/sessions)"
  exchange_status="${exchange_response##*$'\n'}"
  exchange_body="${exchange_response%$'\n'*}"
  if [[ "$exchange_status" != 200 ]]; then
    echo "application session exchange failed for $identity_label: HTTP $exchange_status: $exchange_body" >&2
    return 1
  fi
  printf '%s' "$exchange_body" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["token"])'
}
session() {
  session_from_identity "$(token "$1" "$2")" "$2"
}
expect_status() { expected="$1"; shift; actual="$(curl -sS -o /dev/null -w '%{http_code}' "$@")"; [[ "$actual" == "$expected" ]] || { echo "expected HTTP $expected, got $actual: $*" >&2; return 1; }; }

owner_identity_token="$(token hourpaths-web developer@example.com)"
owner_fixture_id="$(identity_user_id "$(identity_subject "$owner_identity_token")")"
provisional_owner_token="$(session_from_identity "$owner_identity_token" provisional-owner)"
expect_status 401 -X POST \
  -H "Authorization: Bearer $provisional_owner_token" \
  -H 'Idempotency-Key: provisional-path-create-key-0001' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Provisional Path"}' \
  http://localhost:8080/v1/paths
expect_status 401 -X POST \
  -H "Authorization: Bearer $provisional_owner_token" \
  -H 'Idempotency-Key: provisional-timer-start-key-001' \
  "http://localhost:8080/v1/paths/provisional-path/timer"
activated_owner="$(docker compose exec -T postgres psql -Atq -U app_migrator -d app -v "owner_id=$owner_fixture_id" <<'COMPLETE_ACTIVE_OWNER_FIXTURE'
BEGIN;
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at, policy_set_revision)
SELECT :'owner_id', 16, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, revision
FROM current_policy_set_models
WHERE singleton;
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at)
SELECT :'owner_id', 'terms', terms_version, 'accepted', CURRENT_TIMESTAMP FROM current_policy_set_models WHERE singleton
UNION ALL
SELECT :'owner_id', 'privacy', privacy_policy_version, 'acknowledged', CURRENT_TIMESTAMP FROM current_policy_set_models WHERE singleton
UNION ALL
SELECT :'owner_id', 'community_guidelines', community_guidelines_version, 'accepted', CURRENT_TIMESTAMP FROM current_policy_set_models WHERE singleton;
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at)
VALUES (:'owner_id', 1, 'America/New_York', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone)
VALUES (:'owner_id', CURRENT_TIMESTAMP, 'America/New_York');
UPDATE user_models
SET status = 'active', username = 'live.acceptance.owner', profile_visibility = 'private', updated_at = CURRENT_TIMESTAMP
WHERE id = :'owner_id' AND status = 'provisional'
RETURNING id;
UPDATE session_models
SET revoked_at = CURRENT_TIMESTAMP
WHERE user_id = :'owner_id' AND scopes = 'api:onboarding' AND revoked_at IS NULL;
COMMIT;
COMPLETE_ACTIVE_OWNER_FIXTURE
)"
[[ "$activated_owner" == "$owner_fixture_id" ]] || {
  echo "provisional browser identity was not isolated as the complete active-owner fixture" >&2
  exit 1
}
owner_active_identity_token=''
for _ in $(seq 1 5); do
  owner_active_identity_token="$(token hourpaths-web developer@example.com)"
  [[ "$owner_active_identity_token" != "$owner_identity_token" ]] && break
  sleep 1
done
[[ -n "$owner_active_identity_token" && "$owner_active_identity_token" != "$owner_identity_token" ]] || {
  echo "Dex did not issue a distinct active-owner identity token" >&2
  exit 1
}
owner_token="$(session_from_identity "$owner_active_identity_token" owner)"
expect_status 401 http://localhost:8080/v1/me
expect_status 401 -H 'Authorization: Bearer malformed' http://localhost:8080/v1/me
me="$(curl -fsS -H "Authorization: Bearer $owner_token" http://localhost:8080/v1/me)"
owner_id="$(printf '%s' "$me" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')"
[[ "$owner_id" == "$owner_fixture_id" ]] || { echo "active-owner fixture identity changed during exchange" >&2; exit 1; }
WEB_ACCEPTANCE_BASE_URL="$web_base_url" WEB_ACCEPTANCE_API_URL="$api_base_url" WEB_ACCEPTANCE_APPLICATION_TOKEN="$owner_token" node scripts/web-empty-home-acceptance.mjs

path_id='private-path-live-acceptance'
expired_path_token="$(python3 -c 'import base64;raw=b"hourpaths-expired-path-session-1";assert len(raw)==32;print(base64.urlsafe_b64encode(raw).rstrip(b"=").decode())')"
[[ ${#expired_path_token} -eq 43 ]] || { echo "expired Path session fixture is not a valid opaque token" >&2; exit 1; }
expired_path_token_hash="$(EXPIRED_PATH_TOKEN="$expired_path_token" python3 -c 'import hashlib,os;print(hashlib.sha256(os.environ["EXPIRED_PATH_TOKEN"].encode()).hexdigest())')"
administrator_identity_token="$(token hourpaths-web second@example.com)"
participant_identity_token="$(token hourpaths-web invited@example.com)"
supporter_identity_token="$(token hourpaths-web revoked@example.com)"
stranger_identity_token="$(token hourpaths-web uninvited@example.com)"
administrator_subject="$(identity_subject "$administrator_identity_token")"
participant_subject="$(identity_subject "$participant_identity_token")"
supporter_subject="$(identity_subject "$supporter_identity_token")"
stranger_subject="$(identity_subject "$stranger_identity_token")"
administrator_id="$(identity_user_id "$administrator_subject")"
participant_id="$(identity_user_id "$participant_subject")"
supporter_id="$(identity_user_id "$supporter_subject")"
stranger_id="$(identity_user_id "$stranger_subject")"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "owner_id=$owner_id" \
  -v "administrator_id=$administrator_id" \
  -v "participant_id=$participant_id" \
  -v "supporter_id=$supporter_id" \
  -v "stranger_id=$stranger_id" \
  -v "administrator_subject=$administrator_subject" \
  -v "participant_subject=$participant_subject" \
  -v "supporter_subject=$supporter_subject" \
  -v "stranger_subject=$stranger_subject" \
  -v "identity_issuer=$dex_public_issuer" \
  -v "expired_path_token_hash=$expired_path_token_hash" \
  -v "path_id=$path_id" <<'PATH_ACCEPTANCE_SEED'
INSERT INTO user_models (id, email, display_name, status, username, profile_visibility, created_at, updated_at) VALUES
  (:'administrator_id', 'second@example.com', 'Second', 'active', 'live.acceptance.administrator', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (:'participant_id', 'invited@example.com', 'Invited', 'active', 'live.acceptance.participant', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (:'supporter_id', 'revoked@example.com', 'Supporter', 'active', 'live.acceptance.supporter', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (:'stranger_id', 'uninvited@example.com', 'Stranger', 'active', 'live.acceptance.stranger', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO identity_models (issuer, subject, user_id) VALUES
  (:'identity_issuer', :'administrator_subject', :'administrator_id'),
  (:'identity_issuer', :'participant_subject', :'participant_id'),
  (:'identity_issuer', :'supporter_subject', :'supporter_id'),
  (:'identity_issuer', :'stranger_subject', :'stranger_id');
-- COMPLETE_ACTIVE_PATH_ROLE_FIXTURES: role accounts used by browser acceptance
-- must carry the same required active-account aggregate as product-created users.
INSERT INTO user_account_activation_models (user_id, minimum_age_attested, age_attested_at, completed_at, policy_set_revision)
SELECT role_user.id, 16, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, policy.revision
FROM (VALUES (:'administrator_id'), (:'participant_id'), (:'supporter_id'), (:'stranger_id')) AS role_user(id)
CROSS JOIN current_policy_set_models AS policy
WHERE policy.singleton;
INSERT INTO user_policy_acceptance_models (user_id, policy, version, acknowledgement, accepted_at)
SELECT role_user.id, accepted.policy, accepted.version, accepted.acknowledgement, CURRENT_TIMESTAMP
FROM (VALUES (:'administrator_id'), (:'participant_id'), (:'supporter_id'), (:'stranger_id')) AS role_user(id)
CROSS JOIN current_policy_set_models AS current_policy
CROSS JOIN LATERAL (VALUES
  ('terms', current_policy.terms_version, 'accepted'),
  ('privacy', current_policy.privacy_policy_version, 'acknowledged'),
  ('community_guidelines', current_policy.community_guidelines_version, 'accepted')
) AS accepted(policy, version, acknowledgement)
WHERE current_policy.singleton;
INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at) VALUES
  (:'administrator_id', 1, 'America/New_York', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (:'participant_id', 1, 'America/New_York', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (:'supporter_id', 1, 'America/New_York', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (:'stranger_id', 1, 'America/New_York', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO user_time_zone_history_models (user_id, effective_at, time_zone) VALUES
  (:'administrator_id', CURRENT_TIMESTAMP, 'America/New_York'),
  (:'participant_id', CURRENT_TIMESTAMP, 'America/New_York'),
  (:'supporter_id', CURRENT_TIMESTAMP, 'America/New_York'),
  (:'stranger_id', CURRENT_TIMESTAMP, 'America/New_York');
INSERT INTO session_models (token_hash, user_id, scopes, expires_at, absolute_expires_at, created_at)
VALUES (decode(:'expired_path_token_hash', 'hex'), :'owner_id', 'api:user', CURRENT_TIMESTAMP - INTERVAL '2 minutes', CURRENT_TIMESTAMP - INTERVAL '1 minute', CURRENT_TIMESTAMP - INTERVAL '3 minutes');
INSERT INTO path_models (id, owner_user_id, name, visibility, created_at, updated_at)
VALUES (:'path_id', :'owner_id', 'Guitar practice', 'private', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO path_membership_models (path_id, user_id, role) VALUES
  (:'path_id', :'administrator_id', 'administrator'),
  (:'path_id', :'participant_id', 'participant'),
  (:'path_id', :'supporter_id', 'supporter');
PATH_ACCEPTANCE_SEED

(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run 'TestPrivatePathViewPermissionMatrix|TestSeedPrivatePathHTTPAcceptanceRelationships' -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$path_id" \
    -acceptance-path-creator "$owner_id" \
    -acceptance-path-administrator "$administrator_id" \
    -acceptance-path-participant "$participant_id" \
    -acceptance-path-supporter "$supporter_id"
)

administrator_token="$(session_from_identity "$administrator_identity_token" administrator)"
participant_token="$(session_from_identity "$participant_identity_token" participant)"
supporter_token="$(session_from_identity "$supporter_identity_token" supporter)"
stranger_token="$(session_from_identity "$stranger_identity_token" stranger)"
sleep 1
stranger_second_device_identity_token="$(token hourpaths-web uninvited@example.com)"
[[ "$stranger_second_device_identity_token" != "$stranger_identity_token" ]] || {
  echo "Dex did not issue a distinct credential for the second-device session" >&2
  exit 1
}
[[ "$(identity_subject "$stranger_second_device_identity_token")" == "$stranger_subject" ]] || {
  echo "second-device credential did not resolve to the original identity" >&2
  exit 1
}
stranger_second_device_token="$(session_from_identity "$stranger_second_device_identity_token" stranger-second-device)"

successful_path_view_audit_count() {
  docker compose exec -T postgres psql -At -U app -d app \
    -v "owner_id=$owner_id" -v "path_id=$path_id" <<'SUCCESSFUL_PATH_VIEW_AUDIT_SQL'
SELECT count(*)
FROM audit_event_models
WHERE action = 'resource.viewed'
  AND owner_user_id = :'owner_id'
  AND target_type = 'path'
  AND target_id = :'path_id'
  AND outcome = 'succeeded';
SUCCESSFUL_PATH_VIEW_AUDIT_SQL
}

denied_path_view_audit_count() {
  docker compose exec -T postgres psql -At -U app -d app \
    -v "stranger_id=$stranger_id" -v "path_id=$path_id" <<'DENIED_PATH_VIEW_AUDIT_SQL'
SELECT count(*)
FROM audit_event_models
WHERE action = 'resource.access_denied'
  AND owner_user_id = :'stranger_id'
  AND actor_user_id = :'stranger_id'
  AND target_type = 'path'
  AND target_id = :'path_id'
  AND outcome = 'denied';
DENIED_PATH_VIEW_AUDIT_SQL
}

path_view_audit_before="$(successful_path_view_audit_count)"
[[ "$path_view_audit_before" =~ ^[0-9]+$ ]] || {
  echo "successful Path-view audit baseline was not an integer: $path_view_audit_before" >&2
  exit 1
}
for role_and_capabilities in \
  "$owner_token|{\"trackTime\":true,\"inviteMembers\":true,\"manageMembers\":true,\"manageGoals\":true,\"manageLifecycle\":true,\"manageVisibility\":true,\"renamePath\":true,\"transferOwnership\":true,\"leavePath\":false}|shared" \
  "$administrator_token|{\"trackTime\":true,\"inviteMembers\":true,\"manageMembers\":true,\"manageGoals\":true,\"manageLifecycle\":false,\"manageVisibility\":false,\"renamePath\":true,\"transferOwnership\":false,\"leavePath\":true}|shared" \
  "$participant_token|{\"trackTime\":true,\"inviteMembers\":false,\"manageMembers\":false,\"manageGoals\":false,\"manageLifecycle\":false,\"manageVisibility\":false,\"renamePath\":false,\"transferOwnership\":false,\"leavePath\":true}|shared" \
  "$supporter_token|{\"trackTime\":false,\"inviteMembers\":false,\"manageMembers\":false,\"manageGoals\":false,\"manageLifecycle\":false,\"manageVisibility\":false,\"renamePath\":false,\"transferOwnership\":false,\"leavePath\":true}|supporting"; do
  IFS='|' read -r role_token expected_capabilities expected_home_classification <<<"$role_and_capabilities"
  curl -fsS -H "Authorization: Bearer $role_token" "http://localhost:8080/v1/paths/$path_id" |
    EXPECTED_CAPABILITIES="$expected_capabilities" EXPECTED_HOME_CLASSIFICATION="$expected_home_classification" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"id":"private-path-live-acceptance","name":"Guitar practice","visibility":"private","capabilities":json.loads(os.environ["EXPECTED_CAPABILITIES"]),"home":{"classification":os.environ["EXPECTED_HOME_CLASSIFICATION"],"pinned":False}}'
  curl -fsS -H "Authorization: Bearer $role_token" 'http://localhost:8080/v1/paths?limit=25' |
    EXPECTED_CAPABILITIES="$expected_capabilities" EXPECTED_HOME_CLASSIFICATION="$expected_home_classification" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d["data"]==[{"id":"private-path-live-acceptance","name":"Guitar practice","visibility":"private","capabilities":json.loads(os.environ["EXPECTED_CAPABILITIES"]),"home":{"classification":os.environ["EXPECTED_HOME_CLASSIFICATION"],"pinned":False}}] and d["meta"]=={"homePreferences":{"orderMethod":"recent","revision":0,"pinnedPathIds":[],"manualPathIds":[]}}'
done
path_view_audit_after="$(successful_path_view_audit_count)"
[[ "$path_view_audit_after" =~ ^[0-9]+$ ]] &&
  ((path_view_audit_after - path_view_audit_before == 4)) || {
  echo "four authorized Path reads did not append exactly four view audits: before=$path_view_audit_before after=$path_view_audit_after" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $stranger_token" 'http://localhost:8080/v1/paths?limit=25' |
  python3 -c 'import json,sys;d=json.load(sys.stdin);assert d["data"]==[] and d["meta"]=={"homePreferences":{"orderMethod":"recent","revision":0,"pinnedPathIds":[],"manualPathIds":[]}}'

# PATH-05A proves the active-Path rename matrix against the real PostgreSQL and
# SpiceDB boundaries. Denied and stale attempts must not alter the Path.
for denied_role_token in "$participant_token" "$supporter_token"; do
  expect_status 404 -X PUT \
    -H "Authorization: Bearer $denied_role_token" \
    -H 'Idempotency-Key: path-05a-denied-rename-001' \
    -H 'Content-Type: application/json' \
    --data '{"expectedName":"Guitar practice","name":"Denied rename"}' \
    "http://localhost:8080/v1/paths/$path_id/name"
done
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$path_id" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]["name"]=="Guitar practice"'

path_05a_admin_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $administrator_token" \
  -H 'Idempotency-Key: path-05a-admin-rename-0001' \
  -H 'Content-Type: application/json' \
  --data '{"expectedName":"Guitar practice","name":"Shared practice"}' \
  "http://localhost:8080/v1/paths/$path_id/name")"
printf '%s' "$path_05a_admin_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Shared practice" and d["capabilities"]["renamePath"] is True'
path_05a_admin_replay="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $administrator_token" \
  -H 'Idempotency-Key: path-05a-admin-rename-0001' \
  -H 'Content-Type: application/json' \
  --data '{"expectedName":"Guitar practice","name":"Shared practice"}' \
  "http://localhost:8080/v1/paths/$path_id/name")"
[[ "$path_05a_admin_replay" == "$path_05a_admin_response" ]] || {
  echo "PATH-05A administrator replay changed the authoritative projection" >&2
  exit 1
}
expect_status 409 -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05a-stale-rename-0001' \
  -H 'Content-Type: application/json' \
  --data '{"expectedName":"Guitar practice","name":"Stale overwrite"}' \
  "http://localhost:8080/v1/paths/$path_id/name"
path_05a_owner_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05a-owner-rename-0001' \
  -H 'Content-Type: application/json' \
  --data '{"expectedName":"Shared practice","name":"Guitar practice"}' \
  "http://localhost:8080/v1/paths/$path_id/name")"
printf '%s' "$path_05a_owner_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Guitar practice" and d["capabilities"]["renamePath"] is True'
path_05a_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_id" -v "owner_id=$owner_id" -v "administrator_id=$administrator_id" <<'PATH_05A_PERSISTENCE_SQL'
SELECT
  (SELECT count(*) FROM path_models WHERE id = :'path_id' AND name = 'Guitar practice'),
  (SELECT count(*) FROM audit_event_models
   WHERE target_type = 'path' AND target_id = :'path_id'
     AND action = 'resource.updated' AND outcome = 'succeeded'
     AND actor_user_id IN (:'owner_id', :'administrator_id')),
  (SELECT count(*) FROM idempotency_models
   WHERE resource_id = :'path_id' AND operation = 'path.rename');
PATH_05A_PERSISTENCE_SQL
)"
[[ "$path_05a_persistence" == '1|2|2' ]] || {
  echo "PATH-05A persistence or replay evidence mismatch: $path_05a_persistence" >&2
  exit 1
}
echo "PATH-05A creator/administrator rename, denial, stale-write, replay, and persistence acceptance passed"

# PATH-05B proves that a manager can see and cancel a pending invitation sent
# by another manager, while ordinary members receive the same opaque denial.
path_05b_recipient="$(curl -fsS \
  -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$path_id/invitation-recipient?username=live.acceptance.stranger")"
printf '%s' "$path_05b_recipient" |
  STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys;assert json.load(sys.stdin)["data"]=={"userId":os.environ["STRANGER_ID"],"username":"live.acceptance.stranger","displayName":"Stranger"}'
path_05b_send_body="$(STRANGER_ID="$stranger_id" python3 -c 'import json,os;print(json.dumps({"username":"live.acceptance.stranger","expectedRecipientUserId":os.environ["STRANGER_ID"],"offeredRole":"supporter"},separators=(",",":")))')"
path_05b_send_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05b-send-key-00000001' \
  -H 'Content-Type: application/json' \
  --data "$path_05b_send_body" \
  "http://localhost:8080/v1/paths/$path_id/invitations")"
path_05b_invitation_id="$(printf '%s' "$path_05b_send_response" |
  PATH_ID="$path_id" OWNER_ID="$owner_id" STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["inviterUserId"]==os.environ["OWNER_ID"] and d["recipientUserId"]==os.environ["STRANGER_ID"] and d["offeredRole"]=="supporter";print(d["id"])')"
[[ -n "$path_05b_invitation_id" ]] || { echo "PATH-05B invitation send did not return an ID" >&2; exit 1; }

curl -fsS -H "Authorization: Bearer $administrator_token" \
  "http://localhost:8080/v1/paths/$path_id/invitations?limit=25" |
  INVITATION_ID="$path_05b_invitation_id" PATH_ID="$path_id" OWNER_ID="$owner_id" STRANGER_ID="$stranger_id" ADMIN_ID="$administrator_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d["meta"]=={} and len(d["data"])==1;item=d["data"][0];inv=item["invitation"];assert inv["id"]==os.environ["INVITATION_ID"] and inv["pathId"]==os.environ["PATH_ID"] and inv["offeredRole"]=="supporter";assert item["inviter"]=={"userId":os.environ["OWNER_ID"],"username":"live.acceptance.owner","displayName":"developer"};assert item["recipient"]=={"userId":os.environ["STRANGER_ID"],"username":"live.acceptance.stranger","displayName":"Stranger"}'
expect_status 404 -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$path_id/invitations?limit=25"
expect_status 404 -X DELETE -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: path-05b-denied-key-0000001' \
  "http://localhost:8080/v1/paths/$path_id/invitations/$path_05b_invitation_id"

path_05b_cancel_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $administrator_token" \
  -H 'Idempotency-Key: path-05b-cancel-key-0000001' \
  "http://localhost:8080/v1/paths/$path_id/invitations/$path_05b_invitation_id")"
printf '%s' "$path_05b_cancel_response" |
  INVITATION_ID="$path_05b_invitation_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["invitationId"]==os.environ["INVITATION_ID"] and d["canceledAt"]'
path_05b_cancel_replay="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $administrator_token" \
  -H 'Idempotency-Key: path-05b-cancel-key-0000001' \
  "http://localhost:8080/v1/paths/$path_id/invitations/$path_05b_invitation_id")"
[[ "$path_05b_cancel_replay" == "$path_05b_cancel_response" ]] || {
  echo "PATH-05B cancellation replay changed the original representation" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $administrator_token" \
  "http://localhost:8080/v1/paths/$path_id/invitations?limit=25" |
  INVITATION_ID="$path_05b_invitation_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert os.environ["INVITATION_ID"] not in {item["invitation"]["id"] for item in d["data"]}'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  INVITATION_ID="$path_05b_invitation_id" python3 -c 'import json,os,sys;assert os.environ["INVITATION_ID"] not in {item["invitation"]["id"] for item in json.load(sys.stdin)["data"]}'
expect_status 404 -X POST -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-05b-accept-key-0000001' \
  "http://localhost:8080/v1/path-invitations/$path_05b_invitation_id/accept"
path_05b_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "invitation_id=$path_05b_invitation_id" -v "administrator_id=$administrator_id" <<'PATH_05B_PERSISTENCE_SQL'
SELECT
  (SELECT count(*) FROM path_invitation_models WHERE id = :'invitation_id' AND canceled_at IS NOT NULL AND accepted_at IS NULL AND rejected_at IS NULL),
  (SELECT count(*) FROM notification_models WHERE path_invitation_id = :'invitation_id' AND deleted_at IS NOT NULL),
  (SELECT count(*) FROM audit_event_models WHERE target_type = 'path_invitation' AND target_id = :'invitation_id' AND action = 'path_invitation.canceled' AND actor_user_id = :'administrator_id' AND outcome = 'succeeded'),
  (SELECT count(*) FROM idempotency_models WHERE principal_id = :'administrator_id' AND operation = 'path.invitation.cancel' AND resource_id = :'invitation_id');
PATH_05B_PERSISTENCE_SQL
)"
[[ "$path_05b_persistence" == '1|1|1|1' ]] || {
  echo "PATH-05B persistence or replay evidence mismatch: $path_05b_persistence" >&2
  exit 1
}
echo "PATH-05B manager list/cancel, member denial, replay, notification cleanup, and terminal acceptance passed"

# PATH-05C proves creator-only visibility management, immediate audience
# convergence, participant information, and replay without changing Path data.
path_05c_activity_before="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "path_id=$path_id" <<'PATH_05C_ACTIVITY_SQL'
SELECT count(*) FROM recorded_activity_models WHERE path_id = :'path_id';
PATH_05C_ACTIVITY_SQL
)"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "owner_id=$owner_id" <<'PATH_05C_PROFILE_SQL'
UPDATE user_models
SET profile_visibility = 'public', updated_at = CURRENT_TIMESTAMP
WHERE id = :'owner_id';
PATH_05C_PROFILE_SQL
curl -fsS -X POST \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-05c-follow-key-000001' \
  "http://localhost:8080/v1/profiles/live.acceptance.owner/follow" >/dev/null
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "owner_id=$owner_id" <<'PATH_05C_PROFILE_SQL'
UPDATE user_models
SET profile_visibility = 'private', updated_at = CURRENT_TIMESTAMP
WHERE id = :'owner_id';
PATH_05C_PROFILE_SQL
expect_status 404 -H "Authorization: Bearer $stranger_token" "http://localhost:8080/v1/paths/$path_id"

for denied_visibility_token in "$administrator_token" "$participant_token" "$supporter_token"; do
  expect_status 404 -X PUT \
    -H "Authorization: Bearer $denied_visibility_token" \
    -H 'Idempotency-Key: path-05c-denied-key-00001' \
    -H 'Content-Type: application/json' \
    --data '{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}' \
    "http://localhost:8080/v1/paths/$path_id/visibility"
done

path_05c_expand_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05c-expand-key-00001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}' \
  "http://localhost:8080/v1/paths/$path_id/visibility")"
printf '%s' "$path_05c_expand_response" |
  PATH_ID="$path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["PATH_ID"] and d["visibility"]=="followers" and d["capabilities"]["manageVisibility"] is True'
path_05c_expand_replay="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05c-expand-key-00001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}' \
  "http://localhost:8080/v1/paths/$path_id/visibility")"
[[ "$path_05c_expand_replay" == "$path_05c_expand_response" ]] || {
  echo "PATH-05C immediate replay changed the original projection" >&2
  exit 1
}
expect_status 409 -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05c-stale-key-000001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}' \
  "http://localhost:8080/v1/paths/$path_id/visibility"

path_05c_expanded_status=''
for _ in $(seq 1 20); do
  path_05c_expanded_status="$(curl -sS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $stranger_token" "http://localhost:8080/v1/paths/$path_id")"
  [[ "$path_05c_expanded_status" == '200' ]] && break
  sleep 1
done
[[ "$path_05c_expanded_status" == '200' ]] || {
  echo "PATH-05C follower did not gain access after visibility expansion" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100' |
  PATH_ID="$path_id" python3 -c 'import json,os,sys;items=[item for item in json.load(sys.stdin)["data"] if item["type"]=="path_visibility_changed" and item["pathId"]==os.environ["PATH_ID"]];assert len(items)==1;item=items[0];assert item["presentation"]=="informational" and item["pathVisibility"]=="followers" and item["pathName"]=="Guitar practice"'

path_05c_narrow_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05c-narrow-key-00001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"followers","visibility":"private"}' \
  "http://localhost:8080/v1/paths/$path_id/visibility")"
printf '%s' "$path_05c_narrow_response" |
  PATH_ID="$path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["PATH_ID"] and d["visibility"]=="private"'
path_05c_narrowed_status=''
for _ in $(seq 1 20); do
  path_05c_narrowed_status="$(curl -sS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $stranger_token" "http://localhost:8080/v1/paths/$path_id")"
  [[ "$path_05c_narrowed_status" == '404' ]] && break
  sleep 1
done
[[ "$path_05c_narrowed_status" == '404' ]] || {
  echo "PATH-05C follower retained access after visibility contraction" >&2
  exit 1
}

# A prior key must replay its original projection even after a later transition,
# without restoring that old audience in durable state.
path_05c_historical_replay="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-05c-expand-key-00001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}' \
  "http://localhost:8080/v1/paths/$path_id/visibility")"
[[ "$path_05c_historical_replay" == "$path_05c_expand_response" ]] || {
  echo "PATH-05C historical replay lost its original projection" >&2
  exit 1
}
path_05c_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_id" -v "owner_id=$owner_id" -v "participant_id=$participant_id" <<'PATH_05C_PERSISTENCE_SQL'
SELECT
  (SELECT count(*) FROM path_models WHERE id = :'path_id' AND visibility = 'private'),
  (SELECT count(*) FROM path_membership_models WHERE path_id = :'path_id'),
  (SELECT count(*) FROM audit_event_models WHERE target_type = 'path' AND target_id = :'path_id' AND action = 'path.visibility_changed' AND actor_user_id = :'owner_id' AND outcome = 'succeeded'),
  (SELECT count(*) FROM idempotency_models WHERE principal_id = :'owner_id' AND operation = 'path.visibility.set' AND resource_id = :'path_id'),
  (SELECT count(*) FROM notification_models WHERE path_id = :'path_id' AND recipient_user_id = :'participant_id' AND kind = 'path_visibility_changed' AND presentation_class = 'informational' AND path_visibility = 'followers'),
  (SELECT count(*) FROM user_models WHERE id = :'owner_id' AND profile_visibility = 'private');
PATH_05C_PERSISTENCE_SQL
)"
[[ "$path_05c_persistence" == '1|3|2|2|1|1' ]] || {
  echo "PATH-05C persistence, audit, replay, or notification evidence mismatch: $path_05c_persistence" >&2
  exit 1
}
path_05c_activity_after="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "path_id=$path_id" <<'PATH_05C_ACTIVITY_SQL'
SELECT count(*) FROM recorded_activity_models WHERE path_id = :'path_id';
PATH_05C_ACTIVITY_SQL
)"
[[ "$path_05c_activity_after" == "$path_05c_activity_before" ]] || {
  echo "PATH-05C visibility management changed recorded activity" >&2
  exit 1
}
echo "PATH-05C creator-only expansion/contraction, history access, notification, replay, and unchanged data acceptance passed"
WEB_ACCEPTANCE_BASE_URL="$web_base_url" \
  WEB_ACCEPTANCE_APPLICATION_TOKEN="$owner_token" \
  WEB_ACCEPTANCE_PARTICIPANT_TOKEN="$participant_token" \
WEB_ACCEPTANCE_PATH_ID="$path_id" \
  WEB_ACCEPTANCE_PATH_NAME='Guitar practice' \
  node scripts/web-path-visibility-acceptance.mjs

# PATH-05C establishes a real follower to prove followers-only audience access.
# Remove that relationship before later private cross-user fixtures reuse the
# same accounts so their unrelated-account assertions remain isolated.
curl -fsS -X DELETE \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-05c-unfollow-key-00001' \
  "http://localhost:8080/v1/profiles/live.acceptance.owner/follow" |
  python3 -c 'import json,sys;profile=json.load(sys.stdin)["data"]["profile"];assert profile["username"]=="live.acceptance.owner" and profile["relationship"]=="none"'

# The first product slice starts with a name-only Path created by the activated
# Dex identity. Replaying the same request must return that one durable Path.
live_path_create_key='live-path-create-key-00000001'
live_path_request='{"name":"Piano practice"}'
live_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_path_create_key" \
  -H 'Content-Type: application/json' \
  --data "$live_path_request" \
  http://localhost:8080/v1/paths)"
live_path_id="$(printf '%s' "$live_path_response" | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Piano practice";print(d["id"])')"
[[ -n "$live_path_id" ]] || { echo "name-only Path creation did not return an ID" >&2; exit 1; }
replayed_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_path_create_key" \
  -H 'Content-Type: application/json' \
  --data "$live_path_request" \
  http://localhost:8080/v1/paths)"
[[ "$replayed_path_response" == "$live_path_response" ]] || {
  echo "duplicate name-only Path creation did not replay the original result" >&2
  exit 1
}
# LIVE_MANUAL_ACTIVITY_CHRONOLOGY_FIXTURE: the later manual-activity scenario
# records two minutes immediately before its request, so this synthetic Path
# and its creator membership must predate that activity. Production timestamps
# are unchanged; this only makes the acceptance fixture chronologically valid.
live_path_chronology_count="$(docker compose exec -T postgres psql -qAt -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$live_path_id" -v "owner_id=$owner_id" <<'LIVE_PATH_CHRONOLOGY_SQL'
BEGIN;
UPDATE path_models
SET created_at = created_at - INTERVAL '3 minutes'
WHERE id = :'path_id' AND owner_user_id = :'owner_id';
UPDATE path_membership_models
SET joined_at = (SELECT created_at FROM path_models WHERE id = :'path_id')
WHERE path_id = :'path_id' AND user_id = :'owner_id' AND role = 'participant';
SELECT count(*)
FROM path_models AS path
JOIN path_membership_models AS membership
  ON membership.path_id = path.id AND membership.user_id = :'owner_id'
WHERE path.id = :'path_id'
  AND membership.joined_at = path.created_at
  AND path.created_at <= CURRENT_TIMESTAMP - INTERVAL '2 minutes 50 seconds';
COMMIT;
LIVE_PATH_CHRONOLOGY_SQL
)"
[[ "$live_path_chronology_count" == 1 ]] || {
  echo "live manual-activity chronology fixture did not update exactly one owner membership" >&2
  exit 1
}
expect_status 409 -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_path_create_key" \
  -H 'Content-Type: application/json' \
  --data '{"name":"Conflicting replay"}' \
  http://localhost:8080/v1/paths

docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "owner_id=$owner_id" -v "path_id=$live_path_id" <<'LIVE_PATH_CREATOR_SQL' | grep -Fx '1|1|1'
SELECT
  (SELECT count(*) FROM path_models
   WHERE id = :'path_id' AND owner_user_id = :'owner_id' AND name = 'Piano practice'),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'owner_id' AND role = 'participant'),
  (SELECT count(*) FROM authorization_outbox_models
   WHERE resource_type = 'path' AND resource_id = :'path_id'
     AND relation = 'creator' AND subject_id = :'owner_id'
     AND completed_at IS NOT NULL AND dead_lettered_at IS NULL);
LIVE_PATH_CREATOR_SQL

# PATH_03_INVITATION_ACCEPTANCE spans a Dex-backed application session, the
# invitation HTTP contract, PostgreSQL atomic persistence, synchronous SpiceDB
# reconciliation, and the recipient's server-authoritative Home projection.
# Replaying both writes must return the original representation without
# duplicating membership, notifications, audit, idempotency, or authorization.
path_03_create_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-03-create-key-00000001' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Invitation acceptance","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
path_03_path_id="$(printf '%s' "$path_03_create_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Invitation acceptance" and d["visibility"]=="private" and d["capabilities"]=={"trackTime":True,"inviteMembers":True,"manageMembers":True,"manageGoals":True,"manageLifecycle":True,"manageVisibility":True,"renamePath":True,"transferOwnership":True,"leavePath":False};print(d["id"])')"
[[ -n "$path_03_path_id" ]] || { echo "PATH-03 Path creation did not return an ID" >&2; exit 1; }

expect_status 204 -X PUT \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Content-Type: application/json' \
  --data '{"provider":"expo","platform":"android","locale":"es","token":"ExponentPushToken[path-03-stranger]"}' \
  'http://localhost:8080/v1/push-installations/path-03-stranger-installation'
expect_status 204 -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Content-Type: application/json' \
  --data '{"provider":"expo","platform":"ios","locale":"en","token":"ExponentPushToken[path-03-owner]"}' \
  'http://localhost:8080/v1/push-installations/path-03-owner-installation'

path_03_recipient="$(curl -fsS \
  -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id/invitation-recipient?username=live.acceptance.stranger")"
printf '%s' "$path_03_recipient" |
  STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"userId":os.environ["STRANGER_ID"],"username":"live.acceptance.stranger","displayName":"Stranger"}'

path_03_send_body="$(STRANGER_ID="$stranger_id" python3 -c 'import json,os;print(json.dumps({"username":"live.acceptance.stranger","expectedRecipientUserId":os.environ["STRANGER_ID"],"offeredRole":"participant"},separators=(",",":")))')"
path_03_send_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-03-send-key-000000001' \
  -H 'Content-Type: application/json' \
  --data "$path_03_send_body" \
  "http://localhost:8080/v1/paths/$path_03_path_id/invitations")"
path_03_invitation_id="$(printf '%s' "$path_03_send_response" |
  PATH_ID="$path_03_path_id" OWNER_ID="$owner_id" STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["inviterUserId"]==os.environ["OWNER_ID"] and d["recipientUserId"]==os.environ["STRANGER_ID"] and d["offeredRole"]=="participant" and "acceptedAt" not in d;print(d["id"])')"
[[ -n "$path_03_invitation_id" ]] || { echo "PATH-03 invitation send did not return an ID" >&2; exit 1; }
path_03_replayed_send="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-03-send-key-000000001' \
  -H 'Content-Type: application/json' \
  --data "$path_03_send_body" \
  "http://localhost:8080/v1/paths/$path_03_path_id/invitations")"
[[ "$path_03_replayed_send" == "$path_03_send_response" ]] || {
  echo "PATH-03 invitation send replay changed the original representation" >&2
  exit 1
}

curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  INVITATION_ID="$path_03_invitation_id" PATH_ID="$path_03_path_id" OWNER_ID="$owner_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d["meta"]=={} and len(d["data"])==1;item=d["data"][0];invitation=item["invitation"];assert invitation["id"]==os.environ["INVITATION_ID"] and invitation["pathId"]==os.environ["PATH_ID"] and invitation["offeredRole"]=="participant" and "acceptedAt" not in invitation;assert item["pathName"]=="Invitation acceptance" and item["inviter"]=={"userId":os.environ["OWNER_ID"],"username":"live.acceptance.owner","displayName":"developer"}'
path_03_received_notification="$(curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/notifications?limit=25')"
path_03_received_notification_id="$(printf '%s' "$path_03_received_notification" |
  INVITATION_ID="$path_03_invitation_id" PATH_ID="$path_03_path_id" OWNER_ID="$owner_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);items=[item for item in d["data"] if item.get("invitationId")==os.environ["INVITATION_ID"]];assert type(d["meta"]["unreadCount"]) is int and d["meta"]["unreadCount"]>=1 and len(items)==1;item=items[0];assert item["type"]=="path_invitation_received" and item["presentation"]=="actionable" and item["read"] is False and item["pathId"]==os.environ["PATH_ID"] and item["pathName"]=="Invitation acceptance" and item["offeredRole"]=="participant" and item["actor"]=={"userId":os.environ["OWNER_ID"],"username":"live.acceptance.owner","displayName":"developer"};print(item["id"])')"
wait_for_push "$path_03_received_notification_id" |
  NOTIFICATION_ID="$path_03_received_notification_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d=={"to":"ExponentPushToken[path-03-stranger]","title":"Invitación a un camino","body":"developer te invitó a unirte a Invitation acceptance.","priority":"default","data":{"version":"1","notificationId":os.environ["NOTIFICATION_ID"]}}'
curl -fsS -H "Authorization: Bearer $stranger_second_device_token" \
  'http://localhost:8080/v1/notifications?limit=1' |
  NOTIFICATION_ID="$path_03_received_notification_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d["meta"]=={"unreadCount":1} and len(d["data"])==1 and d["data"][0]["id"]==os.environ["NOTIFICATION_ID"] and d["data"][0]["read"] is False'
WEB_ACCEPTANCE_BASE_URL="$web_base_url" \
  WEB_ACCEPTANCE_API_URL="$api_base_url" \
  WEB_ACCEPTANCE_APPLICATION_TOKEN="$stranger_token" \
  WEB_ACCEPTANCE_NOTIFICATION_ID="$path_03_received_notification_id" \
  WEB_ACCEPTANCE_INVITATION_ID="$path_03_invitation_id" \
  node scripts/web-notification-convergence-acceptance.mjs
curl -fsS -H "Authorization: Bearer $stranger_second_device_token" \
  'http://localhost:8080/v1/notifications?limit=1' |
  python3 -c 'import json,sys;d=json.load(sys.stdin);assert d["meta"]=={"unreadCount":0} and d["data"]==[]'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/paths?limit=25' |
  PATH_ID="$path_03_path_id" python3 -c 'import json,os,sys;assert os.environ["PATH_ID"] not in {p["id"] for p in json.load(sys.stdin)["data"]}'

path_03_accept_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-03-accept-key-0000001' \
  "http://localhost:8080/v1/path-invitations/$path_03_invitation_id/accept")"
printf '%s' "$path_03_accept_response" |
  INVITATION_ID="$path_03_invitation_id" PATH_ID="$path_03_path_id" STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["INVITATION_ID"] and d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["STRANGER_ID"] and d["offeredRole"]=="participant" and d["acceptedAt"]'
path_03_replayed_accept="$(curl -fsS -X POST \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-03-accept-key-0000001' \
  "http://localhost:8080/v1/path-invitations/$path_03_invitation_id/accept")"
[[ "$path_03_replayed_accept" == "$path_03_accept_response" ]] || {
  echo "PATH-03 invitation acceptance replay changed the original representation" >&2
  exit 1
}

curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  INVITATION_ID="$path_03_invitation_id" python3 -c 'import json,os,sys;assert os.environ["INVITATION_ID"] not in {i["invitation"]["id"] for i in json.load(sys.stdin)["data"]}'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id" |
  PATH_ID="$path_03_path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"id":os.environ["PATH_ID"],"name":"Invitation acceptance","visibility":"private","capabilities":{"trackTime":True,"inviteMembers":False,"manageMembers":False,"manageGoals":False,"manageLifecycle":False,"manageVisibility":False,"renamePath":False,"transferOwnership":False,"leavePath":True},"home":{"classification":"shared","pinned":False}}'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/paths?limit=25' |
  PATH_ID="$path_03_path_id" python3 -c 'import json,os,sys;items={p["id"]:p for p in json.load(sys.stdin)["data"]};assert items[os.environ["PATH_ID"]]=={"id":os.environ["PATH_ID"],"name":"Invitation acceptance","visibility":"private","capabilities":{"trackTime":True,"inviteMembers":False,"manageMembers":False,"manageGoals":False,"manageLifecycle":False,"manageVisibility":False,"renamePath":False,"transferOwnership":False,"leavePath":True},"home":{"classification":"shared","pinned":False}}'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id/timer" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["running"] is False and "timer" not in d'
path_03_accepted_notification="$(curl -fsS -H "Authorization: Bearer $owner_token" \
  'http://localhost:8080/v1/notifications?limit=25')"
path_03_accepted_notification_id="$(printf '%s' "$path_03_accepted_notification" |
  INVITATION_ID="$path_03_invitation_id" PATH_ID="$path_03_path_id" STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);items=[item for item in d["data"] if item.get("invitationId")==os.environ["INVITATION_ID"]];assert type(d["meta"]["unreadCount"]) is int and d["meta"]["unreadCount"]>=1 and len(items)==1;item=items[0];assert item["type"]=="path_invitation_accepted" and item["presentation"]=="informational" and item["read"] is False and item["pathId"]==os.environ["PATH_ID"] and item["pathName"]=="Invitation acceptance" and item["offeredRole"]=="participant" and item["actor"]=={"userId":os.environ["STRANGER_ID"],"username":"live.acceptance.stranger","displayName":"Stranger"};print(item["id"])')"
wait_for_push "$path_03_accepted_notification_id" |
  NOTIFICATION_ID="$path_03_accepted_notification_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d=={"to":"ExponentPushToken[path-03-owner]","title":"Invitation accepted","body":"Stranger accepted your invitation to Invitation acceptance.","priority":"normal","data":{"version":"1","notificationId":os.environ["NOTIFICATION_ID"]}}'

path_03_persistence_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_03_path_id" -v "invitation_id=$path_03_invitation_id" \
  -v "owner_id=$owner_id" -v "recipient_id=$stranger_id" <<'PATH_03_PERSISTENCE_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_invitation_models
   WHERE id = :'invitation_id' AND path_id = :'path_id'
     AND inviter_user_id = :'owner_id' AND recipient_user_id = :'recipient_id'
     AND offered_role = 'participant' AND accepted_at IS NOT NULL
     AND authorization_change_id IS NOT NULL),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'recipient_id' AND role = 'participant'),
  (SELECT count(*) FROM authorization_outbox_models
   WHERE resource_type = 'path' AND resource_id = :'path_id'
     AND relation = 'participant' AND subject_id = :'recipient_id'
     AND operation = 'touch' AND completed_at IS NOT NULL
     AND dead_lettered_at IS NULL),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'owner_id' AND operation = 'path.invitation.send'
     AND resource_id = :'invitation_id'),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'recipient_id' AND operation = 'path.invitation.accept'
     AND resource_id = :'invitation_id'),
  (SELECT count(*) FROM notification_models
   WHERE path_invitation_id = :'invitation_id' AND kind = 'path_invitation_received'
     AND recipient_user_id = :'recipient_id' AND actor_user_id = :'owner_id'
     AND presentation_class = 'actionable' AND offered_role = 'participant'),
  (SELECT count(*) FROM notification_models
   WHERE path_invitation_id = :'invitation_id' AND kind = 'path_invitation_accepted'
     AND recipient_user_id = :'owner_id' AND actor_user_id = :'recipient_id'
     AND presentation_class = 'informational' AND offered_role = 'participant'),
  (SELECT count(*) FROM notification_push_outbox_models
   WHERE notification_id IN (
     SELECT id FROM notification_models WHERE path_invitation_id = :'invitation_id'
   )),
  (SELECT count(*) FROM push_installation_models
   WHERE id IN ('path-03-stranger-installation', 'path-03-owner-installation')
     AND deleted_at IS NULL AND token_ciphertext IS NOT NULL
     AND token_nonce IS NOT NULL AND token_hash IS NOT NULL),
  (SELECT count(*) FROM notification_push_delivery_models
   WHERE notification_id IN (
     SELECT id FROM notification_models WHERE path_invitation_id = :'invitation_id'
   ) AND provider_ticket <> '' AND attempts = 1
     AND delivered_at IS NULL AND suppressed_at IS NULL
     AND permanently_failed_at IS NULL),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'resource.updated' AND target_type = 'push_installation'
     AND target_id IN ('path-03-stranger-installation', 'path-03-owner-installation')
     AND outcome = 'succeeded'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'path_invitation.created' AND actor_user_id = :'owner_id'
     AND target_id = :'invitation_id' AND outcome = 'succeeded'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'path_invitation.accepted' AND actor_user_id = :'recipient_id'
     AND target_id = :'invitation_id' AND outcome = 'succeeded');
PATH_03_PERSISTENCE_EVIDENCE
)"
[[ "$path_03_persistence_evidence" == '1|1|1|1|1|1|1|2|2|2|2|1|1' ]] || {
  echo "PATH-03 replay or persistence mismatch: expected=1|1|1|1|1|1|1|2|2|2|2|1|1 actual=$path_03_persistence_evidence" >&2
  exit 1
}
echo "PATH-03 invitation send, localized push delivery, user-visible notices, recipient Home projection, exact participant capability, and replay persistence acceptance passed"

# SOC_07_NUDGES proves the complete preset-only encouragement boundary across
# Path-scoped audience policy, rate limiting, ordinary notification history,
# localized push delivery, notification-channel suppression, and exact replay.
curl -fsS -H "Authorization: Bearer $stranger_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id/nudge-preference" |
  PATH_ID="$path_03_path_id" STRANGER_ID="$stranger_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);assert d["data"]=={"pathId":os.environ["PATH_ID"],"userId":os.environ["STRANGER_ID"],"audience":"path_members","revision":0}'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/me/notification-channels/nudges' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"channel":"nudges","enabled":True,"revision":0}'
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudge-eligibility" |
  PATH_ID="$path_03_path_id" STRANGER_ID="$stranger_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)["data"]=={"pathId":os.environ["PATH_ID"],"recipientUserId":os.environ["STRANGER_ID"],"eligible":True}'

soc_07_send_body='{"content":{"kind":"preset","preset":"keep_it_going"}}'
soc_07_send_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc-07-send-key-000000001' \
  -H 'Content-Type: application/json' \
  --data "$soc_07_send_body" \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudges")"
soc_07_nudge_id="$(printf '%s' "$soc_07_send_response" |
  PATH_ID="$path_03_path_id" OWNER_ID="$owner_id" STRANGER_ID="$stranger_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["senderUserId"]==os.environ["OWNER_ID"] and d["recipientUserId"]==os.environ["STRANGER_ID"] and d["pathId"]==os.environ["PATH_ID"] and d["content"]=={"kind":"preset","preset":"keep_it_going"} and d["sentAt"];print(d["id"])')"
soc_07_replayed_send="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc-07-send-key-000000001' \
  -H 'Content-Type: application/json' \
  --data "$soc_07_send_body" \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudges")"
[[ "$soc_07_replayed_send" == "$soc_07_send_response" ]] || {
  echo "SOC-07 replay changed the original nudge representation" >&2
  exit 1
}

soc_07_notification_id="$(curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/notifications?limit=25' |
  PATH_ID="$path_03_path_id" OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=[i for i in d["data"] if i["type"]=="nudge_received" and i.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1 and d["meta"]["unreadCount"]>=1;i=items[0];assert i["presentation"]=="informational" and i["read"] is False and i["pathName"]=="Invitation acceptance" and i["actor"]=={"userId":os.environ["OWNER_ID"],"username":"live.acceptance.owner","displayName":"developer"} and i["content"]=={"kind":"preset","preset":"keep_it_going"} and "nudgeId" not in i;print(i["id"])')"
wait_for_push "$soc_07_notification_id" |
  NOTIFICATION_ID="$soc_07_notification_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)=={"to":"ExponentPushToken[path-03-stranger]","title":"Nuevo ánimo","body":"developer te animó en Invitation acceptance: «¡Sigue así!»","priority":"normal","data":{"version":"1","notificationId":os.environ["NOTIFICATION_ID"]}}'

curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudge-eligibility" |
  PATH_ID="$path_03_path_id" STRANGER_ID="$stranger_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)["data"]=={"pathId":os.environ["PATH_ID"],"recipientUserId":os.environ["STRANGER_ID"],"eligible":False,"reason":"rate_limited"}'
expect_status 429 -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc-07-send-key-000000002' \
  -H 'Content-Type: application/json' \
  --data "$soc_07_send_body" \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudges"
expect_status 422 -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc-07-custom-key-00000001' \
  -H 'Content-Type: application/json' \
  --data '{"content":{"kind":"custom","preset":"keep_it_going","text":"hurry"}}' \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudges"

curl -fsS -X PUT \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: soc-07-audience-key-0000001' \
  -H 'Content-Type: application/json' \
  --data '{"audience":"nobody","expectedRevision":0}' \
  "http://localhost:8080/v1/paths/$path_03_path_id/nudge-preference" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["audience"]=="nobody" and d["revision"]==1'
expect_status 404 -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$stranger_id/nudge-eligibility"
curl -fsS -X PUT \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: soc-07-audience-key-0000002' \
  -H 'Content-Type: application/json' \
  --data '{"audience":"path_members","expectedRevision":1}' \
  "http://localhost:8080/v1/paths/$path_03_path_id/nudge-preference" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["audience"]=="path_members" and d["revision"]==2'

curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc-07-channel-key-00000001' \
  -H 'Content-Type: application/json' \
  --data '{"enabled":false,"expectedRevision":0}' \
  'http://localhost:8080/v1/me/notification-channels/nudges' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"channel":"nudges","enabled":False,"revision":1}'
soc_07_suppressed_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: soc-07-send-key-000000003' \
  -H 'Content-Type: application/json' \
  --data '{"content":{"kind":"preset","preset":"you_have_got_this"}}' \
  "http://localhost:8080/v1/paths/$path_03_path_id/members/$owner_id/nudges")"
printf '%s' "$soc_07_suppressed_response" |
  PATH_ID="$path_03_path_id" OWNER_ID="$owner_id" STRANGER_ID="$stranger_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["senderUserId"]==os.environ["STRANGER_ID"] and d["recipientUserId"]==os.environ["OWNER_ID"] and d["pathId"]==os.environ["PATH_ID"] and d["content"]=={"kind":"preset","preset":"you_have_got_this"}'
curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc-07-channel-key-00000002' \
  -H 'Content-Type: application/json' \
  --data '{"enabled":true,"expectedRevision":1}' \
  'http://localhost:8080/v1/me/notification-channels/nudges' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"channel":"nudges","enabled":True,"revision":2}'

soc_07_persistence_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_03_path_id" -v "owner_id=$owner_id" -v "recipient_id=$stranger_id" \
  -v "nudge_id=$soc_07_nudge_id" <<'SOC_07_PERSISTENCE_EVIDENCE'
SELECT
  (SELECT count(*) FROM social_nudge_models WHERE path_id = :'path_id'),
  (SELECT count(*) FROM social_nudge_replay_models WHERE path_id = :'path_id'),
  (SELECT count(*) FROM notification_models WHERE nudge_id = :'nudge_id' AND recipient_user_id = :'recipient_id' AND kind = 'nudge_received' AND channel = 'nudges'),
  (SELECT count(*) FROM notification_models WHERE path_id = :'path_id' AND recipient_user_id = :'owner_id' AND kind = 'nudge_received'),
  (SELECT count(*) FROM notification_push_outbox_models WHERE notification_id IN (SELECT id FROM notification_models WHERE nudge_id = :'nudge_id')),
  (SELECT count(*) FROM notification_push_delivery_models WHERE notification_id IN (SELECT id FROM notification_models WHERE nudge_id = :'nudge_id') AND attempts = 1),
  (SELECT count(*) FROM path_nudge_preference_replay_models WHERE actor_user_id = :'recipient_id' AND path_id = :'path_id'),
  (SELECT revision FROM path_nudge_preference_models WHERE user_id = :'recipient_id' AND path_id = :'path_id'),
  (SELECT count(*) FROM notification_channel_preference_replay_models WHERE actor_user_id = :'owner_id' AND channel = 'nudges'),
  (SELECT revision FROM notification_channel_preference_models WHERE user_id = :'owner_id' AND channel = 'nudges'),
  (SELECT enabled FROM notification_channel_preference_models WHERE user_id = :'owner_id' AND channel = 'nudges'),
  (SELECT count(*) FROM audit_event_models WHERE action = 'resource.created' AND target_type = 'nudge' AND actor_user_id IN (:'owner_id', :'recipient_id'));
SOC_07_PERSISTENCE_EVIDENCE
)"
[[ "$soc_07_persistence_evidence" == '2|2|1|0|1|1|2|2|2|2|t|2' ]] || {
  echo "SOC-07 persistence or suppression mismatch: expected=2|2|1|0|1|1|2|2|2|2|t|2 actual=$soc_07_persistence_evidence" >&2
  exit 1
}
echo "SOC-07 preset nudge, audience policy, rate limit, localized delivery, channel suppression, and replay acceptance passed"

# PATH_04_PRIVATE_PROFILE_WARNING proves that the warning is projected from
# current server state and that confirmation is part of the atomic invitation
# acceptance boundary, including the SpiceDB relationship written on success.
path_04_create_path() {
  local name="$1" visibility="$2" key="$3" response
  response="$(curl -fsS -X POST \
    -H "Authorization: Bearer $administrator_token" \
    -H "Idempotency-Key: $key" \
    -H 'Content-Type: application/json' \
    --data "{\"name\":\"$name\",\"visibility\":\"$visibility\"}" \
    http://localhost:8080/v1/paths)"
  printf '%s' "$response" |
    EXPECTED_NAME="$name" EXPECTED_VISIBILITY="$visibility" python3 -c \
      'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["name"]==os.environ["EXPECTED_NAME"] and d["visibility"]==os.environ["EXPECTED_VISIBILITY"];print(d["id"])'
}

path_04_send_invitation() {
  local path_id="$1" username="$2" recipient_id="$3" offered_role="$4" key="$5" response
  response="$(curl -fsS -X POST \
    -H "Authorization: Bearer $administrator_token" \
    -H "Idempotency-Key: $key" \
    -H 'Content-Type: application/json' \
    --data "{\"username\":\"$username\",\"expectedRecipientUserId\":\"$recipient_id\",\"offeredRole\":\"$offered_role\"}" \
    "http://localhost:8080/v1/paths/$path_id/invitations")"
  printf '%s' "$response" |
    PATH_ID="$path_id" RECIPIENT_ID="$recipient_id" OFFERED_ROLE="$offered_role" python3 -c \
      'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["RECIPIENT_ID"] and d["offeredRole"]==os.environ["OFFERED_ROLE"] and "acceptedAt" not in d;print(d["id"])'
}

# Use a dedicated public-profile Path owner so every public Path and the
# followers-to-public transition below remain valid under the profile ceiling.
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "user_id=$administrator_id" <<'PATH_04_PUBLIC_OWNER'
UPDATE user_models SET profile_visibility = 'public', updated_at = CURRENT_TIMESTAMP
WHERE id = :'user_id';
PATH_04_PUBLIC_OWNER

# PATH_04_BROWSER_WARNING exercises the same current-visibility confirmation
# through the real localized Playwright and Dex web boundary. The recipient's
# private profile makes this participant invitation warning-eligible.
path_04_browser_path_id="$(path_04_create_path \
  'Browser visibility warning' followers path-04-browser-create-key-001)"
path_04_browser_invitation_id="$(path_04_send_invitation \
  "$path_04_browser_path_id" live.acceptance.owner "$owner_id" participant \
  path-04-browser-send-key-0001)"
WEB_ACCEPTANCE_BASE_URL="$web_base_url" \
  WEB_ACCEPTANCE_API_URL="$api_base_url" \
  WEB_ACCEPTANCE_DEX_ORIGIN="${dex_public_issuer%/dex}" \
  WEB_ACCEPTANCE_INVITATION_ID="$path_04_browser_invitation_id" \
  WEB_ACCEPTANCE_INVITATION_PATH_NAME='Browser visibility warning' \
  node scripts/web-browser-acceptance.mjs
path_04_browser_acceptance_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_04_browser_path_id" -v "invitation_id=$path_04_browser_invitation_id" \
  -v "recipient_id=$owner_id" <<'PATH_04_BROWSER_ACCEPTANCE_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_invitation_models
   WHERE id = :'invitation_id' AND accepted_at IS NOT NULL),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'recipient_id' AND role = 'participant'),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'recipient_id' AND operation = 'path.invitation.accept'
     AND resource_id = :'invitation_id'),
  (SELECT count(*) FROM authorization_outbox_models
   WHERE resource_type = 'path' AND resource_id = :'path_id'
     AND relation = 'participant' AND subject_id = :'recipient_id'
     AND completed_at IS NOT NULL AND dead_lettered_at IS NULL),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'path_invitation.accepted' AND actor_user_id = :'recipient_id'
     AND target_id = :'invitation_id' AND outcome = 'succeeded');
PATH_04_BROWSER_ACCEPTANCE_EVIDENCE
)"
[[ "$path_04_browser_acceptance_evidence" == '1|1|1|1|1' ]] || {
  echo "PATH-04 browser acceptance persistence mismatch: $path_04_browser_acceptance_evidence" >&2
  exit 1
}
echo "browser Dex visibility-warning acceptance persistence passed"

path_04_warning_path_id="$(path_04_create_path \
  'Current visibility warning' followers path-04-warning-create-key-001)"
path_04_warning_invitation_id="$(path_04_send_invitation \
  "$path_04_warning_path_id" live.acceptance.stranger "$stranger_id" participant \
  path-04-warning-send-key-0001)"

path_04_warning_context_before_change="$(curl -fsS \
  -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/path-invitations?limit=25')"
printf '%s' "$path_04_warning_context_before_change" |
  INVITATION_ID="$path_04_warning_invitation_id" python3 -c \
    'import json,os,sys;matches=[item for item in json.load(sys.stdin)["data"] if item["invitation"]["id"]==os.environ["INVITATION_ID"]];assert len(matches)==1 and matches[0]["warning"]=={"pathVisibility":"followers","hasRetainedActivity":False}'

# Model a visibility change between review and confirmation. The stale
# acknowledgement below must not authorize the now-public Path.
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$path_04_warning_path_id" <<'PATH_04_CURRENT_VISIBILITY_CHANGE'
UPDATE path_models SET visibility = 'public', updated_at = CURRENT_TIMESTAMP
WHERE id = :'path_id';
PATH_04_CURRENT_VISIBILITY_CHANGE
path_04_warning_context_after_change="$(curl -fsS \
  -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/path-invitations?limit=25')"
printf '%s' "$path_04_warning_context_after_change" |
  INVITATION_ID="$path_04_warning_invitation_id" python3 -c \
    'import json,os,sys;matches=[item for item in json.load(sys.stdin)["data"] if item["invitation"]["id"]==os.environ["INVITATION_ID"]];assert len(matches)==1 and matches[0]["warning"]=={"pathVisibility":"public","hasRetainedActivity":False}'

expect_status 404 -H "Authorization: Bearer $stranger_token" \
  "http://localhost:8080/v1/paths/$path_04_warning_path_id"
path_04_missing_warning_response="$(curl -sS -X POST -w $'\\n%{http_code}' \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-04-warning-missing-key-01' \
  "http://localhost:8080/v1/path-invitations/$path_04_warning_invitation_id/accept")"
[[ "${path_04_missing_warning_response##*$'\n'}" == '409' ]] || {
  echo "PATH-04 missing acknowledgement did not fail with 409" >&2
  exit 1
}
printf '%s' "${path_04_missing_warning_response%$'\n'*}" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["code"]=="invitation_warning_required"'

path_04_stale_warning_response="$(curl -sS -X POST -w $'\\n%{http_code}' \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-04-warning-stale-key-0001' \
  -H 'Content-Type: application/json' \
  --data '{"visibilityWarningAcknowledgement":{"pathVisibility":"followers"}}' \
  "http://localhost:8080/v1/path-invitations/$path_04_warning_invitation_id/accept")"
[[ "${path_04_stale_warning_response##*$'\n'}" == '409' ]] || {
  echo "PATH-04 stale acknowledgement did not fail with 409" >&2
  exit 1
}
printf '%s' "${path_04_stale_warning_response%$'\n'*}" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["code"]=="invitation_warning_required"'

path_04_failed_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_04_warning_path_id" -v "invitation_id=$path_04_warning_invitation_id" \
  -v "recipient_id=$stranger_id" <<'PATH_04_FAILED_ACCEPTANCE_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_invitation_models
   WHERE id = :'invitation_id' AND accepted_at IS NULL),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'recipient_id'),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'recipient_id' AND operation = 'path.invitation.accept'
     AND resource_id = :'invitation_id'),
  (SELECT count(*) FROM notification_models
   WHERE path_invitation_id = :'invitation_id' AND kind = 'path_invitation_accepted'),
  (SELECT count(*) FROM authorization_outbox_models
   WHERE resource_type = 'path' AND resource_id = :'path_id'
     AND relation = 'participant' AND subject_id = :'recipient_id'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'path_invitation.accepted' AND actor_user_id = :'recipient_id'
     AND target_id = :'invitation_id');
PATH_04_FAILED_ACCEPTANCE_EVIDENCE
)"
[[ "$path_04_failed_evidence" == '1|0|0|0|0|0' ]] || {
  echo "PATH-04 failed confirmation changed durable state: $path_04_failed_evidence" >&2
  exit 1
}
expect_status 404 -H "Authorization: Bearer $stranger_token" \
  "http://localhost:8080/v1/paths/$path_04_warning_path_id"

curl -fsS -X POST \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-04-warning-confirm-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"visibilityWarningAcknowledgement":{"pathVisibility":"public"}}' \
  "http://localhost:8080/v1/path-invitations/$path_04_warning_invitation_id/accept" |
  INVITATION_ID="$path_04_warning_invitation_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["INVITATION_ID"] and d["offeredRole"]=="participant" and d["acceptedAt"]'
expect_status 200 -H "Authorization: Bearer $stranger_token" \
  "http://localhost:8080/v1/paths/$path_04_warning_path_id"

# PATH_04_CONTROL_INVITATIONS: role, Path visibility, and profile visibility
# independently bypass the warning exactly as specified.
path_04_supporter_path_id="$(path_04_create_path \
  'Supporter warning control' public path-04-supporter-create-key-01)"
path_04_supporter_invitation_id="$(path_04_send_invitation \
  "$path_04_supporter_path_id" live.acceptance.supporter "$supporter_id" supporter \
  path-04-supporter-send-key-001)"
path_04_private_path_id="$(path_04_create_path \
  'Private Path warning control' private path-04-private-create-key-0001)"
path_04_private_path_invitation_id="$(path_04_send_invitation \
  "$path_04_private_path_id" live.acceptance.participant "$participant_id" participant \
  path-04-private-send-key-00001)"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "user_id=$stranger_id" <<'PATH_04_PUBLIC_PROFILE'
UPDATE user_models SET profile_visibility = 'public', updated_at = CURRENT_TIMESTAMP
WHERE id = :'user_id';
PATH_04_PUBLIC_PROFILE
path_04_public_profile_path_id="$(path_04_create_path \
  'Public profile warning control' public path-04-public-profile-create-01)"
path_04_public_profile_invitation_id="$(path_04_send_invitation \
  "$path_04_public_profile_path_id" live.acceptance.stranger "$stranger_id" participant \
  path-04-public-profile-send-0001)"

path_04_control_invitations="$(curl -fsS \
  -H "Authorization: Bearer $supporter_token" \
  'http://localhost:8080/v1/path-invitations?limit=25')"
printf '%s' "$path_04_control_invitations" |
  SUPPORTER_INVITATION_ID="$path_04_supporter_invitation_id" python3 -c \
    'import json,os,sys;matches={item["invitation"]["id"]:item for item in json.load(sys.stdin)["data"]};assert "warning" not in matches[os.environ["SUPPORTER_INVITATION_ID"]]'
curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  PRIVATE_PATH_INVITATION_ID="$path_04_private_path_invitation_id" python3 -c \
    'import json,os,sys;matches={item["invitation"]["id"]:item for item in json.load(sys.stdin)["data"]};assert "warning" not in matches[os.environ["PRIVATE_PATH_INVITATION_ID"]]'
curl -fsS -H "Authorization: Bearer $stranger_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  PUBLIC_PROFILE_INVITATION_ID="$path_04_public_profile_invitation_id" python3 -c \
    'import json,os,sys;matches={item["invitation"]["id"]:item for item in json.load(sys.stdin)["data"]};assert "warning" not in matches[os.environ["PUBLIC_PROFILE_INVITATION_ID"]]'

# PATH_04_RETAINED_ACTIVITY: a prior activity remains hidden without membership,
# is disclosed in the warning context, and becomes visible again only after the
# private-profile participant confirms the current Path audience.
path_04_retained_path_id="$(path_04_create_path \
  'Retained activity rejoin' followers path-04-retained-create-key-001)"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$path_04_retained_path_id" -v "participant_id=$participant_id" <<'PATH_04_RETAINED_ACTIVITY'
INSERT INTO recorded_activity_models (
  id, path_id, participant_id, started_at, ended_at, occurrence_time_zone,
  created_at, updated_at, note
) VALUES (
  'path-04-retained-activity', :'path_id', :'participant_id',
  CURRENT_TIMESTAMP - INTERVAL '2 hours',
  CURRENT_TIMESTAMP - INTERVAL '1 hour', 'America/New_York',
  CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '1 hour',
  'retained private note'
);
PATH_04_RETAINED_ACTIVITY
expect_status 404 -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$path_04_retained_path_id/activities?limit=25"
path_04_retained_invitation_id="$(path_04_send_invitation \
  "$path_04_retained_path_id" live.acceptance.participant "$participant_id" participant \
  path-04-retained-send-key-0001)"
curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  INVITATION_ID="$path_04_retained_invitation_id" python3 -c \
    'import json,os,sys;matches=[item for item in json.load(sys.stdin)["data"] if item["invitation"]["id"]==os.environ["INVITATION_ID"]];assert len(matches)==1 and matches[0]["warning"]=={"pathVisibility":"followers","hasRetainedActivity":True}'
curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: path-04-retained-confirm-key-1' \
  -H 'Content-Type: application/json' \
  --data '{"visibilityWarningAcknowledgement":{"pathVisibility":"followers"}}' \
  "http://localhost:8080/v1/path-invitations/$path_04_retained_invitation_id/accept" |
  INVITATION_ID="$path_04_retained_invitation_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["INVITATION_ID"] and d["acceptedAt"]'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$path_04_retained_path_id/activities?limit=25" |
  python3 -c 'import json,sys;matches=[item for item in json.load(sys.stdin)["data"] if item["activity"]["id"]=="path-04-retained-activity"];assert len(matches)==1 and matches[0]["activity"]["note"]=="retained private note"'

path_04_accepted_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "warning_path_id=$path_04_warning_path_id" \
  -v "warning_invitation_id=$path_04_warning_invitation_id" \
  -v "retained_path_id=$path_04_retained_path_id" \
  -v "retained_invitation_id=$path_04_retained_invitation_id" \
  -v "warning_recipient_id=$stranger_id" -v "retained_recipient_id=$participant_id" \
  <<'PATH_04_ACCEPTED_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_invitation_models
   WHERE id = :'warning_invitation_id' AND accepted_at IS NOT NULL),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'warning_path_id' AND user_id = :'warning_recipient_id'
     AND role = 'participant'),
  (SELECT count(*) FROM authorization_outbox_models
   WHERE resource_type = 'path' AND resource_id = :'warning_path_id'
     AND relation = 'participant' AND subject_id = :'warning_recipient_id'
     AND completed_at IS NOT NULL AND dead_lettered_at IS NULL),
  (SELECT count(*) FROM path_invitation_models
   WHERE id = :'retained_invitation_id' AND accepted_at IS NOT NULL),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'retained_path_id' AND user_id = :'retained_recipient_id'
     AND role = 'participant'),
  (SELECT count(*) FROM recorded_activity_models
   WHERE id = 'path-04-retained-activity' AND path_id = :'retained_path_id'
     AND participant_id = :'retained_recipient_id'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'path_invitation.accepted'
     AND target_id IN (:'warning_invitation_id', :'retained_invitation_id')
     AND outcome = 'succeeded');
PATH_04_ACCEPTED_EVIDENCE
)"
[[ "$path_04_accepted_evidence" == '1|1|1|1|1|1|2' ]] || {
  echo "PATH-04 accepted persistence mismatch: $path_04_accepted_evidence" >&2
  exit 1
}
echo "PATH-04 current-visibility warning, rollback, controls, atomic acceptance, and retained-activity rejoin acceptance passed"

# PATH-06 rejects one still-pending control invitation through the recipient
# boundary, including opaque cross-user denial, replay, and zero access grants.
expect_status 404 -X POST \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: path-06-wrong-user-key-01' \
  "http://localhost:8080/v1/path-invitations/$path_04_supporter_invitation_id/reject"
path_06_reject_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: path-06-reject-key-00001' \
  "http://localhost:8080/v1/path-invitations/$path_04_supporter_invitation_id/reject")"
printf '%s' "$path_06_reject_response" |
  INVITATION_ID="$path_04_supporter_invitation_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["invitationId"]==os.environ["INVITATION_ID"] and d["rejectedAt"] and isinstance(d["unreadCount"],int) and d["unreadCount"]>=0'
path_06_replay_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: path-06-reject-key-00001' \
  "http://localhost:8080/v1/path-invitations/$path_04_supporter_invitation_id/reject")"
[[ "$path_06_replay_response" == "$path_06_reject_response" ]] || {
  echo "PATH-06 same-key rejection replay changed the representation" >&2
  exit 1
}
expect_status 404 -X POST \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: path-06-new-key-0000001' \
  "http://localhost:8080/v1/path-invitations/$path_04_supporter_invitation_id/reject"
curl -fsS -H "Authorization: Bearer $supporter_token" \
  'http://localhost:8080/v1/path-invitations?limit=25' |
  INVITATION_ID="$path_04_supporter_invitation_id" python3 -c \
    'import json,os,sys;assert os.environ["INVITATION_ID"] not in {item["invitation"]["id"] for item in json.load(sys.stdin)["data"]}'
path_06_rejection_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$path_04_supporter_path_id" \
  -v "invitation_id=$path_04_supporter_invitation_id" \
  -v "recipient_id=$supporter_id" <<'PATH_06_REJECTION_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_invitation_models
   WHERE id = :'invitation_id' AND accepted_at IS NULL
     AND rejected_at IS NOT NULL AND canceled_at IS NULL),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'recipient_id'),
  (SELECT count(*) FROM authorization_outbox_models
   WHERE resource_type = 'path' AND resource_id = :'path_id'
     AND subject_id = :'recipient_id'),
  (SELECT count(*) FROM notification_models
   WHERE path_invitation_id = :'invitation_id'
     AND kind = 'path_invitation_accepted'),
  (SELECT count(*) FROM notification_models
   WHERE path_invitation_id = :'invitation_id'
     AND kind = 'path_invitation_received' AND deleted_at IS NOT NULL),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'recipient_id' AND operation = 'path.invitation.reject'
     AND resource_id = :'invitation_id'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'path_invitation.rejected' AND actor_user_id = :'recipient_id'
     AND target_id = :'invitation_id' AND outcome = 'succeeded');
PATH_06_REJECTION_EVIDENCE
)"
[[ "$path_06_rejection_evidence" == '1|0|0|0|1|1|1' ]] || {
  echo "PATH-06 rejection persistence mismatch: $path_06_rejection_evidence" >&2
  exit 1
}
echo "PATH-06 opaque, replayable invitation rejection with no access grant passed"

# PATH_GOAL_CASES exercises every independent optionality state and each
# supported recurrence through the live Dex-backed HTTP and PostgreSQL boundary.
create_goal_path() {
  local name="$1" idempotency_key="$2" request="$3" expected="$4" response
  response="$(curl -fsS -X POST \
    -H "Authorization: Bearer $owner_token" \
    -H "Idempotency-Key: $idempotency_key" \
    -H 'Content-Type: application/json' \
    --data "$request" \
    http://localhost:8080/v1/paths)"
  printf '%s' "$response" | EXPECTED_PATH="$expected" python3 -c \
    'import json,os,sys; actual=json.load(sys.stdin)["data"]; expected={**json.loads(os.environ["EXPECTED_PATH"]),"home":{"classification":"solo","pinned":False}}; path_id=actual.pop("id"); actual == expected or sys.exit(f"POST Path mismatch: actual={actual!r} expected={expected!r}"); print(path_id)'
}

assert_goal_path_get() {
  local path_id="$1" expected="$2"
  curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$path_id" |
    PATH_ID="$path_id" EXPECTED_PATH="$expected" python3 -c \
      'import json,os,sys; actual=json.load(sys.stdin)["data"]; expected={"id":os.environ["PATH_ID"],**json.loads(os.environ["EXPECTED_PATH"]),"home":{"classification":"solo","pinned":False}}; actual == expected or sys.exit(f"GET Path mismatch: actual={actual!r} expected={expected!r}")'
}

goal_case='neither'
neither_expected='{"name":"Goal case neither","visibility":"followers","capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
neither_path_id="$(create_goal_path 'Goal case neither' 'goal-path-neither-key-000001' \
  '{"name":"Goal case neither"}' "$neither_expected")"

goal_case='interval-only'
recurrence='hourly'
hourly_expected='{"name":"Goal default hourly","visibility":"followers","intervalGoal":{"targetSeconds":30,"recurrence":"hourly","alignment":{"minute":0}},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
hourly_path_id="$(create_goal_path 'Goal default hourly' 'goal-path-hourly-key-000001' \
  '{"name":"Goal default hourly","intervalGoal":{"targetSeconds":30,"recurrence":"hourly"}}' "$hourly_expected")"

goal_case='overall-only'
overall_expected='{"name":"Goal case overall only","visibility":"followers","overallTarget":{"targetSeconds":36000000},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
overall_path_id="$(create_goal_path 'Goal case overall only' 'goal-path-overall-key-000001' \
  '{"name":"Goal case overall only","overallTarget":{"targetSeconds":36000000}}' "$overall_expected")"

goal_case='both'
recurrence='daily'
daily_expected='{"name":"Goal default daily with overall","visibility":"followers","intervalGoal":{"targetSeconds":600,"recurrence":"daily","alignment":{"hour":0}},"overallTarget":{"targetSeconds":7200},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
daily_path_id="$(create_goal_path 'Goal default daily with overall' 'goal-path-daily-key-000001' \
  '{"name":"Goal default daily with overall","intervalGoal":{"targetSeconds":600,"recurrence":"daily"},"overallTarget":{"targetSeconds":7200}}' "$daily_expected")"

recurrence='weekly'
weekly_expected='{"name":"Goal default weekly","visibility":"followers","intervalGoal":{"targetSeconds":3600,"recurrence":"weekly","alignment":{"isoWeekday":1}},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
weekly_path_id="$(create_goal_path 'Goal default weekly' 'goal-path-weekly-key-000001' \
  '{"name":"Goal default weekly","intervalGoal":{"targetSeconds":3600,"recurrence":"weekly"}}' "$weekly_expected")"

recurrence='monthly'
monthly_expected='{"name":"Goal default monthly","visibility":"followers","intervalGoal":{"targetSeconds":14400,"recurrence":"monthly","alignment":{"day":1}},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
monthly_path_id="$(create_goal_path 'Goal default monthly' 'goal-path-monthly-key-00001' \
  '{"name":"Goal default monthly","intervalGoal":{"targetSeconds":14400,"recurrence":"monthly"}}' "$monthly_expected")"

recurrence='yearly'
yearly_expected='{"name":"Goal default yearly","visibility":"followers","intervalGoal":{"targetSeconds":86400,"recurrence":"yearly","alignment":{"month":1,"day":1}},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
yearly_path_id="$(create_goal_path 'Goal default yearly' 'goal-path-yearly-key-000001' \
  '{"name":"Goal default yearly","intervalGoal":{"targetSeconds":86400,"recurrence":"yearly"}}' "$yearly_expected")"

assert_goal_path_get "$neither_path_id" "$neither_expected"
assert_goal_path_get "$hourly_path_id" "$hourly_expected"
assert_goal_path_get "$overall_path_id" "$overall_expected"
assert_goal_path_get "$daily_path_id" "$daily_expected"
assert_goal_path_get "$weekly_path_id" "$weekly_expected"
assert_goal_path_get "$monthly_path_id" "$monthly_expected"
assert_goal_path_get "$yearly_path_id" "$yearly_expected"

# PATH_GOAL_LIST_ASSERTION verifies the complete goal-bearing Path shapes.
curl -fsS -H "Authorization: Bearer $owner_token" 'http://localhost:8080/v1/paths?limit=25' |
  NEITHER_ID="$neither_path_id" HOURLY_ID="$hourly_path_id" OVERALL_ID="$overall_path_id" DAILY_ID="$daily_path_id" \
  WEEKLY_ID="$weekly_path_id" MONTHLY_ID="$monthly_path_id" YEARLY_ID="$yearly_path_id" \
  python3 -c 'import json,os,sys; data=json.load(sys.stdin)["data"]; by_id={item["id"]:item for item in data}; capabilities={"trackTime":True,"inviteMembers":True,"manageMembers":True,"manageGoals":True,"manageLifecycle":True,"manageVisibility":True,"renamePath":True,"transferOwnership":True,"leavePath":False}; home={"classification":"solo","pinned":False}; expected={os.environ["NEITHER_ID"]:{"name":"Goal case neither","visibility":"followers"},os.environ["HOURLY_ID"]:{"name":"Goal default hourly","visibility":"followers","intervalGoal":{"targetSeconds":30,"recurrence":"hourly","alignment":{"minute":0}}},os.environ["OVERALL_ID"]:{"name":"Goal case overall only","visibility":"followers","overallTarget":{"targetSeconds":36000000}},os.environ["DAILY_ID"]:{"name":"Goal default daily with overall","visibility":"followers","intervalGoal":{"targetSeconds":600,"recurrence":"daily","alignment":{"hour":0}},"overallTarget":{"targetSeconds":7200}},os.environ["WEEKLY_ID"]:{"name":"Goal default weekly","visibility":"followers","intervalGoal":{"targetSeconds":3600,"recurrence":"weekly","alignment":{"isoWeekday":1}}},os.environ["MONTHLY_ID"]:{"name":"Goal default monthly","visibility":"followers","intervalGoal":{"targetSeconds":14400,"recurrence":"monthly","alignment":{"day":1}}},os.environ["YEARLY_ID"]:{"name":"Goal default yearly","visibility":"followers","intervalGoal":{"targetSeconds":86400,"recurrence":"yearly","alignment":{"month":1,"day":1}}}}; mismatches={path_id:(by_id.get(path_id),{"id":path_id,**path,"capabilities":capabilities,"home":home}) for path_id,path in expected.items() if by_id.get(path_id)!={"id":path_id,**path,"capabilities":capabilities,"home":home}}; not mismatches or sys.exit(f"LIST Path mismatches: {mismatches!r}")'

goal_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app <<'PATH_GOAL_PERSISTENCE_SQL'
SELECT
  name,
  COALESCE(interval_goal_target_seconds::text, '<null>'),
  COALESCE(interval_goal_recurrence, '<null>'),
  COALESCE(interval_goal_start_minute::text, '<null>'),
  COALESCE(interval_goal_start_hour::text, '<null>'),
  COALESCE(interval_goal_start_weekday::text, '<null>'),
  COALESCE(interval_goal_start_day::text, '<null>'),
  COALESCE(interval_goal_start_month::text, '<null>'),
  COALESCE(overall_target_seconds::text, '<null>')
FROM path_models
WHERE name LIKE 'Goal case %' OR name LIKE 'Goal default %'
ORDER BY name;
PATH_GOAL_PERSISTENCE_SQL
)"
expected_goal_persistence=$'Goal case neither|<null>|<null>|<null>|<null>|<null>|<null>|<null>|<null>\nGoal case overall only|<null>|<null>|<null>|<null>|<null>|<null>|<null>|36000000\nGoal default daily with overall|600|daily|<null>|0|<null>|<null>|<null>|7200\nGoal default hourly|30|hourly|0|<null>|<null>|<null>|<null>|<null>\nGoal default monthly|14400|monthly|<null>|<null>|<null>|1|<null>|<null>\nGoal default weekly|3600|weekly|<null>|<null>|1|<null>|<null>|<null>\nGoal default yearly|86400|yearly|<null>|<null>|<null>|1|1|<null>'
[[ "$goal_persistence" == "$expected_goal_persistence" ]] || {
  echo "persisted optional Path goal columns did not match the API responses" >&2
  diff -u <(printf '%s\n' "$expected_goal_persistence") <(printf '%s\n' "$goal_persistence") >&2 || true
  exit 1
}

# SOC-03B_GOAL_ACHIEVEMENT_FEED proves the goal transition lifecycle entirely
# through the activated owner's public Path, activity, feed, interaction, and
# notification APIs. The isolated Path keeps counts deterministic.
soc03b_expected='{"name":"Goal achievement acceptance","visibility":"private","intervalGoal":{"targetSeconds":60,"recurrence":"yearly","alignment":{"month":1,"day":1}},"overallTarget":{"targetSeconds":60},"capabilities":{"trackTime":true,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":true,"renamePath":true,"transferOwnership":true,"leavePath":false}}'
soc03b_path_id="$(create_goal_path 'Goal achievement acceptance' 'soc03b-path-create-key-0001' \
  '{"name":"Goal achievement acceptance","visibility":"private","intervalGoal":{"targetSeconds":60,"recurrence":"yearly"},"overallTarget":{"targetSeconds":60}}' "$soc03b_expected")"
# SOC03B_PATH_CHRONOLOGY_FIXTURE: this scenario deliberately records activity
# two days ago, so its Path and creator membership must already have existed.
# Production-created Paths retain their real creation and membership instants.
soc03b_chronology_count="$(docker compose exec -T postgres psql -qAt -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$soc03b_path_id" -v "owner_id=$owner_id" <<'SOC03B_PATH_CHRONOLOGY_SQL'
BEGIN;
UPDATE path_models
SET created_at = created_at - INTERVAL '3 days'
WHERE id = :'path_id' AND owner_user_id = :'owner_id';
UPDATE path_membership_models
SET joined_at = (SELECT created_at FROM path_models WHERE id = :'path_id')
WHERE path_id = :'path_id' AND user_id = :'owner_id' AND role = 'participant';
SELECT count(*)
FROM path_models AS path
JOIN path_membership_models AS membership
  ON membership.path_id = path.id AND membership.user_id = :'owner_id'
WHERE path.id = :'path_id'
  AND membership.joined_at = path.created_at
  AND path.created_at <= CURRENT_TIMESTAMP - INTERVAL '2 days 23 hours';
COMMIT;
SOC03B_PATH_CHRONOLOGY_SQL
)"
[[ "$soc03b_chronology_count" == 1 ]] || {
  echo "SOC-03B Path chronology fixture did not update exactly one owner membership" >&2
  exit 1
}
soc03b_invitation_body="$(SOC03B_PARTICIPANT_ID="$participant_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.participant","expectedRecipientUserId":os.environ["SOC03B_PARTICIPANT_ID"],"offeredRole":"participant"},separators=(",",":")))')"
soc03b_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-invitation-send-key-01' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_invitation_body" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/invitations")"
soc03b_invitation_id="$(printf '%s' "$soc03b_invitation_response" | SOC03B_PATH_ID="$soc03b_path_id" SOC03B_PARTICIPANT_ID="$participant_id" python3 -c \
  'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["SOC03B_PATH_ID"] and d["recipientUserId"]==os.environ["SOC03B_PARTICIPANT_ID"] and d["offeredRole"]=="participant";print(d["id"])')"
curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: soc03b-invitation-accept-key-1' \
  "http://localhost:8080/v1/path-invitations/$soc03b_invitation_id/accept" |
  SOC03B_INVITATION_ID="$soc03b_invitation_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["SOC03B_INVITATION_ID"] and d["acceptedAt"]'
expect_status 200 -H "Authorization: Bearer $participant_token" "http://localhost:8080/v1/paths/$soc03b_path_id"

soc03b_feed() {
  curl -fsS -H "Authorization: Bearer $participant_token" 'http://localhost:8080/v1/social/feed?limit=100'
}

soc03b_path_notification_ids() {
  curl -fsS -H "Authorization: Bearer $1" 'http://localhost:8080/v1/notifications?limit=100' |
    SOC03B_PATH_ID="$soc03b_path_id" python3 -c \
      'import json,os,sys;print(json.dumps(sorted(item["id"] for item in json.load(sys.stdin)["data"] if item.get("pathId")==os.environ["SOC03B_PATH_ID"]),separators=(",",":")))'
}

soc03b_achievement_ids() {
  SOC03B_PATH_ID="$soc03b_path_id" SOC03B_OWNER_ID="$owner_id" \
    SOC03B_PRACTICE_COUNT="$1" SOC03B_ACHIEVEMENT_COUNT="$2" python3 -c '
import datetime,json,os,sys
d=json.load(sys.stdin)
items=d["data"]["items"]
def instant(value): return datetime.datetime.fromisoformat(value.replace("Z","+00:00"))
order=[(instant(item["publishedAt"]),item["id"]) for item in items]
assert order==sorted(order,reverse=True), "feed is not newest-first with event-ID tie breaking"
path_items=[item for item in items if item["path"]["id"]==os.environ["SOC03B_PATH_ID"]]
practice=[item for item in path_items if item["type"]=="practice_session"]
achievements=[item for item in path_items if item["type"]=="goal_achievement"]
assert len(practice)==int(os.environ["SOC03B_PRACTICE_COUNT"]), (len(practice),path_items)
assert len(achievements)==int(os.environ["SOC03B_ACHIEVEMENT_COUNT"]), (len(achievements),path_items)
common={"id","type","publishedAt","participant","path","reactions","viewerReaction","commentsEnabled","reactionsEnabled"}
for item in path_items:
  assert item["participant"]["userId"]==os.environ["SOC03B_OWNER_ID"]
  assert {"userId","username","displayName"} <= set(item["participant"]) <= {"userId","username","displayName","profilePictureURL"}
  assert item["path"]=={"id":os.environ["SOC03B_PATH_ID"],"name":"Goal achievement acceptance"}
  assert item["commentsEnabled"] is True and item["reactionsEnabled"] is True
  assert item["reactions"]=={"heart":0,"applause":0,"fire":0,"strong":0,"celebrate":0} and item["viewerReaction"] is None
  instant(item["publishedAt"])
for item in practice:
  assert set(item)==common|{"activity"} and set(item["activity"])=={"id","durationSeconds","edited"}
  assert isinstance(item["activity"]["durationSeconds"],int) and item["activity"]["durationSeconds"]>0 and isinstance(item["activity"]["edited"],bool)
if achievements:
  assert {item["achievement"]["kind"] for item in achievements}=={"interval","overall"}
  by_kind={item["achievement"]["kind"]:item for item in achievements}
  interval=by_kind["interval"]["achievement"]
  overall=by_kind["overall"]["achievement"]
  assert set(by_kind["interval"])==common|{"achievement"} and set(by_kind["overall"])==common|{"achievement"}
  assert set(interval)=={"kind","targetSeconds","intervalStartedAt","intervalEndedAt"} and interval["targetSeconds"]==60
  assert instant(interval["intervalEndedAt"])>instant(interval["intervalStartedAt"])
  assert overall=={"kind":"overall","targetSeconds":60}
  print(by_kind["interval"]["id"]+"|"+by_kind["overall"]["id"])
'
}

# Configuring goals is not itself an achievement transition.
soc03b_feed | soc03b_achievement_ids 0 0 >/dev/null
soc03b_owner_notification_ids="$(soc03b_path_notification_ids "$owner_token")"
soc03b_participant_notification_ids="$(soc03b_path_notification_ids "$participant_token")"

soc03b_defaults="$(curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities/manual-defaults")"
soc03b_local_date="$(printf '%s' "$soc03b_defaults" | python3 -c \
  'import datetime,json,sys;d=json.load(sys.stdin)["data"];print(datetime.date.fromisoformat(d["localDate"])-datetime.timedelta(days=2))')"
soc03b_activity_payload() {
  SOC03B_DATE="$soc03b_local_date" SOC03B_TIME="$1" SOC03B_DURATION="$2" SOC03B_NOTE="$3" python3 -c \
    'import json,os;print(json.dumps({"localDate":os.environ["SOC03B_DATE"],"localStartTime":os.environ["SOC03B_TIME"],"durationSeconds":int(os.environ["SOC03B_DURATION"]),"note":os.environ["SOC03B_NOTE"]},separators=(",",":")))'
}

soc03b_first_payload="$(soc03b_activity_payload '12:00:00' 60 'private achievement source')"
soc03b_first_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-first-activity-key-001' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_first_payload" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities")"
soc03b_first_activity_id="$(printf '%s' "$soc03b_first_response" | python3 -c \
  'import json,sys;d=json.load(sys.stdin)["data"];assert d["activity"]["durationSeconds"]==60 and d["accumulatedSeconds"]==60;print(d["activity"]["id"])')"
soc03b_first_ids="$(soc03b_feed | soc03b_achievement_ids 1 2)"
IFS='|' read -r soc03b_first_interval_id soc03b_first_overall_id <<<"$soc03b_first_ids"
[[ -n "$soc03b_first_interval_id" && -n "$soc03b_first_overall_id" && "$soc03b_first_interval_id" != "$soc03b_first_overall_id" ]] || {
  echo "SOC-03B first crossing did not return distinct achievement identities" >&2
  exit 1
}

[[ "$(soc03b_path_notification_ids "$owner_token")" == "$soc03b_owner_notification_ids" && \
   "$(soc03b_path_notification_ids "$participant_token")" == "$soc03b_participant_notification_ids" ]] || {
  echo "SOC-03B achievement emitted a notification" >&2
  exit 1
}

soc03b_first_replay="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-first-activity-key-001' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_first_payload" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities")"
[[ "$soc03b_first_replay" == "$soc03b_first_response" ]] || { echo "SOC-03B source activity replay drifted" >&2; exit 1; }
[[ "$(soc03b_feed | soc03b_achievement_ids 1 2)" == "$soc03b_first_ids" ]] || {
  echo "SOC-03B source replay duplicated or replaced achievements" >&2
  exit 1
}

soc03b_above_payload="$(soc03b_activity_payload '12:02:00' 10 'above target')"
curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-above-activity-key-001' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_above_payload" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]["accumulatedSeconds"]==70'
[[ "$(soc03b_feed | soc03b_achievement_ids 2 2)" == "$soc03b_first_ids" ]] || {
  echo "SOC-03B additional above-target time duplicated achievements" >&2
  exit 1
}

# SOC-04D_ACHIEVEMENT_INTERACTIONS exercises the positive achievement
# interaction boundary before the source edit invalidates both achievements.
# The owner and comment author both heart the same comment so the roster proves
# deterministic ordering, opaque cursor pagination, and viewer-relative state.
soc04d_event_id="$soc03b_first_interval_id"
curl -fsS -X PUT \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: soc04d-achievement-reaction-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"reaction":"heart"}' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/reaction" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d=={"reactions":{"heart":1,"applause":0,"fire":0,"strong":0,"celebrate":0},"viewerReaction":"heart"}'
soc04d_comment_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: soc04d-achievement-comment-key-001' \
  -H 'Content-Type: application/json' \
  --data '{"text":"Goal achieved together"}' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments")"
soc04d_comment_id="$(printf '%s' "$soc04d_comment_response" | SOC04D_EVENT_ID="$soc04d_event_id" SOC04D_AUTHOR_ID="$participant_id" python3 -c \
  'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["eventId"]==os.environ["SOC04D_EVENT_ID"] and d["authorUserId"]==os.environ["SOC04D_AUTHOR_ID"] and d["text"]=="Goal achieved together" and d["version"]==1 and d["edited"]==False;print(d["id"])')"

curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc04d-owner-heart-key-000001' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/heart" |
  SOC04D_COMMENT_ID="$soc04d_comment_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"commentId":os.environ["SOC04D_COMMENT_ID"],"heartCount":1,"heartedByViewer":True}'
curl -fsS -X PUT \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: soc04d-author-heart-key-0001' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/heart" |
  SOC04D_COMMENT_ID="$soc04d_comment_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"commentId":os.environ["SOC04D_COMMENT_ID"],"heartCount":2,"heartedByViewer":True}'

curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments?limit=25" |
  SOC04D_COMMENT_ID="$soc04d_comment_id" SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=d["data"]["items"];assert len(items)==1 and d["meta"]=={};item=items[0];assert item["comment"]["id"]==os.environ["SOC04D_COMMENT_ID"] and item["comment"]["eventId"]==os.environ["SOC04D_EVENT_ID"] and item["heartCount"]==2 and item["heartedByViewer"]==True'
soc04d_roster_order() {
  curl -fsS -H "Authorization: Bearer $owner_token" \
    "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/hearts?limit=100" |
    python3 -c 'import json,sys;d=json.load(sys.stdin);assert d["meta"]=={};print("|".join(item["id"] for item in d["data"]["items"]))'
}
soc04d_roster_order_first="$(soc04d_roster_order)"
soc04d_roster_order_second="$(soc04d_roster_order)"
[[ "$soc04d_roster_order_first" == "$soc04d_roster_order_second" ]] || {
  echo "SOC-04D achievement comment heart roster order drifted" >&2
  exit 1
}
SOC04D_ROSTER_ORDER="$soc04d_roster_order_first" SOC04D_OWNER_ID="$owner_id" SOC04D_AUTHOR_ID="$participant_id" python3 -c \
  'import os;ids=os.environ["SOC04D_ROSTER_ORDER"].split("|");assert len(ids)==2 and len(set(ids))==2 and set(ids)=={os.environ["SOC04D_OWNER_ID"],os.environ["SOC04D_AUTHOR_ID"]}'
soc04d_roster_first="$(curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/hearts?limit=1")"
soc04d_roster_cursor="$(printf '%s' "$soc04d_roster_first" | python3 -c \
  'import json,sys;d=json.load(sys.stdin);assert len(d["data"]["items"])==1 and d["meta"]["nextCursor"];print(d["meta"]["nextCursor"])')"
soc04d_roster_first_id="$(printf '%s' "$soc04d_roster_first" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["items"][0]["id"])')"
soc04d_roster_second_id="$(curl -fsS --get \
  -H "Authorization: Bearer $owner_token" \
  --data-urlencode "cursor=$soc04d_roster_cursor" \
  --data 'limit=1' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/hearts" |
  SOC04D_ROSTER_CURSOR="$soc04d_roster_cursor" python3 -c \
    'import json,os,sys;assert os.environ["SOC04D_ROSTER_CURSOR"];d=json.load(sys.stdin);assert len(d["data"]["items"])==1 and d["meta"]=={};print(d["data"]["items"][0]["id"])')"
[[ "$soc04d_roster_first_id|$soc04d_roster_second_id" == "$soc04d_roster_order_first" ]] || {
  echo "SOC-04D paged achievement heart roster did not preserve stable order" >&2
  exit 1
}
# The delayed comment notice must survive the grace window, reach the owner as
# push rather than being classified as an ineligible practice-only subject, and
# remain tied to the achievement comment that invalidation removes below.
soc04d_comment_notification_id="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "event_id=$soc04d_event_id" -v "comment_id=$soc04d_comment_id" \
  -v "actor_id=$participant_id" -v "recipient_id=$owner_id" <<'SOC04D_COMMENT_NOTIFICATION_ID'
SELECT notification.id
FROM notification_models notification
JOIN social_practice_comment_models comment_row ON comment_row.id = notification.comment_id
WHERE notification.kind = 'practice_comment'
  AND notification.social_feed_event_id = :'event_id'
  AND notification.comment_id = :'comment_id'
  AND notification.actor_user_id = :'actor_id'
  AND notification.recipient_user_id = :'recipient_id'
  AND notification.created_at = comment_row.created_at + interval '5 seconds'
  AND notification.deleted_at IS NULL;
SOC04D_COMMENT_NOTIFICATION_ID
)"
[[ -n "$soc04d_comment_notification_id" ]] || {
  echo "SOC-04D achievement comment did not create one delayed owner notification" >&2
  exit 1
}
wait_for_push "$soc04d_comment_notification_id" |
  NOTIFICATION_ID="$soc04d_comment_notification_id" SOC04D_PATH_NAME='Goal achievement acceptance' python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);assert d["to"]=="ExponentPushToken[path-03-owner]" and d["priority"]=="normal" and d["data"]=={"version":"1","notificationId":os.environ["NOTIFICATION_ID"]};body=d["body"];assert os.environ["SOC04D_PATH_NAME"] in body and "practice" not in body.lower()'
soc03b_feed | SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
  'import json,os,sys;items=json.load(sys.stdin)["data"]["items"];item=next(item for item in items if item["id"]==os.environ["SOC04D_EVENT_ID"]);assert item["reactions"]=={"heart":1,"applause":0,"fire":0,"strong":0,"celebrate":0} and item["viewerReaction"]=="heart" and item["commentsEnabled"] is True and item["reactionsEnabled"] is True'

# SOC-05_PROFILE_INTERACTION_CONTROLS proves that the event owner's two
# profile-wide settings are independent. Disabled engagement is hidden rather
# than destroyed, new engagement is denied opaquely, all notices and pending or
# delivered push work are permanently removed, and a previously delivered push
# resolves only to accessible event context plus the disabled interaction type.
curl -fsS -H "Authorization: Bearer $owner_token" \
  'http://localhost:8080/v1/me/interaction-settings' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"commentsEnabled":True,"reactionsEnabled":True}'

soc05_reaction_notification_id="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "event_id=$soc04d_event_id" -v "actor_id=$participant_id" -v "recipient_id=$owner_id" <<'SOC05_REACTION_NOTIFICATION_ID'
SELECT id FROM notification_models
WHERE kind = 'practice_reaction'
  AND social_feed_event_id = :'event_id'
  AND actor_user_id = :'actor_id'
  AND recipient_user_id = :'recipient_id'
  AND deleted_at IS NULL;
SOC05_REACTION_NOTIFICATION_ID
)"
soc05_heart_notification_id="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "event_id=$soc04d_event_id" -v "comment_id=$soc04d_comment_id" \
  -v "actor_id=$owner_id" -v "recipient_id=$participant_id" <<'SOC05_HEART_NOTIFICATION_ID'
SELECT id FROM notification_models
WHERE kind = 'comment_heart'
  AND social_feed_event_id = :'event_id'
  AND comment_id = :'comment_id'
  AND actor_user_id = :'actor_id'
  AND recipient_user_id = :'recipient_id'
  AND deleted_at IS NULL;
SOC05_HEART_NOTIFICATION_ID
)"
[[ -n "$soc05_reaction_notification_id" && -n "$soc05_heart_notification_id" ]] || {
  echo "SOC-05 prerequisite interaction notices were not projected" >&2
  exit 1
}

soc05_comments_disabled="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc05-disable-comments-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"commentsEnabled":false,"reactionsEnabled":true}' \
  'http://localhost:8080/v1/me/interaction-settings')"
printf '%s' "$soc05_comments_disabled" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"commentsEnabled":False,"reactionsEnabled":True}'
soc05_comments_disabled_replay="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc05-disable-comments-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"commentsEnabled":false,"reactionsEnabled":true}' \
  'http://localhost:8080/v1/me/interaction-settings')"
[[ "$soc05_comments_disabled_replay" == "$soc05_comments_disabled" ]] || {
  echo "SOC-05 comment-disable replay drifted" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $owner_token" \
  'http://localhost:8080/v1/me/interaction-settings' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"commentsEnabled":False,"reactionsEnabled":True}'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments?limit=25" |
  python3 -c 'import json,sys;d=json.load(sys.stdin);assert d["data"]["items"]==[] and d["meta"]=={}'
expect_status 404 -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: soc05-denied-comment-key-001' \
  -H 'Content-Type: application/json' \
  --data '{"text":"This must stay hidden"}' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments"
expect_status 404 -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/hearts?limit=25"
soc03b_feed | SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
  'import json,os,sys;item=next(i for i in json.load(sys.stdin)["data"]["items"] if i["id"]==os.environ["SOC04D_EVENT_ID"]);assert item["commentsEnabled"] is False and item["reactionsEnabled"] is True and item["reactions"]["heart"]==1 and item["viewerReaction"]=="heart"'
# SOC05_EXACT_EVENT_RESOLUTION keeps old-push navigation independent of feed
# pagination: the referenced accessible event must be directly retrievable.
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id" |
  SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
    'import json,os,sys;item=json.load(sys.stdin)["data"];assert item["id"]==os.environ["SOC04D_EVENT_ID"] and item["commentsEnabled"] is False and item["reactionsEnabled"] is True'

# SOC05_HIDDEN_STORAGE_EVIDENCE directly distinguishes reversible projection
# hiding from destructive engagement deletion while proving notice and push
# cleanup across both the event owner and the comment author recipients.
soc05_comments_hidden_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "event_id=$soc04d_event_id" -v "comment_id=$soc04d_comment_id" \
  -v "comment_notice=$soc04d_comment_notification_id" -v "heart_notice=$soc05_heart_notification_id" \
  -v "reaction_notice=$soc05_reaction_notification_id" <<'SOC05_HIDDEN_STORAGE_EVIDENCE'
SELECT
  (SELECT count(*) FROM social_practice_comment_models WHERE id = :'comment_id' AND social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM social_practice_comment_heart_models WHERE comment_id = :'comment_id'),
  (SELECT count(*) FROM social_practice_reaction_models WHERE social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM notification_models WHERE id IN (:'comment_notice', :'heart_notice') AND deleted_at IS NOT NULL AND interaction_disabled_reason = 'comments'),
  (SELECT count(*) FROM notification_models WHERE id IN (:'comment_notice', :'heart_notice') AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models WHERE id = :'reaction_notice' AND deleted_at IS NULL AND interaction_disabled_reason IS NULL),
  (SELECT count(*) FROM notification_push_delivery_models WHERE notification_id IN (:'comment_notice', :'heart_notice')),
  (SELECT count(*) FROM notification_push_outbox_models WHERE notification_id IN (:'comment_notice', :'heart_notice'));
SOC05_HIDDEN_STORAGE_EVIDENCE
)"
[[ "$soc05_comments_hidden_evidence" == '1|2|1|2|0|1|0|0' ]] || {
  echo "SOC-05 comment hidden-storage or notification cleanup evidence drifted: $soc05_comments_hidden_evidence" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/notifications/$soc04d_comment_notification_id" |
  SOC05_EVENT_ID="$soc04d_event_id" SOC05_PATH_ID="$soc03b_path_id" SOC05_OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["type"]=="practice_comment" and d["interactionDisabled"]=="comments" and d["socialFeedEventId"]==os.environ["SOC05_EVENT_ID"] and d["pathId"]==os.environ["SOC05_PATH_ID"] and d["actor"]["userId"]==os.environ["SOC05_OWNER_ID"] and "commentId" not in d and "reaction" not in d'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/notifications/$soc05_heart_notification_id" |
  SOC05_EVENT_ID="$soc04d_event_id" SOC05_PATH_ID="$soc03b_path_id" SOC05_OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["type"]=="comment_heart" and d["interactionDisabled"]=="comments" and d["socialFeedEventId"]==os.environ["SOC05_EVENT_ID"] and d["pathId"]==os.environ["SOC05_PATH_ID"] and d["actor"]["userId"]==os.environ["SOC05_OWNER_ID"] and "commentId" not in d and "reaction" not in d'

curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc05-enable-comments-key-001' \
  -H 'Content-Type: application/json' \
  --data '{"commentsEnabled":true,"reactionsEnabled":true}' \
  'http://localhost:8080/v1/me/interaction-settings' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"commentsEnabled":True,"reactionsEnabled":True}'
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments?limit=25" |
  SOC04D_COMMENT_ID="$soc04d_comment_id" python3 -c \
    'import json,os,sys;items=json.load(sys.stdin)["data"]["items"];assert len(items)==1 and items[0]["comment"]["id"]==os.environ["SOC04D_COMMENT_ID"] and items[0]["heartCount"]==2'

soc05_reactions_disabled="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc05-disable-reactions-key-1' \
  -H 'Content-Type: application/json' \
  --data '{"commentsEnabled":true,"reactionsEnabled":false}' \
  'http://localhost:8080/v1/me/interaction-settings')"
printf '%s' "$soc05_reactions_disabled" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"commentsEnabled":True,"reactionsEnabled":False}'
soc05_reactions_disabled_replay="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc05-disable-reactions-key-1' \
  -H 'Content-Type: application/json' \
  --data '{"commentsEnabled":true,"reactionsEnabled":false}' \
  'http://localhost:8080/v1/me/interaction-settings')"
[[ "$soc05_reactions_disabled_replay" == "$soc05_reactions_disabled" ]] || {
  echo "SOC-05 reaction-disable replay drifted" >&2
  exit 1
}
soc03b_feed | SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
  'import json,os,sys;item=next(i for i in json.load(sys.stdin)["data"]["items"] if i["id"]==os.environ["SOC04D_EVENT_ID"]);assert item["commentsEnabled"] is True and item["reactionsEnabled"] is False and item["reactions"]=={"heart":0,"applause":0,"fire":0,"strong":0,"celebrate":0} and item["viewerReaction"] is None'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id" |
  SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
    'import json,os,sys;item=json.load(sys.stdin)["data"];assert item["id"]==os.environ["SOC04D_EVENT_ID"] and item["commentsEnabled"] is True and item["reactionsEnabled"] is False and item["reactions"]=={"heart":0,"applause":0,"fire":0,"strong":0,"celebrate":0} and item["viewerReaction"] is None'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments?limit=25" |
  SOC04D_COMMENT_ID="$soc04d_comment_id" python3 -c \
    'import json,os,sys;items=json.load(sys.stdin)["data"]["items"];assert len(items)==1 and items[0]["comment"]["id"]==os.environ["SOC04D_COMMENT_ID"] and items[0]["heartCount"]==2'
expect_status 404 -X PUT \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: soc05-denied-reaction-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"reaction":"fire"}' \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/reaction"
soc05_reactions_hidden_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "event_id=$soc04d_event_id" -v "reaction_notice=$soc05_reaction_notification_id" \
  -v "comment_notice=$soc04d_comment_notification_id" -v "heart_notice=$soc05_heart_notification_id" <<'SOC05_REACTION_HIDDEN_STORAGE_EVIDENCE'
SELECT
  (SELECT count(*) FROM social_practice_reaction_models WHERE social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM notification_models WHERE id = :'reaction_notice' AND deleted_at IS NOT NULL AND interaction_disabled_reason = 'reactions'),
  (SELECT count(*) FROM notification_push_delivery_models WHERE notification_id = :'reaction_notice'),
  (SELECT count(*) FROM notification_push_outbox_models WHERE notification_id = :'reaction_notice'),
  (SELECT count(*) FROM notification_models WHERE id IN (:'comment_notice', :'heart_notice') AND deleted_at IS NOT NULL AND interaction_disabled_reason = 'comments');
SOC05_REACTION_HIDDEN_STORAGE_EVIDENCE
)"
[[ "$soc05_reactions_hidden_evidence" == '1|1|0|0|2' ]] || {
  echo "SOC-05 reaction hidden-storage or permanent notice evidence drifted: $soc05_reactions_hidden_evidence" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/notifications/$soc05_reaction_notification_id" |
  SOC05_EVENT_ID="$soc04d_event_id" SOC05_PATH_ID="$soc03b_path_id" SOC05_OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["type"]=="practice_reaction" and d["interactionDisabled"]=="reactions" and d["socialFeedEventId"]==os.environ["SOC05_EVENT_ID"] and d["pathId"]==os.environ["SOC05_PATH_ID"] and d["actor"]["userId"]==os.environ["SOC05_OWNER_ID"] and "commentId" not in d and "reaction" not in d'

curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc05-enable-reactions-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"commentsEnabled":true,"reactionsEnabled":true}' \
  'http://localhost:8080/v1/me/interaction-settings' |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]=={"commentsEnabled":True,"reactionsEnabled":True}'
soc03b_feed | SOC04D_EVENT_ID="$soc04d_event_id" python3 -c \
  'import json,os,sys;item=next(i for i in json.load(sys.stdin)["data"]["items"] if i["id"]==os.environ["SOC04D_EVENT_ID"]);assert item["commentsEnabled"] is True and item["reactionsEnabled"] is True and item["reactions"]["heart"]==1 and item["viewerReaction"]=="heart"'
[[ "$(soc03b_path_notification_ids "$owner_token")" == "$soc03b_owner_notification_ids" && \
   "$(soc03b_path_notification_ids "$participant_token")" == "$soc03b_participant_notification_ids" ]] || {
  echo "SOC-05 restored interaction notices that must remain permanently removed" >&2
  exit 1
}
echo "SOC-05 independent comment/reaction disable, cleanup, denial, and restore acceptance passed"

soc03b_below_payload="$(soc03b_activity_payload '12:00:00' 40 'edited below target')"
curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-edit-below-key-00001' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_below_payload" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities/$soc03b_first_activity_id" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["activity"]["durationSeconds"]==40 and d["accumulatedSeconds"]==50'
soc03b_feed | soc03b_achievement_ids 2 0 >/dev/null
expect_status 404 -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments?limit=25"
expect_status 404 -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/social/feed/$soc04d_event_id/comments/$soc04d_comment_id/hearts?limit=25"
soc04d_cleanup_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "event_id=$soc04d_event_id" -v "comment_id=$soc04d_comment_id" <<'SOC04D_CLEANUP_EVIDENCE'
SELECT
  (SELECT count(*) FROM social_practice_reaction_models WHERE social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM social_practice_comment_models WHERE social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM social_practice_comment_heart_models WHERE comment_id = :'comment_id'),
  (SELECT count(*) FROM notification_models WHERE social_feed_event_id = :'event_id' OR comment_id = :'comment_id');
SOC04D_CLEANUP_EVIDENCE
)"
[[ "$soc04d_cleanup_evidence" == '0|0|0|0' ]] || {
  echo "SOC-04D invalidated achievement engagement remained: $soc04d_cleanup_evidence" >&2
  exit 1
}
[[ "$(soc03b_path_notification_ids "$owner_token")" == "$soc03b_owner_notification_ids" && \
   "$(soc03b_path_notification_ids "$participant_token")" == "$soc03b_participant_notification_ids" ]] || {
  echo "SOC-04D invalidated achievement notifications remained visible" >&2
  exit 1
}
echo "SOC-04D achievement reaction, comment, paged heart roster, and invalidation cleanup acceptance passed"

soc03b_recross_payload="$(soc03b_activity_payload '12:03:00' 10 'first genuine recross')"
soc03b_recross_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-recross-activity-key-01' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_recross_payload" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities")"
soc03b_recross_activity_id="$(printf '%s' "$soc03b_recross_response" | python3 -c \
  'import json,sys;d=json.load(sys.stdin)["data"];assert d["accumulatedSeconds"]==60;print(d["activity"]["id"])')"
soc03b_recross_ids="$(soc03b_feed | soc03b_achievement_ids 3 2)"
IFS='|' read -r soc03b_recross_interval_id soc03b_recross_overall_id <<<"$soc03b_recross_ids"
[[ "$soc03b_recross_interval_id" != "$soc03b_first_interval_id" && "$soc03b_recross_interval_id" != "$soc03b_first_overall_id" && \
   "$soc03b_recross_overall_id" != "$soc03b_first_interval_id" && "$soc03b_recross_overall_id" != "$soc03b_first_overall_id" ]] || {
  echo "SOC-03B genuine recross restored an invalidated achievement identity" >&2
  exit 1
}

curl -fsS -X DELETE \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-delete-below-key-0001' \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities/$soc03b_recross_activity_id" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["accumulatedSeconds"]==50 and d["intervalProgress"]=={"accumulatedSeconds":50,"targetSeconds":60}'
soc03b_feed | soc03b_achievement_ids 2 0 >/dev/null

soc03b_final_payload="$(soc03b_activity_payload '12:04:00' 10 'second genuine recross')"
curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: soc03b-final-activity-key-001' \
  -H 'Content-Type: application/json' \
  --data "$soc03b_final_payload" \
  "http://localhost:8080/v1/paths/$soc03b_path_id/activities" |
  python3 -c 'import json,sys;assert json.load(sys.stdin)["data"]["accumulatedSeconds"]==60'
soc03b_final_ids="$(soc03b_feed | soc03b_achievement_ids 3 2)"
IFS='|' read -r soc03b_final_interval_id soc03b_final_overall_id <<<"$soc03b_final_ids"
for soc03b_final_id in "$soc03b_final_interval_id" "$soc03b_final_overall_id"; do
  [[ "$soc03b_final_id" != "$soc03b_first_interval_id" && "$soc03b_final_id" != "$soc03b_first_overall_id" && \
     "$soc03b_final_id" != "$soc03b_recross_interval_id" && "$soc03b_final_id" != "$soc03b_recross_overall_id" ]] || {
    echo "SOC-03B post-delete recross reused an invalidated achievement identity" >&2
    exit 1
  }
done
[[ "$(soc03b_path_notification_ids "$owner_token")" == "$soc03b_owner_notification_ids" && \
   "$(soc03b_path_notification_ids "$participant_token")" == "$soc03b_participant_notification_ids" ]] || {
  echo "SOC-03B achievement lifecycle emitted a notification" >&2
  exit 1
}
echo "SOC-03B mixed goal-achievement feed lifecycle acceptance passed"

# Existing fixtures establish that a real supporter can view but cannot track,
# while an unrelated active account cannot track either the created Path or the
# role-matrix Path. These HTTP denials exercise the live SpiceDB decision point.
supporter_denied_timer="$(curl -sS -X POST -w '|%{http_code}' \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: supporter-timer-denial-key-01' \
  "http://localhost:8080/v1/paths/$path_id/timer")"
supporter_missing_timer="$(curl -sS -X POST -w '|%{http_code}' \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: supporter-timer-denial-key-01' \
  'http://localhost:8080/v1/paths/missing-supporter-path/timer')"
[[ "$supporter_denied_timer" == "$supporter_missing_timer" && "$supporter_denied_timer" == *'|404' ]] || {
  echo "supporter timer denial disclosed whether the Path exists" >&2
  exit 1
}
stranger_denied_timer="$(curl -sS -X POST -w '|%{http_code}' \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: stranger-timer-denial-key-001' \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
stranger_missing_timer="$(curl -sS -X POST -w '|%{http_code}' \
  -H "Authorization: Bearer $stranger_token" \
  -H 'Idempotency-Key: stranger-timer-denial-key-001' \
  'http://localhost:8080/v1/paths/missing-stranger-path/timer')"
[[ "$stranger_denied_timer" == "$stranger_missing_timer" && "$stranger_denied_timer" == *'|404' ]] || {
  echo "cross-user timer denial disclosed whether the Path exists" >&2
  exit 1
}

live_timer_start_key='live-timer-start-key-000001'
live_timer_start_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_timer_start_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
live_timer_id="$(printf '%s' "$live_timer_start_response" | LIVE_PATH_ID="$live_path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is True and d["accumulatedSeconds"]==0 and d["timer"]["pathId"]==os.environ["LIVE_PATH_ID"];print(d["timer"]["id"])')"
[[ -n "$live_timer_id" ]] || { echo "timer start did not return a durable timer ID" >&2; exit 1; }
replayed_timer_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_timer_start_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
[[ "$replayed_timer_response" == "$live_timer_start_response" ]] || {
  echo "duplicate timer start did not replay the original running state" >&2
  exit 1
}
conflicting_timer_start_key='live-timer-conflict-key-0001'
conflicting_timer_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $conflicting_timer_start_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
[[ "$conflicting_timer_response" == "$live_timer_start_response" ]] || {
  echo "distinct second start did not surface the existing running timer" >&2
  exit 1
}
simultaneous_path_id="$neither_path_id"
simultaneous_timer_start_key='live-simultaneous-start-key-001'
simultaneous_timer_start_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $simultaneous_timer_start_key" \
  "http://localhost:8080/v1/paths/$simultaneous_path_id/timer")"
simultaneous_timer_id="$(printf '%s' "$simultaneous_timer_start_response" | SIMULTANEOUS_PATH_ID="$simultaneous_path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is True and d["accumulatedSeconds"]==0 and d["timer"]["pathId"]==os.environ["SIMULTANEOUS_PATH_ID"];print(d["timer"]["id"])')"
[[ -n "$simultaneous_timer_id" && "$simultaneous_timer_id" != "$live_timer_id" ]] || {
  echo "different-Path timer start did not return an independent timer" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$live_path_id/timer" |
  LIVE_TIMER_ID="$live_timer_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is True and d["timer"]["id"]==os.environ["LIVE_TIMER_ID"] and d["accumulatedSeconds"]==0'
sleep 2

live_timer_stop_key='live-timer-stop-key-0000001'
live_timer_stop_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_timer_stop_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer/$live_timer_id")"
live_activity_evidence="$(printf '%s' "$live_timer_stop_response" | LIVE_PATH_ID="$live_path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["running"] is False and d["saved"] is True and d["subsecond"] is False and a["pathId"]==os.environ["LIVE_PATH_ID"] and a["durationSeconds"]>=1 and d["accumulatedSeconds"]==a["durationSeconds"];print(a["id"], a["durationSeconds"], sep="|")')"
live_activity_id="${live_activity_evidence%%|*}"
live_accumulated_seconds="${live_activity_evidence##*|}"
replayed_stop_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_timer_stop_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer/$live_timer_id")"
[[ "$replayed_stop_response" == "$live_timer_stop_response" ]] || {
  echo "duplicate timer stop did not replay the original activity" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$live_path_id/timer" |
  LIVE_ACCUMULATED_SECONDS="$live_accumulated_seconds" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is False and d["accumulatedSeconds"]==int(os.environ["LIVE_ACCUMULATED_SECONDS"]), "reloaded timer state did not retain accumulated time"'
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$simultaneous_path_id/timer" |
  SIMULTANEOUS_TIMER_ID="$simultaneous_timer_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is True and d["timer"]["id"]==os.environ["SIMULTANEOUS_TIMER_ID"] and d["accumulatedSeconds"]==0, "stopping one Path altered its simultaneous timer"'
simultaneous_timer_stop_key='live-simultaneous-stop-key-0001'
simultaneous_timer_stop_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $simultaneous_timer_stop_key" \
  "http://localhost:8080/v1/paths/$simultaneous_path_id/timer/$simultaneous_timer_id")"
simultaneous_activity_evidence="$(printf '%s' "$simultaneous_timer_stop_response" | SIMULTANEOUS_PATH_ID="$simultaneous_path_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["running"] is False and d["saved"] is True and a["pathId"]==os.environ["SIMULTANEOUS_PATH_ID"] and a["durationSeconds"]>=1 and d["accumulatedSeconds"]==a["durationSeconds"];print(a["id"],a["durationSeconds"],sep="|")')"
simultaneous_activity_id="${simultaneous_activity_evidence%%|*}"
simultaneous_accumulated_seconds="${simultaneous_activity_evidence##*|}"
delayed_start_replay="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_timer_start_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
printf '%s' "$delayed_start_replay" |
  LIVE_ACCUMULATED_SECONDS="$live_accumulated_seconds" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is False and "timer" not in d and d["accumulatedSeconds"]==int(os.environ["LIVE_ACCUMULATED_SECONDS"]), "delayed start replay resurrected a timer stopped on another client"'
delayed_conflicting_start_replay="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $conflicting_timer_start_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
printf '%s' "$delayed_conflicting_start_replay" |
  LIVE_ACCUMULATED_SECONDS="$live_accumulated_seconds" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is False and "timer" not in d and d["accumulatedSeconds"]==int(os.environ["LIVE_ACCUMULATED_SECONDS"]), "delayed conflicting start replay resurrected a timer"'
restarted_timer_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: live-timer-restarted-key-001' \
  "http://localhost:8080/v1/paths/$live_path_id/timer")"
restarted_timer_id="$(printf '%s' "$restarted_timer_response" | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["running"] is True;print(d["timer"]["id"])')"
delayed_stop_replay="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $live_timer_stop_key" \
  "http://localhost:8080/v1/paths/$live_path_id/timer/$live_timer_id")"
printf '%s' "$delayed_stop_replay" |
  RESTARTED_TIMER_ID="$restarted_timer_id" LIVE_ACTIVITY_ID="$live_activity_id" LIVE_ACCUMULATED_SECONDS="$live_accumulated_seconds" \
  python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["running"] is True and d["timer"]["id"]==os.environ["RESTARTED_TIMER_ID"] and d["saved"] is True and d["activity"]["id"]==os.environ["LIVE_ACTIVITY_ID"] and d["accumulatedSeconds"]==int(os.environ["LIVE_ACCUMULATED_SECONDS"]), "delayed stop replay hid a newer timer from another client"'

docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "owner_id=$owner_id" -v "path_id=$live_path_id" -v "timer_id=$live_timer_id" \
  -v "activity_id=$live_activity_id" -v "duration=$live_accumulated_seconds" \
  -v "start_key=$live_timer_start_key" -v "conflict_key=$conflicting_timer_start_key" \
  -v "stop_key=$live_timer_stop_key" -v "restart_key=live-timer-restarted-key-001" \
  <<'LIVE_ACTIVITY_SQL' | grep -Fx '1|1|4|1|1|1|1|1'
SELECT
  (SELECT count(*) FROM recorded_activity_models
   WHERE id = :'activity_id' AND path_id = :'path_id' AND participant_id = :'owner_id'
     AND FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))::bigint = :'duration'::bigint),
  (SELECT count(*) FROM running_timer_models
   WHERE id = :'timer_id' OR (path_id = :'path_id' AND participant_id = :'owner_id')),
  (SELECT count(*) FROM activity_mutation_models
   WHERE path_id = :'path_id' AND participant_id = :'owner_id'
     AND operation IN ('activity.timer.start', 'activity.timer.stop')),
  (SELECT count(*) FROM activity_mutation_models
   WHERE path_id = :'path_id' AND participant_id = :'owner_id'
     AND operation = 'activity.timer.start' AND key = :'start_key'),
  (SELECT count(*) FROM activity_mutation_models
   WHERE path_id = :'path_id' AND participant_id = :'owner_id'
     AND operation = 'activity.timer.start' AND key = :'conflict_key'),
  (SELECT count(*) FROM activity_mutation_models
   WHERE path_id = :'path_id' AND participant_id = :'owner_id'
     AND operation = 'activity.timer.stop' AND key = :'stop_key'),
  (SELECT count(*) FROM activity_mutation_models
   WHERE path_id = :'path_id' AND participant_id = :'owner_id'
     AND operation = 'activity.timer.start' AND key = :'restart_key'),
  (SELECT count(*) FROM recorded_activity_models
   WHERE path_id = :'path_id' AND participant_id = :'owner_id');
LIVE_ACTIVITY_SQL
# The final column mechanically proves exactly one recorded activity exists.
docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "owner_id=$owner_id" -v "first_path_id=$live_path_id" -v "second_path_id=$simultaneous_path_id" \
  -v "first_activity_id=$live_activity_id" -v "second_activity_id=$simultaneous_activity_id" \
  -v "first_duration=$live_accumulated_seconds" -v "second_duration=$simultaneous_accumulated_seconds" \
  -v "second_start_key=$simultaneous_timer_start_key" -v "second_stop_key=$simultaneous_timer_stop_key" \
  <<'LIVE_SIMULTANEOUS_ACTIVITY_SQL' | grep -Fx '2|2|1|1|1|1|1'
SELECT
  (SELECT count(*) FROM recorded_activity_models
   WHERE participant_id = :'owner_id' AND path_id IN (:'first_path_id', :'second_path_id')),
  (SELECT count(*) FROM activity_mutation_models
   WHERE participant_id = :'owner_id' AND path_id = :'second_path_id'
     AND operation IN ('activity.timer.start', 'activity.timer.stop')),
  (SELECT count(*) FROM activity_mutation_models
   WHERE participant_id = :'owner_id' AND path_id = :'second_path_id'
     AND operation = 'activity.timer.start' AND key = :'second_start_key'),
  (SELECT count(*) FROM activity_mutation_models
   WHERE participant_id = :'owner_id' AND path_id = :'second_path_id'
     AND operation = 'activity.timer.stop' AND key = :'second_stop_key'),
  (SELECT count(*) FROM recorded_activity_models first_entry
   JOIN recorded_activity_models second_entry
     ON first_entry.started_at < second_entry.ended_at
    AND second_entry.started_at < first_entry.ended_at
   WHERE first_entry.id = :'first_activity_id' AND second_entry.id = :'second_activity_id'),
  (SELECT count(*) FROM recorded_activity_models
   WHERE id = :'first_activity_id'
     AND FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))::bigint = :'first_duration'::bigint),
  (SELECT count(*) FROM recorded_activity_models
   WHERE id = :'second_activity_id'
     AND FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))::bigint = :'second_duration'::bigint);
LIVE_SIMULTANEOUS_ACTIVITY_SQL
echo "live Dex account and two-Path timers retained two fully counted overlapping activities"

manual_defaults="$(curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/manual-defaults")"
manual_payload="$(printf '%s' "$manual_defaults" | python3 -c 'import datetime,json,sys;d=json.load(sys.stdin)["data"];start=datetime.datetime.fromisoformat(d["localDate"]+"T"+d["localStartTime"])-datetime.timedelta(seconds=120);print(json.dumps({"localDate":start.date().isoformat(),"localStartTime":start.time().isoformat(),"durationSeconds":120,"note":"Cafe\u0301 live"},separators=(",",":")))')"
manual_create_key='live-manual-create-key-0001'
manual_create_response="$(curl -fsS -X POST -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_create_key" -H 'Content-Type: application/json' --data "$manual_payload" "http://localhost:8080/v1/paths/$live_path_id/activities")"
manual_activity_evidence="$(printf '%s' "$manual_create_response" | python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["version"]==1 and a["durationSeconds"]==120 and a["note"]=="Café live";print(a["id"],d["accumulatedSeconds"],sep="|")')"
manual_activity_id="${manual_activity_evidence%%|*}"
manual_create_total="${manual_activity_evidence##*|}"
manual_create_replay="$(curl -fsS -X POST -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_create_key" -H 'Content-Type: application/json' --data "$manual_payload" "http://localhost:8080/v1/paths/$live_path_id/activities")"
[[ "$manual_create_replay" == "$manual_create_response" ]] || { echo "manual activity create replay drifted" >&2; exit 1; }
manual_update_payload="$(printf '%s' "$manual_payload" | python3 -c 'import json,sys;d=json.load(sys.stdin);d["durationSeconds"]=60;d["note"]="edited private";print(json.dumps(d,separators=(",",":")))')"
manual_update_key='live-manual-update-key-0001'
manual_update_response="$(curl -fsS -X PUT -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_update_key" -H 'Content-Type: application/json' --data "$manual_update_payload" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id")"
manual_update_total="$(printf '%s' "$manual_update_response" | MANUAL_ACTIVITY_ID="$manual_activity_id" MANUAL_CREATE_TOTAL="$manual_create_total" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["version"]==2 and a["id"]==os.environ["MANUAL_ACTIVITY_ID"] and a["durationSeconds"]==60 and a["note"]=="edited private" and d["accumulatedSeconds"]==int(os.environ["MANUAL_CREATE_TOTAL"])-60;print(d["accumulatedSeconds"])')"
manual_update_replay="$(curl -fsS -X PUT -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_update_key" -H 'Content-Type: application/json' --data "$manual_update_payload" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id")"
[[ "$manual_update_replay" == "$manual_update_response" ]] || { echo "manual activity update replay drifted" >&2; exit 1; }
manual_public_updated_at="$(printf '%s' "$manual_update_response" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["activity"]["updatedAt"])')"
manual_note_only_payload="$(printf '%s' "$manual_update_payload" | python3 -c 'import json,sys;d=json.load(sys.stdin);d["note"]="hidden note-only edit";print(json.dumps(d,separators=(",",":")))')"
manual_note_only_key='live-manual-note-only-key-001'
curl -fsS -X PUT -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_note_only_key" -H 'Content-Type: application/json' --data "$manual_note_only_payload" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" MANUAL_UPDATE_TOTAL="$manual_update_total" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["version"]==3 and a["id"]==os.environ["MANUAL_ACTIVITY_ID"] and a["note"]=="hidden note-only edit" and d["accumulatedSeconds"]==int(os.environ["MANUAL_UPDATE_TOTAL"])'
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["version"]==3 and d["activity"]["id"]==os.environ["MANUAL_ACTIVITY_ID"] and d["activity"]["note"]=="hidden note-only edit"'
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id/revisions" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" python3 -c 'import json,os,sys;items=json.load(sys.stdin)["data"];assert len(items)==2 and [item["version"] for item in items]==[2,1] and items[0]["note"]=="edited private" and items[1]["id"]==os.environ["MANUAL_ACTIVITY_ID"] and items[1]["durationSeconds"]==120 and items[1]["note"]=="Café live"'
manual_revision_page_one="$(curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id/revisions?limit=1")"
manual_revision_cursor="$(printf '%s' "$manual_revision_page_one" | python3 -c 'import json,sys;d=json.load(sys.stdin);assert [item["version"] for item in d["data"]]==[2];print(d["meta"]["nextCursor"])')"
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id/revisions?limit=1&cursor=$manual_revision_cursor" |
  python3 -c 'import json,sys;d=json.load(sys.stdin);assert [item["version"] for item in d["data"]]==[1] and d["meta"]=={}'
manual_activity_page_one="$(curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities?limit=1")"
manual_activity_page_cursor="$(printf '%s' "$manual_activity_page_one" | python3 -c 'import json,sys;d=json.load(sys.stdin);assert len(d["data"])==1;print(d["data"][0]["activity"]["id"],d["meta"]["nextCursor"],sep="|")')"
manual_activity_page_one_id="${manual_activity_page_cursor%%|*}"
manual_activity_cursor="${manual_activity_page_cursor#*|}"
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities?limit=1&cursor=$manual_activity_cursor" |
  FIRST_ACTIVITY_ID="$manual_activity_page_one_id" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert len(d["data"])==1 and d["data"][0]["activity"]["id"]!=os.environ["FIRST_ACTIVITY_ID"]'
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$live_path_id" -v "participant_id=$participant_id" <<'MANUAL_ACTIVITY_PARTICIPANT_MEMBERSHIP'
INSERT INTO path_membership_models (path_id, user_id, role)
VALUES (:'path_id', :'participant_id', 'participant');
MANUAL_ACTIVITY_PARTICIPANT_MEMBERSHIP
(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run TestSeedPathHTTPAcceptanceParticipant -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$live_path_id" \
    -acceptance-path-participant "$participant_id"
)
expect_status 200 -H "Authorization: Bearer $participant_token" "http://localhost:8080/v1/paths/$live_path_id"
expect_status 404 -H "Authorization: Bearer $stranger_token" "http://localhost:8080/v1/paths/$live_path_id"
curl -fsS -H "Authorization: Bearer $participant_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" MANUAL_PUBLIC_UPDATED_AT="$manual_public_updated_at" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["version"]==2 and a["id"]==os.environ["MANUAL_ACTIVITY_ID"] and "note" not in a and a["updatedAt"]==os.environ["MANUAL_PUBLIC_UPDATED_AT"]'
curl -fsS -H "Authorization: Bearer $participant_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id/revisions" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" python3 -c 'import json,os,sys;items=json.load(sys.stdin)["data"];assert len(items)==1 and items[0]["id"]==os.environ["MANUAL_ACTIVITY_ID"] and items[0]["version"]==1 and "note" not in items[0]'
curl -fsS -H "Authorization: Bearer $participant_token" "http://localhost:8080/v1/paths/$live_path_id/activities" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" python3 -c 'import json,os,sys;matches=[item for item in json.load(sys.stdin)["data"] if item["activity"]["id"]==os.environ["MANUAL_ACTIVITY_ID"]];assert len(matches)==1 and matches[0]["version"]==2 and "note" not in matches[0]["activity"]'
manual_pg_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app -v "activity_id=$manual_activity_id" -v "owner_id=$owner_id" -v "path_id=$live_path_id" -v "create_key=$manual_create_key" -v "update_key=$manual_update_key" -v "note_key=$manual_note_only_key" <<'MANUAL_ACTIVITY_EVIDENCE'
SELECT
  (SELECT count(*) FROM recorded_activity_models WHERE id = :'activity_id' AND participant_id = :'owner_id' AND path_id = :'path_id' AND note = 'hidden note-only edit' AND FLOOR(EXTRACT(EPOCH FROM (ended_at-started_at)))::bigint = 60),
  (SELECT count(*) FROM recorded_activity_revision_models WHERE activity_id = :'activity_id'),
  (SELECT count(*) FROM recorded_activity_revision_models WHERE activity_id = :'activity_id' AND public_changed),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'owner_id' AND operation = 'activity.manual.create' AND key = :'create_key' AND result_version = 1),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'owner_id' AND operation = 'activity.update' AND key = :'update_key' AND result_version = 2),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'owner_id' AND operation = 'activity.update' AND key = :'note_key' AND result_version = 3);
MANUAL_ACTIVITY_EVIDENCE
)"
[[ "$manual_pg_evidence" == '1|2|1|1|1|1' ]] || { echo "manual activity persistence evidence drifted: $manual_pg_evidence" >&2; exit 1; }
echo "live manual activity create, owner edit/replay, private note projection, Path history, and immutable revision acceptance passed"

manual_denied_delete_key='live-manual-denied-delete-key-01'
manual_missing_activity_id='missing-manual-activity'
manual_denied_delete="$(curl -sS -X DELETE -w '|%{http_code}' -H "Authorization: Bearer $participant_token" -H "Idempotency-Key: $manual_denied_delete_key" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id")"
manual_missing_delete="$(curl -sS -X DELETE -w '|%{http_code}' -H "Authorization: Bearer $participant_token" -H "Idempotency-Key: $manual_denied_delete_key" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_missing_activity_id")"
[[ "$manual_denied_delete" == "$manual_missing_delete" && "$manual_denied_delete" == *'|404' ]] || {
  echo "cross-owner and missing activity deletion responses differed" >&2
  exit 1
}

manual_delete_key='live-manual-delete-key-0001'
manual_delete_total=$((manual_update_total - 60))
((manual_delete_total >= 0)) || { echo "manual activity deletion expected a negative accumulated total" >&2; exit 1; }
manual_delete_response="$(curl -fsS -X DELETE -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_delete_key" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id")"
manual_delete_unread_count="$(printf '%s' "$manual_delete_response" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" MANUAL_DELETE_TOTAL="$manual_delete_total" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"accumulatedSeconds":int(os.environ["MANUAL_DELETE_TOTAL"]),"sessionCount":1,"unreadNotificationCount":d["unreadNotificationCount"],"removedFeedEventIds":["practice:"+os.environ["MANUAL_ACTIVITY_ID"]]} and type(d["unreadNotificationCount"]) is int and d["unreadNotificationCount"]>=0;print(d["unreadNotificationCount"])')"
manual_delete_replay="$(curl -fsS -X DELETE -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_delete_key" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id")"
[[ "$manual_delete_replay" == "$manual_delete_response" ]] || { echo "manual activity deletion did not replay the original result" >&2; exit 1; }
expect_status 409 -X DELETE -H "Authorization: Bearer $owner_token" -H "Idempotency-Key: $manual_delete_key" "http://localhost:8080/v1/paths/$live_path_id/activities/$live_activity_id"

curl -fsS -H "Authorization: Bearer $owner_token" 'http://localhost:8080/v1/notifications?limit=1' |
  MANUAL_DELETE_UNREAD_COUNT="$manual_delete_unread_count" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d["meta"]["unreadCount"]==int(os.environ["MANUAL_DELETE_UNREAD_COUNT"])'

expect_status 404 -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id"
expect_status 404 -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$manual_activity_id/revisions"
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities" |
  MANUAL_ACTIVITY_ID="$manual_activity_id" LIVE_ACTIVITY_ID="$live_activity_id" python3 -c 'import json,os,sys;items=json.load(sys.stdin)["data"];ids=[item["activity"]["id"] for item in items];assert os.environ["MANUAL_ACTIVITY_ID"] not in ids and ids.count(os.environ["LIVE_ACTIVITY_ID"])==1'
curl -fsS -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$live_path_id/activities/$live_activity_id" |
  LIVE_ACTIVITY_ID="$live_activity_id" LIVE_DURATION="$live_accumulated_seconds" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];a=d["activity"];assert d["version"]==1 and a["id"]==os.environ["LIVE_ACTIVITY_ID"] and a["durationSeconds"]==int(os.environ["LIVE_DURATION"])'

manual_delete_pg_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "activity_id=$manual_activity_id" -v "unrelated_activity_id=$live_activity_id" -v "owner_id=$owner_id" -v "participant_id=$participant_id" \
  -v "path_id=$live_path_id" -v "missing_activity_id=$manual_missing_activity_id" -v "delete_key=$manual_delete_key" -v "denied_key=$manual_denied_delete_key" -v "delete_total=$manual_delete_total" -v "delete_unread_count=$manual_delete_unread_count" <<'MANUAL_ACTIVITY_DELETE_EVIDENCE'
SELECT
  (SELECT count(*) FROM recorded_activity_models WHERE id = :'activity_id'),
  (SELECT count(*) FROM recorded_activity_revision_models WHERE activity_id = :'activity_id'),
  (SELECT count(*) FROM recorded_activity_models WHERE id = :'unrelated_activity_id' AND participant_id = :'owner_id' AND path_id = :'path_id'),
  (SELECT count(*) FROM audit_event_models WHERE action = 'resource.deleted' AND owner_user_id = :'owner_id' AND actor_user_id = :'owner_id' AND target_type = 'activity' AND target_id = :'activity_id' AND outcome = 'succeeded'),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'owner_id' AND result_activity_id = :'activity_id'),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'owner_id' AND result_activity_id = :'activity_id' AND operation <> 'activity.delete' AND result_activity_deleted AND NOT result_activity_saved AND result_started_at IS NULL AND result_ended_at IS NULL AND result_time_zone = '' AND result_created_at IS NULL AND result_updated_at IS NULL AND result_note IS NULL AND result_version IS NULL AND result_accumulated_seconds IS NULL),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'owner_id' AND operation = 'activity.delete' AND key = :'delete_key' AND path_id = :'path_id' AND result_activity_id = :'activity_id' AND result_activity_deleted AND NOT result_activity_saved AND result_accumulated_seconds = :'delete_total'::bigint AND result_session_count = 1 AND result_unread_notification_count = :'delete_unread_count'::bigint AND result_removed_feed_event_ids = ARRAY['practice:' || :'activity_id']::text[]),
  (SELECT count(*) FROM audit_event_models WHERE action = 'resource.access_denied' AND owner_user_id = :'participant_id' AND actor_user_id = :'participant_id' AND target_type = 'activity' AND target_id = :'activity_id' AND outcome = 'denied'),
  (SELECT count(*) FROM audit_event_models WHERE action = 'resource.access_denied' AND owner_user_id = :'participant_id' AND actor_user_id = :'participant_id' AND target_type = 'activity' AND target_id = :'missing_activity_id' AND outcome = 'denied'),
  (SELECT count(*) FROM activity_mutation_models WHERE participant_id = :'participant_id' AND operation = 'activity.delete' AND key = :'denied_key');
MANUAL_ACTIVITY_DELETE_EVIDENCE
)"
[[ "$manual_delete_pg_evidence" == '0|0|1|1|4|3|1|1|1|0' ]] || {
  echo "manual activity deletion persistence mismatch: expected=0|0|1|1|4|3|1|1|1|0 actual=$manual_delete_pg_evidence" >&2
  exit 1
}
echo "manual activity delete/replay, opaque ownership, cascade, tombstone, and unrelated preservation acceptance passed"
browser_progress_path_key='browser-progress-path-create-key-0001'
browser_progress_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $browser_progress_path_key" \
  -H 'Content-Type: application/json' \
  --data '{"name":"Browser overall progress","intervalGoal":{"targetSeconds":60,"recurrence":"daily","alignment":{"hour":0}},"overallTarget":{"targetSeconds":120}}' \
  http://localhost:8080/v1/paths)"
browser_progress_path_id="$(printf '%s' "$browser_progress_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Browser overall progress" and d["intervalGoal"]=={"targetSeconds":60,"recurrence":"daily","alignment":{"hour":0}} and d["overallTarget"]=={"targetSeconds":120};print(d["id"])')"
[[ -n "$browser_progress_path_id" ]] || { echo "browser progress Path creation did not return an ID" >&2; exit 1; }
# BROWSER_MANUAL_ACTIVITY_CHRONOLOGY_FIXTURE: the browser scenario creates a
# one-minute activity and then moves its start two minutes earlier, so the
# isolated Path and creator role must predate the final edited occurrence.
browser_progress_chronology_count="$(docker compose exec -T postgres psql -qAt -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$browser_progress_path_id" -v "owner_id=$owner_id" <<'BROWSER_PROGRESS_CHRONOLOGY_SQL'
BEGIN;
UPDATE path_models
SET created_at = created_at - INTERVAL '4 minutes'
WHERE id = :'path_id' AND owner_user_id = :'owner_id';
UPDATE path_membership_models
SET joined_at = (SELECT created_at FROM path_models WHERE id = :'path_id')
WHERE path_id = :'path_id' AND user_id = :'owner_id' AND role = 'participant';
SELECT count(*)
FROM path_models AS path
JOIN path_membership_models AS membership
  ON membership.path_id = path.id AND membership.user_id = :'owner_id'
WHERE path.id = :'path_id'
  AND membership.joined_at = path.created_at
  AND path.created_at <= CURRENT_TIMESTAMP - INTERVAL '3 minutes 50 seconds';
COMMIT;
BROWSER_PROGRESS_CHRONOLOGY_SQL
)"
[[ "$browser_progress_chronology_count" == 1 ]] || {
  echo "browser manual-activity chronology fixture did not update exactly one owner membership" >&2
  exit 1
}
WEB_ACCEPTANCE_BASE_URL="$web_base_url" WEB_ACCEPTANCE_API_URL="$api_base_url" WEB_ACCEPTANCE_APPLICATION_TOKEN="$owner_token" WEB_ACCEPTANCE_PATH_NAME='Browser overall progress' node scripts/web-manual-activity-acceptance.mjs

browser_goal_alignment_weekday="$(python3 -c 'import datetime,zoneinfo;weekday=datetime.datetime.now(zoneinfo.ZoneInfo("America/New_York")).isoweekday();print(7 if weekday == 1 else weekday - 1)')"
browser_goal_path_key='browser-goal-path-create-key-0001'
browser_goal_path_request="$(BROWSER_GOAL_ALIGNMENT_WEEKDAY="$browser_goal_alignment_weekday" python3 -c 'import json,os;print(json.dumps({"name":"Browser goal reconfiguration","intervalGoal":{"targetSeconds":60,"recurrence":"weekly","alignment":{"isoWeekday":int(os.environ["BROWSER_GOAL_ALIGNMENT_WEEKDAY"])}}},separators=(",",":")))')"
browser_goal_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $browser_goal_path_key" \
  -H 'Content-Type: application/json' \
  --data "$browser_goal_path_request" \
  http://localhost:8080/v1/paths)"
browser_goal_path_id="$(printf '%s' "$browser_goal_path_response" |
  BROWSER_GOAL_ALIGNMENT_WEEKDAY="$browser_goal_alignment_weekday" python3 -c 'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Browser goal reconfiguration" and d["intervalGoal"]=={"targetSeconds":60,"recurrence":"weekly","alignment":{"isoWeekday":int(os.environ["BROWSER_GOAL_ALIGNMENT_WEEKDAY"])}} and "overallTarget" not in d;print(d["id"])')"
[[ -n "$browser_goal_path_id" ]] || { echo "browser goal-update Path creation did not return an ID" >&2; exit 1; }

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$browser_goal_path_id" -v "administrator_id=$administrator_id" -v "participant_id=$participant_id" <<'BROWSER_GOAL_ROLE_MEMBERSHIPS'
INSERT INTO path_membership_models (path_id, user_id, role)
VALUES
  (:'path_id', :'administrator_id', 'administrator'),
  (:'path_id', :'participant_id', 'participant');
BROWSER_GOAL_ROLE_MEMBERSHIPS
# BROWSER_GOAL_ACTIVITY_CHRONOLOGY_FIXTURE: the administrator's synthetic
# 45-second activity must begin after that role incarnation joined the Path.
browser_goal_chronology_count="$(docker compose exec -T postgres psql -qAt -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$browser_goal_path_id" -v "administrator_id=$administrator_id" <<'BROWSER_GOAL_CHRONOLOGY_SQL'
BEGIN;
UPDATE path_models
SET created_at = created_at - INTERVAL '1 minute'
WHERE id = :'path_id';
UPDATE path_membership_models
SET joined_at = (SELECT created_at FROM path_models WHERE id = :'path_id')
WHERE path_id = :'path_id' AND user_id = :'administrator_id' AND role = 'administrator';
SELECT count(*)
FROM path_models AS path
JOIN path_membership_models AS membership
  ON membership.path_id = path.id AND membership.user_id = :'administrator_id'
WHERE path.id = :'path_id'
  AND membership.joined_at = path.created_at
  AND path.created_at <= CURRENT_TIMESTAMP - INTERVAL '50 seconds';
COMMIT;
BROWSER_GOAL_CHRONOLOGY_SQL
)"
[[ "$browser_goal_chronology_count" == 1 ]] || {
  echo "browser goal chronology fixture did not update exactly one administrator membership" >&2
  exit 1
}
(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run TestSeedPrivatePathHTTPAcceptanceRelationships -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$browser_goal_path_id" \
    -acceptance-path-creator "$owner_id" \
    -acceptance-path-administrator "$administrator_id" \
    -acceptance-path-participant "$participant_id" \
    -acceptance-path-supporter "$supporter_id"
)

# BROWSER_GOAL_ADMINISTRATOR_ACTIVITY proves that goal progress remains
# participant-scoped while an administrator manages the shared configuration.
browser_goal_defaults="$(curl -fsS -H "Authorization: Bearer $administrator_token" "http://localhost:8080/v1/paths/$browser_goal_path_id/activities/manual-defaults")"
browser_goal_activity_payload="$(printf '%s' "$browser_goal_defaults" |
  python3 -c 'import datetime,json,sys;d=json.load(sys.stdin)["data"];start=datetime.datetime.fromisoformat(d["localDate"]+"T"+d["localStartTime"])-datetime.timedelta(seconds=45);print(json.dumps({"localDate":start.date().isoformat(),"localStartTime":start.time().isoformat(),"durationSeconds":45,"note":"goal reconfiguration evidence"},separators=(",",":")))')"
browser_goal_activity_key='browser-goal-activity-create-key-001'
browser_goal_activity_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $administrator_token" \
  -H "Idempotency-Key: $browser_goal_activity_key" \
  -H 'Content-Type: application/json' \
  --data "$browser_goal_activity_payload" \
  "http://localhost:8080/v1/paths/$browser_goal_path_id/activities")"
browser_goal_activity_id="$(printf '%s' "$browser_goal_activity_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];p=d.get("intervalProgress");assert d["activity"]["durationSeconds"]==45 and d["accumulatedSeconds"]==45 and p and p["accumulatedSeconds"]==45 and p["targetSeconds"]==60;print(d["activity"]["id"])')"
[[ -n "$browser_goal_activity_id" ]] || { echo "browser goal-update activity creation did not return an ID" >&2; exit 1; }

WEB_ACCEPTANCE_APPLICATION_TOKEN="$administrator_token" \
  WEB_ACCEPTANCE_BASE_URL="$web_base_url" \
  WEB_ACCEPTANCE_PARTICIPANT_TOKEN="$participant_token" \
  WEB_ACCEPTANCE_PATH_ID="$browser_goal_path_id" \
  WEB_ACCEPTANCE_PATH_NAME='Browser goal reconfiguration' \
  WEB_ACCEPTANCE_ALIGNMENT_WEEKDAY="$browser_goal_alignment_weekday" \
  node scripts/web-goal-update-acceptance.mjs

browser_goal_update_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$browser_goal_path_id" -v "activity_id=$browser_goal_activity_id" \
  -v "owner_id=$owner_id" -v "administrator_id=$administrator_id" -v "participant_id=$participant_id" <<'BROWSER_GOAL_UPDATE_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_models
   WHERE id = :'path_id'
     AND interval_goal_target_seconds IS NULL
     AND interval_goal_recurrence IS NULL
     AND interval_goal_start_minute IS NULL
     AND interval_goal_start_hour IS NULL
     AND interval_goal_start_weekday IS NULL
     AND interval_goal_start_day IS NULL
     AND interval_goal_start_month IS NULL
     AND overall_target_seconds IS NULL),
  (SELECT count(*) FROM recorded_activity_models
   WHERE id = :'activity_id' AND path_id = :'path_id' AND participant_id = :'administrator_id'
     AND FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))::bigint = 45),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'administrator_id' AND operation = 'path.goals.update' AND resource_id = :'path_id'),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'participant_id' AND operation = 'path.goals.update' AND resource_id = :'path_id'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'resource.updated' AND owner_user_id = :'owner_id' AND actor_user_id = :'administrator_id'
     AND target_type = 'path' AND target_id = :'path_id' AND outcome = 'succeeded'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'resource.access_denied' AND owner_user_id = :'participant_id' AND actor_user_id = :'participant_id'
     AND target_type = 'path' AND target_id = :'path_id' AND outcome = 'denied');
BROWSER_GOAL_UPDATE_EVIDENCE
)"
[[ "$browser_goal_update_evidence" == '1|1|2|0|2|0' ]] || {
  echo "browser goal update persistence mismatch: expected=1|1|2|0|2|0 actual=$browser_goal_update_evidence" >&2
  exit 1
}
echo "browser goal update confirmation, capability gating, removal, and persistence acceptance passed"

# PATH_07_ARCHIVE_ACCEPTANCE proves the lifecycle transition through the
# Dex-backed HTTP boundary. It intentionally uses two role-owned timers so the
# assertion covers every server-known timer on the Path rather than only the
# creator's timer.
archive_path_key='path-07-create-key-00000001'
archive_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H "Idempotency-Key: $archive_path_key" \
  -H 'Content-Type: application/json' \
  --data '{"name":"Archive acceptance","visibility":"private","overallTarget":{"targetSeconds":3600}}' \
  http://localhost:8080/v1/paths)"
archive_path_id="$(printf '%s' "$archive_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Archive acceptance" and d["visibility"]=="private" and d["overallTarget"]=={"targetSeconds":3600} and d["capabilities"]=={"trackTime":True,"inviteMembers":True,"manageMembers":True,"manageGoals":True,"manageLifecycle":True,"manageVisibility":True,"renamePath":True,"transferOwnership":True,"leavePath":False} and "archivedAt" not in d;print(d["id"])')"
[[ -n "$archive_path_id" ]] || { echo "PATH-07 Path creation did not return an ID" >&2; exit 1; }

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$archive_path_id" -v "participant_id=$participant_id" <<'PATH_07_MEMBERSHIP'
INSERT INTO path_membership_models (path_id, user_id, role)
VALUES (:'path_id', :'participant_id', 'participant');
PATH_07_MEMBERSHIP
(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run TestSeedPrivatePathHTTPAcceptanceRelationships -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$archive_path_id" \
    -acceptance-path-creator "$owner_id" \
    -acceptance-path-administrator "$administrator_id" \
    -acceptance-path-participant "$participant_id" \
    -acceptance-path-supporter "$supporter_id"
)

archive_manual_defaults="$(curl -fsS \
  -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/paths/$archive_path_id/activities/manual-defaults")"
archive_manual_payload="$(printf '%s' "$archive_manual_defaults" |
  python3 -c 'import datetime,json,sys;d=json.load(sys.stdin)["data"];start=datetime.datetime.fromisoformat(d["localDate"]+"T"+d["localStartTime"])-datetime.timedelta(seconds=60);print(json.dumps({"localDate":start.date().isoformat(),"localStartTime":start.time().isoformat(),"durationSeconds":60,"note":"must not be created while archived"},separators=(",",":")))')"

curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-07-owner-timer-start-001' \
  "http://localhost:8080/v1/paths/$archive_path_id/timer" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["running"] and d["timer"]["pathId"]==sys.argv[1]' "$archive_path_id"
curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: path-07-participant-start-001' \
  "http://localhost:8080/v1/paths/$archive_path_id/timer" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["running"] and d["timer"]["pathId"]==sys.argv[1]' "$archive_path_id"
sleep 2

# Dismissing the confirmation and a participant's concealed creator-only
# attempt must leave both timers running.
expect_status 400 -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-07-declined-archive-001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":false,"expectedArchived":false,"archived":true}' \
  "http://localhost:8080/v1/paths/$archive_path_id/archive-state"
expect_status 404 -X PUT \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: path-07-denied-archive-0001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedArchived":false,"archived":true}' \
  "http://localhost:8080/v1/paths/$archive_path_id/archive-state"
for role_token in "$owner_token" "$participant_token"; do
  curl -fsS -H "Authorization: Bearer $role_token" \
    "http://localhost:8080/v1/paths/$archive_path_id/timer" |
    python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["running"] and d["timer"]["pathId"]==sys.argv[1]' "$archive_path_id"
done

archive_state_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-07-confirmed-archive-01' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedArchived":false,"archived":true}' \
  "http://localhost:8080/v1/paths/$archive_path_id/archive-state")"
archive_instant="$(printf '%s' "$archive_state_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["id"]==sys.argv[1] and d["name"]=="Archive acceptance" and d["visibility"]=="private" and d["overallTarget"]=={"targetSeconds":3600} and d["capabilities"]=={"trackTime":False,"inviteMembers":False,"manageMembers":False,"manageGoals":False,"manageLifecycle":True,"manageVisibility":False,"renamePath":False,"transferOwnership":False,"leavePath":False};print(d["archivedAt"])' "$archive_path_id")"
[[ -n "$archive_instant" ]] || { echo "confirmed PATH-07 archive omitted archivedAt" >&2; exit 1; }

curl -fsS -H "Authorization: Bearer $owner_token" \
  'http://localhost:8080/v1/paths?limit=100' |
  python3 -c 'import json,sys;assert sys.argv[1] not in {p["id"] for p in json.load(sys.stdin)["data"]}' "$archive_path_id"
curl -fsS -H "Authorization: Bearer $owner_token" \
  'http://localhost:8080/v1/paths?archived=true&limit=100' |
  ARCHIVE_INSTANT="$archive_instant" python3 -c 'import json,os,sys;items={p["id"]:p for p in json.load(sys.stdin)["data"]};p=items[sys.argv[1]];assert p["archivedAt"]==os.environ["ARCHIVE_INSTANT"] and p["overallTarget"]=={"targetSeconds":3600} and p["capabilities"]=={"trackTime":False,"inviteMembers":False,"manageMembers":False,"manageGoals":False,"manageLifecycle":True,"manageVisibility":False,"renamePath":False,"transferOwnership":False,"leavePath":False}' "$archive_path_id"
for role_and_capabilities in \
  "$owner_token|{\"trackTime\":false,\"inviteMembers\":false,\"manageMembers\":false,\"manageGoals\":false,\"manageLifecycle\":true,\"manageVisibility\":false,\"renamePath\":false,\"transferOwnership\":false,\"leavePath\":false}" \
  "$participant_token|{\"trackTime\":false,\"inviteMembers\":false,\"manageMembers\":false,\"manageGoals\":false,\"manageLifecycle\":false,\"manageVisibility\":false,\"renamePath\":false,\"transferOwnership\":false,\"leavePath\":false}"; do
  IFS='|' read -r role_token expected_capabilities <<<"$role_and_capabilities"
  curl -fsS -H "Authorization: Bearer $role_token" \
    "http://localhost:8080/v1/paths/$archive_path_id" |
    ARCHIVE_INSTANT="$archive_instant" EXPECTED_CAPABILITIES="$expected_capabilities" python3 -c 'import json,os,sys;p=json.load(sys.stdin)["data"];assert p["id"]==sys.argv[1] and p["archivedAt"]==os.environ["ARCHIVE_INSTANT"] and p["overallTarget"]=={"targetSeconds":3600} and p["capabilities"]==json.loads(os.environ["EXPECTED_CAPABILITIES"])' "$archive_path_id"
done

archive_history="$(curl -fsS \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$archive_path_id/activities?limit=25")"
archive_activity_ids="$(printf '%s' "$archive_history" |
  OWNER_ID="$owner_id" PARTICIPANT_ID="$participant_id" python3 -c 'import json,os,sys;items=json.load(sys.stdin)["data"];assert len(items)==2;assert {i["activity"]["participantId"] for i in items}=={os.environ["OWNER_ID"],os.environ["PARTICIPANT_ID"]};assert all(i["activity"]["durationSeconds"]>=1 for i in items);print("|".join(sorted(i["activity"]["id"] for i in items)))')"
[[ -n "$archive_activity_ids" ]] || { echo "PATH-07 archive did not retain stopped timers as history" >&2; exit 1; }

# Archived Paths are view-only: both a new timer and manual activity are
# rejected at the live persistence boundary.
expect_status 409 -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-07-archived-start-0001' \
  "http://localhost:8080/v1/paths/$archive_path_id/timer"
expect_status 409 -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-07-archived-manual-001' \
  -H 'Content-Type: application/json' \
  --data "$archive_manual_payload" \
  "http://localhost:8080/v1/paths/$archive_path_id/activities"

unarchive_state_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: path-07-confirm-unarchive-001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedArchived":true,"archived":false}' \
  "http://localhost:8080/v1/paths/$archive_path_id/archive-state")"
printf '%s' "$unarchive_state_response" |
  python3 -c 'import json,sys;p=json.load(sys.stdin)["data"];assert p["id"]==sys.argv[1] and p["name"]=="Archive acceptance" and p["overallTarget"]=={"targetSeconds":3600} and p["capabilities"]=={"trackTime":True,"inviteMembers":True,"manageMembers":True,"manageGoals":True,"manageLifecycle":True,"manageVisibility":True,"renamePath":True,"transferOwnership":True,"leavePath":False} and "archivedAt" not in p' "$archive_path_id"

for role_token in "$owner_token" "$participant_token"; do
  curl -fsS -H "Authorization: Bearer $role_token" \
    "http://localhost:8080/v1/paths/$archive_path_id/timer" |
    python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert not d["running"] and "timer" not in d'
done
unarchived_history="$(curl -fsS \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$archive_path_id/activities?limit=25")"
unarchived_activity_ids="$(printf '%s' "$unarchived_history" |
  python3 -c 'import json,sys;print("|".join(sorted(i["activity"]["id"] for i in json.load(sys.stdin)["data"])))')"
[[ "$unarchived_activity_ids" == "$archive_activity_ids" ]] || {
  echo "PATH-07 unarchive restarted a timer or changed retained activity: before=$archive_activity_ids after=$unarchived_activity_ids" >&2
  exit 1
}

path_07_evidence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$archive_path_id" -v "owner_id=$owner_id" -v "participant_id=$participant_id" <<'PATH_07_EVIDENCE'
SELECT
  (SELECT count(*) FROM path_models
   WHERE id = :'path_id' AND owner_user_id = :'owner_id' AND archived_at IS NULL
     AND name = 'Archive acceptance' AND visibility = 'private' AND overall_target_seconds = 3600),
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'participant_id' AND role = 'participant'),
  (SELECT count(*) FROM running_timer_models WHERE path_id = :'path_id'),
  (SELECT count(*) FROM recorded_activity_models WHERE path_id = :'path_id'),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'owner_id' AND operation = 'path.archive-state.set' AND resource_id = :'path_id'),
  (SELECT count(*) FROM idempotency_models
   WHERE principal_id = :'participant_id' AND operation = 'path.archive-state.set' AND resource_id = :'path_id'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'resource.updated' AND owner_user_id = :'owner_id' AND actor_user_id = :'owner_id'
     AND target_type = 'path' AND target_id = :'path_id' AND outcome = 'succeeded'),
  (SELECT count(*) FROM audit_event_models
   WHERE action = 'resource.access_denied' AND owner_user_id = :'participant_id' AND actor_user_id = :'participant_id'
     AND target_type = 'path' AND target_id = :'path_id' AND outcome = 'denied');
PATH_07_EVIDENCE
)"
[[ "$path_07_evidence" == '1|1|0|2|2|0|2|1' ]] || {
  echo "PATH-07 persistence mismatch: expected=1|1|0|2|2|0|2|1 actual=$path_07_evidence" >&2
  exit 1
}
echo "PATH-07 creator confirmation, timer save, archived projection/read-only state, and non-restarting unarchive acceptance passed"

# NOTE_03A_MANAGER_REMOVAL proves the narrow loss-of-access lifecycle with two
# real application accounts. A Path nudge is stale after the manager removes
# its supporter, while an unrelated invitation and the explanatory removal
# notice remain authoritative. Pausing the controlled provider makes the
# not-yet-handed-off nudge push observable without relying on a timing race.
expect_status 204 -X PUT \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Content-Type: application/json' \
  --data '{"provider":"expo","platform":"ios","locale":"en","token":"ExponentPushToken[note-03a-supporter]"}' \
  'http://localhost:8080/v1/push-installations/note-03a-supporter-installation'

# Use a dedicated, product-created Path and invitation so this scenario proves
# the reachable nudge lifecycle for a progress-tracking participant without
# inheriting membership or rate-limit state from earlier acceptance slices.
note_03a_target_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03a-target-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Notification access cleanup","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03a_target_path_id="$(printf '%s' "$note_03a_target_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Notification access cleanup" and d["visibility"]=="private";print(d["id"])')"
note_03a_target_invitation_body="$(SUPPORTER_ID="$supporter_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.supporter","expectedRecipientUserId":os.environ["SUPPORTER_ID"],"offeredRole":"participant"},separators=(",",":")))')"
note_03a_target_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03a-target-invitation-key-01' \
  -H 'Content-Type: application/json' \
  --data "$note_03a_target_invitation_body" \
  "http://localhost:8080/v1/paths/$note_03a_target_path_id/invitations")"
note_03a_target_invitation_id="$(printf '%s' "$note_03a_target_invitation_response" |
  PATH_ID="$note_03a_target_path_id" SUPPORTER_ID="$supporter_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["SUPPORTER_ID"] and d["offeredRole"]=="participant";print(d["id"])')"
note_03a_target_accept_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $supporter_token" \
  -H 'Idempotency-Key: note-03a-target-accept-key-0001' \
  "http://localhost:8080/v1/path-invitations/$note_03a_target_invitation_id/accept")"
printf '%s' "$note_03a_target_accept_response" |
  PATH_ID="$note_03a_target_path_id" SUPPORTER_ID="$supporter_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["SUPPORTER_ID"] and d["offeredRole"]=="participant" and d["acceptedAt"]'

note_03a_unrelated_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03a-unrelated-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Unrelated notification control","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03a_unrelated_path_id="$(printf '%s' "$note_03a_unrelated_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Unrelated notification control" and d["visibility"]=="private";print(d["id"])')"
note_03a_unrelated_invitation_body="$(SUPPORTER_ID="$supporter_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.supporter","expectedRecipientUserId":os.environ["SUPPORTER_ID"],"offeredRole":"supporter"},separators=(",",":")))')"
note_03a_unrelated_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03a-unrelated-invitation-key-01' \
  -H 'Content-Type: application/json' \
  --data "$note_03a_unrelated_invitation_body" \
  "http://localhost:8080/v1/paths/$note_03a_unrelated_path_id/invitations")"
note_03a_unrelated_invitation_id="$(printf '%s' "$note_03a_unrelated_invitation_response" |
  PATH_ID="$note_03a_unrelated_path_id" SUPPORTER_ID="$supporter_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["SUPPORTER_ID"] and d["offeredRole"]=="supporter";print(d["id"])')"
note_03a_before_nudge="$(curl -fsS -H "Authorization: Bearer $supporter_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
note_03a_unrelated_notification_id="$(printf '%s' "$note_03a_before_nudge" |
  INVITATION_ID="$note_03a_unrelated_invitation_id" PATH_ID="$note_03a_unrelated_path_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=[item for item in d["data"] if item.get("invitationId","")==os.environ["INVITATION_ID"]];assert len(items)==1;i=items[0];assert i["type"]=="path_invitation_received" and i["pathId"]==os.environ["PATH_ID"] and i["read"] is False;print(i["id"])')"
note_03a_unrelated_before="$(printf '%s' "$note_03a_before_nudge" |
  NOTIFICATION_ID="$note_03a_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[item for item in json.load(sys.stdin)["data"] if item["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
note_03a_unread_before="$(printf '%s' "$note_03a_before_nudge" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["meta"]["unreadCount"])')"
[[ "$note_03a_unread_before" =~ ^[0-9]+$ ]] || {
  echo "NOTE-03A unread baseline was not an integer: $note_03a_unread_before" >&2
  exit 1
}
wait_for_push "$note_03a_unrelated_notification_id" >/dev/null

docker compose stop push-provider
note_03a_nudge_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03a-nudge-key-000000001' \
  -H 'Content-Type: application/json' \
  --data '{"content":{"kind":"preset","preset":"keep_it_going"}}' \
  "http://localhost:8080/v1/paths/$note_03a_target_path_id/members/$supporter_id/nudges")"
note_03a_nudge_id="$(printf '%s' "$note_03a_nudge_response" |
  PATH_ID="$note_03a_target_path_id" SUPPORTER_ID="$supporter_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["SUPPORTER_ID"] and d["content"]=={"kind":"preset","preset":"keep_it_going"};print(d["id"])')"
note_03a_after_nudge="$(curl -fsS -H "Authorization: Bearer $supporter_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
note_03a_nudge_notification_id="$(printf '%s' "$note_03a_after_nudge" |
  PATH_ID="$note_03a_target_path_id" NUDGE_ID="$note_03a_nudge_id" EXPECTED_UNREAD_COUNT="$((note_03a_unread_before + 1))" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=[item for item in d["data"] if item["type"]=="nudge_received" and item.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1 and d["meta"]["unreadCount"]==int(os.environ["EXPECTED_UNREAD_COUNT"]);i=items[0];assert i["read"] is False and "nudgeId" not in i;print(i["id"])')"

# Wait until the worker has attempted the intentionally unavailable provider
# and durably queued the old nudge for retry. Restarting before removal then
# lets the explanatory access-loss push hand off immediately, while removal
# must suppress the still-pending nudge retry atomically.
note_03a_nudge_retry_queued=''
for _ in $(seq 1 100); do
  note_03a_nudge_retry_queued="$(docker compose exec -T postgres psql -At -U app -d app \
    -v "notification_id=$note_03a_nudge_notification_id" <<'NOTE_03A_NUDGE_RETRY'
SELECT count(*)
FROM notification_push_delivery_models
WHERE notification_id = :'notification_id'
  AND failure_code = 'network'
  AND available_at > CURRENT_TIMESTAMP
  AND delivered_at IS NULL
  AND suppressed_at IS NULL
  AND permanently_failed_at IS NULL;
NOTE_03A_NUDGE_RETRY
)"
  [[ "$note_03a_nudge_retry_queued" == 1 ]] && break
  sleep 0.1
done
[[ "$note_03a_nudge_retry_queued" == 1 ]] || {
  echo "NOTE-03A nudge was not durably queued while the provider was unavailable" >&2
  exit 1
}
docker compose start push-provider
note_03a_provider_health=''
for _ in $(seq 1 100); do
  note_03a_provider_health="$(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q push-provider)")"
  [[ "$note_03a_provider_health" == healthy ]] && break
  sleep 0.1
done
[[ "$note_03a_provider_health" == healthy ]] || {
  echo "NOTE-03A controlled push provider did not become healthy after restart" >&2
  exit 1
}

note_03a_removal_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03a-member-removal-key-001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedRole":"participant"}' \
  "http://localhost:8080/v1/paths/$note_03a_target_path_id/members/$supporter_id")"
printf '%s' "$note_03a_removal_response" |
  PATH_ID="$note_03a_target_path_id" SUPPORTER_ID="$supporter_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"pathId":os.environ["PATH_ID"],"userId":os.environ["SUPPORTER_ID"],"removed":True,"activityDeleted":True}'

note_03a_after_removal=''
for _ in $(seq 1 40); do
  note_03a_after_removal="$(curl -fsS -H "Authorization: Bearer $supporter_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  if printf '%s' "$note_03a_after_removal" |
    PATH_ID="$note_03a_target_path_id" OWNER_ID="$owner_id" NUDGE_NOTIFICATION_ID="$note_03a_nudge_notification_id" \
    UNRELATED_NOTIFICATION_ID="$note_03a_unrelated_notification_id" EXPECTED_UNREAD_COUNT="$((note_03a_unread_before + 1))" python3 -c \
      'import json,os,sys;d=json.load(sys.stdin);items=d["data"];removals=[item for item in items if item["type"]=="path_member_removed" and item.get("pathId")==os.environ["PATH_ID"]];assert len(removals)==1;r=removals[0];assert r["presentation"]=="informational" and r["read"] is False and r["offeredRole"]=="participant" and r["actor"]["userId"]==os.environ["OWNER_ID"];assert all(item["id"]!=os.environ["NUDGE_NOTIFICATION_ID"] for item in items);assert any(item["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for item in items);assert d["meta"]["unreadCount"]==int(os.environ["EXPECTED_UNREAD_COUNT"])' 2>/dev/null; then
    break
  fi
  sleep 0.1
done
note_03a_removal_notification_id="$(printf '%s' "$note_03a_after_removal" |
  PATH_ID="$note_03a_target_path_id" python3 -c \
    'import json,os,sys;items=[item for item in json.load(sys.stdin)["data"] if item["type"]=="path_member_removed" and item.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1;print(items[0]["id"])')"
note_03a_unrelated_after="$(printf '%s' "$note_03a_after_removal" |
  UNRELATED_NOTIFICATION_ID="$note_03a_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[item for item in json.load(sys.stdin)["data"] if item["id"]==os.environ["UNRELATED_NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
[[ "$note_03a_unrelated_after" == "$note_03a_unrelated_before" ]] || {
  echo "NOTE-03A removal changed the unrelated notification" >&2
  exit 1
}

note_03a_removed_nudge_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $supporter_token" \
  "http://localhost:8080/v1/notifications/$note_03a_nudge_notification_id")"
note_03a_missing_notification_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $supporter_token" \
  'http://localhost:8080/v1/notifications/note-03a-nonexistent-notification')"
[[ "$note_03a_removed_nudge_response" == "$note_03a_missing_notification_response" &&
   "$note_03a_removed_nudge_response" == *'|404' ]] || {
  echo "NOTE-03A removed and nonexistent notification responses differed" >&2
  exit 1
}

note_03a_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$note_03a_target_path_id" -v "supporter_id=$supporter_id" \
  -v "nudge_id=$note_03a_nudge_id" -v "nudge_notification_id=$note_03a_nudge_notification_id" \
  -v "removal_notification_id=$note_03a_removal_notification_id" \
  -v "unrelated_notification_id=$note_03a_unrelated_notification_id" <<'NOTE_03A_PERSISTENCE'
SELECT
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'supporter_id'),
  (SELECT count(*) FROM notification_models
   WHERE id = :'nudge_notification_id' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'removal_notification_id' AND recipient_user_id = :'supporter_id'
     AND path_id = :'path_id' AND kind = 'path_member_removed'
     AND presentation_class = 'informational' AND offered_role = 'participant'
     AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'unrelated_notification_id' AND recipient_user_id = :'supporter_id'
     AND kind = 'path_invitation_received' AND deleted_at IS NULL),
  NOT EXISTS (
    SELECT 1 FROM notification_push_delivery_models
    WHERE notification_id = :'nudge_notification_id'
      AND delivered_at IS NULL AND suppressed_at IS NULL
      AND permanently_failed_at IS NULL
  );
NOTE_03A_PERSISTENCE
)"
[[ "$note_03a_persistence" == '0|0|1|1|t' ]] || {
  echo "NOTE-03A persistence mismatch: expected=0|0|1|1|t actual=$note_03a_persistence" >&2
  exit 1
}

wait_for_push "$note_03a_removal_notification_id" >/dev/null
PUSH_NOTIFICATION_ID="$note_03a_nudge_notification_id" python3 -c \
  'import json,os,sys;p=os.environ["PUSH_NOTIFICATION_ID"];rows=[json.loads(line) for line in open(sys.argv[1],encoding="utf-8") if line.endswith("\n") and line.strip()];assert all(row.get("data",{}).get("notificationId")!=p for row in rows)' \
  "$push_provider_log"
echo "NOTE-03A manager removal stale-nudge cleanup, unread convergence, opaque lookup, unrelated retention, and queued-push suppression acceptance passed"

# NOTE_03B_VOLUNTARY_LEAVE proves that the same loss-of-access cleanup occurs
# when a participant chooses to leave a private Path. The participant's
# unrelated notification and the creator's explanatory member-left notice stay
# authoritative while the stale nudge and its pending push retry disappear.
expect_status 204 -X PUT \
  -H "Authorization: Bearer $participant_token" \
  -H 'Content-Type: application/json' \
  --data '{"provider":"expo","platform":"ios","locale":"en","token":"ExponentPushToken[note-03b-participant]"}' \
  'http://localhost:8080/v1/push-installations/note-03b-participant-installation'

note_03b_target_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03b-target-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Voluntary leave notification cleanup","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03b_target_path_id="$(printf '%s' "$note_03b_target_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Voluntary leave notification cleanup" and d["visibility"]=="private";print(d["id"])')"
note_03b_target_invitation_body="$(PARTICIPANT_ID="$participant_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.participant","expectedRecipientUserId":os.environ["PARTICIPANT_ID"],"offeredRole":"participant"},separators=(",",":")))')"
note_03b_target_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03b-target-invitation-key-01' \
  -H 'Content-Type: application/json' \
  --data "$note_03b_target_invitation_body" \
  "http://localhost:8080/v1/paths/$note_03b_target_path_id/invitations")"
note_03b_target_invitation_id="$(printf '%s' "$note_03b_target_invitation_response" |
  PATH_ID="$note_03b_target_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["PARTICIPANT_ID"] and d["offeredRole"]=="participant";print(d["id"])')"
note_03b_target_accept_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: note-03b-target-accept-key-0001' \
  "http://localhost:8080/v1/path-invitations/$note_03b_target_invitation_id/accept")"
printf '%s' "$note_03b_target_accept_response" |
  PATH_ID="$note_03b_target_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["PARTICIPANT_ID"] and d["offeredRole"]=="participant" and d["acceptedAt"]'

note_03b_unrelated_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03b-unrelated-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Voluntary leave unrelated control","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03b_unrelated_path_id="$(printf '%s' "$note_03b_unrelated_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Voluntary leave unrelated control";print(d["id"])')"
note_03b_unrelated_invitation_body="$(PARTICIPANT_ID="$participant_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.participant","expectedRecipientUserId":os.environ["PARTICIPANT_ID"],"offeredRole":"supporter"},separators=(",",":")))')"
note_03b_unrelated_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03b-unrelated-invitation-key-01' \
  -H 'Content-Type: application/json' \
  --data "$note_03b_unrelated_invitation_body" \
  "http://localhost:8080/v1/paths/$note_03b_unrelated_path_id/invitations")"
note_03b_unrelated_invitation_id="$(printf '%s' "$note_03b_unrelated_invitation_response" |
  PATH_ID="$note_03b_unrelated_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["PARTICIPANT_ID"] and d["offeredRole"]=="supporter";print(d["id"])')"
note_03b_before_nudge="$(curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
note_03b_unrelated_notification_id="$(printf '%s' "$note_03b_before_nudge" |
  INVITATION_ID="$note_03b_unrelated_invitation_id" PATH_ID="$note_03b_unrelated_path_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i.get("invitationId")==os.environ["INVITATION_ID"]];assert len(items)==1 and items[0]["type"]=="path_invitation_received" and items[0]["pathId"]==os.environ["PATH_ID"] and items[0]["read"] is False;print(items[0]["id"])')"
note_03b_unrelated_before="$(printf '%s' "$note_03b_before_nudge" |
  NOTIFICATION_ID="$note_03b_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
note_03b_unread_before="$(printf '%s' "$note_03b_before_nudge" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["meta"]["unreadCount"])')"
[[ "$note_03b_unread_before" =~ ^[0-9]+$ ]] || {
  echo "NOTE-03B unread baseline was not an integer: $note_03b_unread_before" >&2
  exit 1
}
wait_for_push "$note_03b_unrelated_notification_id" >/dev/null

docker compose stop push-provider
note_03b_nudge_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03b-nudge-key-000000001' \
  -H 'Content-Type: application/json' \
  --data '{"content":{"kind":"preset","preset":"keep_it_going"}}' \
  "http://localhost:8080/v1/paths/$note_03b_target_path_id/members/$participant_id/nudges")"
note_03b_nudge_id="$(printf '%s' "$note_03b_nudge_response" |
  PATH_ID="$note_03b_target_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["pathId"]==os.environ["PATH_ID"] and d["recipientUserId"]==os.environ["PARTICIPANT_ID"];print(d["id"])')"
note_03b_after_nudge="$(curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
note_03b_nudge_notification_id="$(printf '%s' "$note_03b_after_nudge" |
  PATH_ID="$note_03b_target_path_id" EXPECTED_UNREAD_COUNT="$((note_03b_unread_before + 1))" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=[i for i in d["data"] if i["type"]=="nudge_received" and i.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1 and items[0]["read"] is False and d["meta"]["unreadCount"]==int(os.environ["EXPECTED_UNREAD_COUNT"]);print(items[0]["id"])')"

note_03b_nudge_retry_queued=''
for _ in $(seq 1 100); do
  note_03b_nudge_retry_queued="$(docker compose exec -T postgres psql -At -U app -d app \
    -v "notification_id=$note_03b_nudge_notification_id" <<'NOTE_03B_NUDGE_RETRY'
SELECT count(*)
FROM notification_push_delivery_models
WHERE notification_id = :'notification_id'
  AND failure_code = 'network'
  AND available_at > CURRENT_TIMESTAMP
  AND delivered_at IS NULL
  AND suppressed_at IS NULL
  AND permanently_failed_at IS NULL;
NOTE_03B_NUDGE_RETRY
)"
  [[ "$note_03b_nudge_retry_queued" == 1 ]] && break
  sleep 0.1
done
[[ "$note_03b_nudge_retry_queued" == 1 ]] || {
  echo "NOTE-03B nudge was not durably queued while the provider was unavailable" >&2
  exit 1
}
docker compose start push-provider
note_03b_provider_health=''
for _ in $(seq 1 100); do
  note_03b_provider_health="$(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q push-provider)")"
  [[ "$note_03b_provider_health" == healthy ]] && break
  sleep 0.1
done
[[ "$note_03b_provider_health" == healthy ]] || {
  echo "NOTE-03B controlled push provider did not become healthy after restart" >&2
  exit 1
}

note_03b_leave_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: note-03b-leave-key-000000001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"retainActivity":true}' \
  "http://localhost:8080/v1/paths/$note_03b_target_path_id/membership")"
printf '%s' "$note_03b_leave_response" |
  PATH_ID="$note_03b_target_path_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)["data"]=={"pathId":os.environ["PATH_ID"],"left":True,"activityRetained":True}'

note_03b_after_leave=''
note_03b_owner_after_leave=''
for _ in $(seq 1 40); do
  note_03b_after_leave="$(curl -fsS -H "Authorization: Bearer $participant_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  note_03b_owner_after_leave="$(curl -fsS -H "Authorization: Bearer $owner_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  if printf '%s' "$note_03b_after_leave" |
    NUDGE_NOTIFICATION_ID="$note_03b_nudge_notification_id" UNRELATED_NOTIFICATION_ID="$note_03b_unrelated_notification_id" \
    EXPECTED_UNREAD_COUNT="$note_03b_unread_before" python3 -c \
      'import json,os,sys;d=json.load(sys.stdin);items=d["data"];assert all(i["id"]!=os.environ["NUDGE_NOTIFICATION_ID"] for i in items);assert any(i["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for i in items);assert d["meta"]["unreadCount"]==int(os.environ["EXPECTED_UNREAD_COUNT"])' 2>/dev/null &&
    printf '%s' "$note_03b_owner_after_leave" |
      PATH_ID="$note_03b_target_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
        'import json,os,sys;items=[item for item in json.load(sys.stdin)["data"] if item["type"]=="path_member_left" and item.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1;i=items[0];assert i["presentation"]=="informational" and i["read"] is False and i["actor"]["userId"]==os.environ["PARTICIPANT_ID"]' 2>/dev/null; then
    break
  fi
  sleep 0.1
done
note_03b_manager_notification_id="$(printf '%s' "$note_03b_owner_after_leave" |
  PATH_ID="$note_03b_target_path_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["type"]=="path_member_left" and i.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1;print(items[0]["id"])')"
note_03b_unrelated_after="$(printf '%s' "$note_03b_after_leave" |
  UNRELATED_NOTIFICATION_ID="$note_03b_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["id"]==os.environ["UNRELATED_NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
[[ "$note_03b_unrelated_after" == "$note_03b_unrelated_before" ]] || {
  echo "NOTE-03B leave changed the unrelated notification" >&2
  exit 1
}

note_03b_removed_nudge_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/notifications/$note_03b_nudge_notification_id")"
note_03b_missing_notification_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications/note-03b-nonexistent-notification')"
[[ "$note_03b_removed_nudge_response" == "$note_03b_missing_notification_response" &&
   "$note_03b_removed_nudge_response" == *'|404' ]] || {
  echo "NOTE-03B removed and nonexistent notification responses differed" >&2
  exit 1
}

note_03b_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$note_03b_target_path_id" -v "participant_id=$participant_id" \
  -v "nudge_notification_id=$note_03b_nudge_notification_id" \
  -v "manager_notification_id=$note_03b_manager_notification_id" \
  -v "unrelated_notification_id=$note_03b_unrelated_notification_id" <<'NOTE_03B_PERSISTENCE'
SELECT
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'participant_id'),
  (SELECT count(*) FROM notification_models
   WHERE id = :'nudge_notification_id' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'manager_notification_id' AND path_id = :'path_id'
     AND kind = 'path_member_left' AND actor_user_id = :'participant_id'
     AND presentation_class = 'informational' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'unrelated_notification_id' AND recipient_user_id = :'participant_id'
     AND kind = 'path_invitation_received' AND deleted_at IS NULL),
  NOT EXISTS (
    SELECT 1 FROM notification_push_delivery_models
    WHERE notification_id = :'nudge_notification_id'
      AND delivered_at IS NULL AND suppressed_at IS NULL
      AND permanently_failed_at IS NULL
  );
NOTE_03B_PERSISTENCE
)"
[[ "$note_03b_persistence" == '0|0|1|1|t' ]] || {
  echo "NOTE-03B persistence mismatch: expected=0|0|1|1|t actual=$note_03b_persistence" >&2
  exit 1
}

wait_for_push "$note_03b_manager_notification_id" >/dev/null
PUSH_NOTIFICATION_ID="$note_03b_nudge_notification_id" python3 -c \
  'import json,os,sys;p=os.environ["PUSH_NOTIFICATION_ID"];rows=[json.loads(line) for line in open(sys.argv[1],encoding="utf-8") if line.endswith("\n") and line.strip()];assert all(row.get("data",{}).get("notificationId")!=p for row in rows)' \
  "$push_provider_log"
echo "NOTE-03B voluntary leave stale-nudge cleanup, unread convergence, opaque lookup, unrelated and manager-notice retention, and queued-push suppression acceptance passed"

# NOTE_03C_HIDDEN_RETAINED_ACTIVITY proves that voluntary retain-activity leave
# retires a target-opening interaction notice even though the underlying
# activity, feed event, and interaction remain durable for a possible rejoin.
# Keeping the controlled provider stopped through leave makes suppression occur
# before any retry can race with the membership transition.
note_03c_target_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03c-target-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Retained activity notification cleanup","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03c_target_path_id="$(printf '%s' "$note_03c_target_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Retained activity notification cleanup" and d["visibility"]=="private";print(d["id"])')"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$note_03c_target_path_id" -v "participant_id=$participant_id" <<'NOTE_03C_ACTIVITY_MEMBERSHIP'
INSERT INTO path_membership_models (path_id, user_id, role, joined_at)
SELECT id, :'participant_id', 'participant', date_trunc('minute', created_at)
FROM path_models
WHERE id = :'path_id';
NOTE_03C_ACTIVITY_MEMBERSHIP
(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run TestSeedPathHTTPAcceptanceParticipant -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$note_03c_target_path_id" \
    -acceptance-path-participant "$participant_id"
)

note_03c_activity_defaults="$(curl -fsS \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$note_03c_target_path_id/activities/manual-defaults")"
note_03c_activity_payload="$(printf '%s' "$note_03c_activity_defaults" | python3 -c \
  'import json,sys;d=json.load(sys.stdin)["data"];print(json.dumps({"localDate":d["localDate"],"localStartTime":d["localStartTime"],"durationSeconds":1,"note":"Retain this practice"},separators=(",",":")))')"
sleep 1
note_03c_activity_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: note-03c-activity-create-key-0001' \
  -H 'Content-Type: application/json' \
  --data "$note_03c_activity_payload" \
  "http://localhost:8080/v1/paths/$note_03c_target_path_id/activities")"
note_03c_activity_id="$(printf '%s' "$note_03c_activity_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["activity"]["durationSeconds"]==1 and d["activity"]["note"]=="Retain this practice";print(d["activity"]["id"])')"
note_03c_event_id="practice:$note_03c_activity_id"
curl -fsS -H "Authorization: Bearer $owner_token" \
  "http://localhost:8080/v1/social/feed/$note_03c_event_id" |
  EVENT_ID="$note_03c_event_id" PATH_ID="$note_03c_target_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["EVENT_ID"] and d["path"]["id"]==os.environ["PATH_ID"] and d["participant"]["userId"]==os.environ["PARTICIPANT_ID"]'

note_03c_unrelated_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03c-unrelated-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Retained activity unrelated control","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03c_unrelated_path_id="$(printf '%s' "$note_03c_unrelated_path_response" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')"
note_03c_unrelated_invitation_body="$(PARTICIPANT_ID="$participant_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.participant","expectedRecipientUserId":os.environ["PARTICIPANT_ID"],"offeredRole":"supporter"},separators=(",",":")))')"
note_03c_unrelated_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03c-unrelated-invitation-key-01' \
  -H 'Content-Type: application/json' \
  --data "$note_03c_unrelated_invitation_body" \
  "http://localhost:8080/v1/paths/$note_03c_unrelated_path_id/invitations")"
note_03c_unrelated_invitation_id="$(printf '%s' "$note_03c_unrelated_invitation_response" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')"
note_03c_before_interaction="$(curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
note_03c_unrelated_notification_id="$(printf '%s' "$note_03c_before_interaction" |
  INVITATION_ID="$note_03c_unrelated_invitation_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i.get("invitationId")==os.environ["INVITATION_ID"]];assert len(items)==1 and items[0]["type"]=="path_invitation_received" and items[0]["read"] is False;print(items[0]["id"])')"
note_03c_unrelated_before="$(printf '%s' "$note_03c_before_interaction" |
  NOTIFICATION_ID="$note_03c_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
note_03c_unread_before="$(printf '%s' "$note_03c_before_interaction" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["meta"]["unreadCount"])')"
[[ "$note_03c_unread_before" =~ ^[0-9]+$ ]] || {
  echo "NOTE-03C unread baseline was not an integer: $note_03c_unread_before" >&2
  exit 1
}
wait_for_push "$note_03c_unrelated_notification_id" >/dev/null

docker compose stop push-provider
note_03c_comment_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03c-comment-create-key-00001' \
  -H 'Content-Type: application/json' \
  --data '{"text":"Keep going"}' \
  "http://localhost:8080/v1/social/feed/$note_03c_event_id/comments")"
note_03c_comment_id="$(printf '%s' "$note_03c_comment_response" |
  EVENT_ID="$note_03c_event_id" OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["eventId"]==os.environ["EVENT_ID"] and d["authorUserId"]==os.environ["OWNER_ID"] and d["text"]=="Keep going";print(d["id"])')"
note_03c_interaction_notification_id="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "event_id=$note_03c_event_id" -v "comment_id=$note_03c_comment_id" \
  -v "owner_id=$owner_id" -v "participant_id=$participant_id" <<'NOTE_03C_INTERACTION_NOTIFICATION'
SELECT id FROM notification_models
WHERE kind = 'practice_comment'
  AND social_feed_event_id = :'event_id'
  AND comment_id = :'comment_id'
  AND actor_user_id = :'owner_id'
  AND recipient_user_id = :'participant_id'
  AND deleted_at IS NULL;
NOTE_03C_INTERACTION_NOTIFICATION
)"
[[ -n "$note_03c_interaction_notification_id" ]] || {
  echo "NOTE-03C comment did not create an interaction notification" >&2
  exit 1
}

note_03c_interaction_queued=''
for _ in $(seq 1 150); do
  note_03c_interaction_queued="$(docker compose exec -T postgres psql -At -U app -d app \
    -v "notification_id=$note_03c_interaction_notification_id" <<'NOTE_03C_INTERACTION_QUEUED'
SELECT count(*)
FROM notification_push_delivery_models
WHERE notification_id = :'notification_id'
  AND delivered_at IS NULL
  AND suppressed_at IS NULL
  AND permanently_failed_at IS NULL;
NOTE_03C_INTERACTION_QUEUED
)"
  [[ "$note_03c_interaction_queued" == 1 ]] && break
  sleep 0.1
done
[[ "$note_03c_interaction_queued" == 1 ]] || {
  echo "NOTE-03C interaction push was not durably queued while the provider was unavailable" >&2
  exit 1
}
note_03c_after_interaction=''
for _ in $(seq 1 100); do
  note_03c_after_interaction="$(curl -fsS -H "Authorization: Bearer $participant_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  if printf '%s' "$note_03c_after_interaction" |
    NOTIFICATION_ID="$note_03c_interaction_notification_id" python3 -c \
      'import json,os,sys;assert any(i["id"]==os.environ["NOTIFICATION_ID"] for i in json.load(sys.stdin)["data"])' 2>/dev/null; then
    break
  fi
  sleep 0.1
done
printf '%s' "$note_03c_after_interaction" |
  NOTIFICATION_ID="$note_03c_interaction_notification_id" EVENT_ID="$note_03c_event_id" \
  COMMENT_ID="$note_03c_comment_id" OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=[i for i in d["data"] if i["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1 and type(d["meta"]["unreadCount"]) is int and d["meta"]["unreadCount"]>=1;i=items[0];assert i["type"]=="practice_comment" and i["socialFeedEventId"]==os.environ["EVENT_ID"] and i["commentId"]==os.environ["COMMENT_ID"] and i["actor"]["userId"]==os.environ["OWNER_ID"] and i["read"] is False'

note_03c_leave_response="$(curl -fsS -X DELETE \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: note-03c-leave-key-000000001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"retainActivity":true}' \
  "http://localhost:8080/v1/paths/$note_03c_target_path_id/membership")"
printf '%s' "$note_03c_leave_response" |
  PATH_ID="$note_03c_target_path_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)["data"]=={"pathId":os.environ["PATH_ID"],"left":True,"activityRetained":True}'

note_03c_after_leave=''
note_03c_owner_after_leave=''
for _ in $(seq 1 40); do
  note_03c_after_leave="$(curl -fsS -H "Authorization: Bearer $participant_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  note_03c_owner_after_leave="$(curl -fsS -H "Authorization: Bearer $owner_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  if printf '%s' "$note_03c_after_leave" |
    INTERACTION_NOTIFICATION_ID="$note_03c_interaction_notification_id" UNRELATED_NOTIFICATION_ID="$note_03c_unrelated_notification_id" \
    python3 -c \
      'import json,os,sys;d=json.load(sys.stdin);items=d["data"];assert all(i["id"]!=os.environ["INTERACTION_NOTIFICATION_ID"] for i in items);assert any(i["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for i in items);assert type(d["meta"]["unreadCount"]) is int and d["meta"]["unreadCount"]>=0' 2>/dev/null &&
    printf '%s' "$note_03c_owner_after_leave" |
      PATH_ID="$note_03c_target_path_id" PARTICIPANT_ID="$participant_id" python3 -c \
        'import json,os,sys;items=[item for item in json.load(sys.stdin)["data"] if item["type"]=="path_member_left" and item.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1 and items[0]["actor"]["userId"]==os.environ["PARTICIPANT_ID"] and items[0]["presentation"]=="informational"' 2>/dev/null; then
    break
  fi
  sleep 0.1
done
note_03c_manager_notification_id="$(printf '%s' "$note_03c_owner_after_leave" |
  PATH_ID="$note_03c_target_path_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["type"]=="path_member_left" and i.get("pathId")==os.environ["PATH_ID"]];assert len(items)==1;print(items[0]["id"])')"
note_03c_unrelated_after="$(printf '%s' "$note_03c_after_leave" |
  NOTIFICATION_ID="$note_03c_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
[[ "$note_03c_unrelated_after" == "$note_03c_unrelated_before" ]] || {
  echo "NOTE-03C leave changed the unrelated notification" >&2
  exit 1
}

note_03c_removed_interaction_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/notifications/$note_03c_interaction_notification_id")"
note_03c_missing_notification_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications/note-03c-nonexistent-notification')"
[[ "$note_03c_removed_interaction_response" == "$note_03c_missing_notification_response" &&
   "$note_03c_removed_interaction_response" == *'|404' ]] || {
  echo "NOTE-03C removed and nonexistent notification responses differed" >&2
  exit 1
}
expect_status 404 -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$note_03c_target_path_id/activities/$note_03c_activity_id"
expect_status 404 -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$note_03c_event_id"

docker compose start push-provider
note_03c_provider_health=''
for _ in $(seq 1 100); do
  note_03c_provider_health="$(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q push-provider)")"
  [[ "$note_03c_provider_health" == healthy ]] && break
  sleep 0.1
done
[[ "$note_03c_provider_health" == healthy ]] || {
  echo "NOTE-03C controlled push provider did not become healthy after restart" >&2
  exit 1
}
PUSH_NOTIFICATION_ID="$note_03c_interaction_notification_id" python3 -c \
  'import json,os,sys;p=os.environ["PUSH_NOTIFICATION_ID"];rows=[json.loads(line) for line in open(sys.argv[1],encoding="utf-8") if line.endswith("\n") and line.strip()];assert all(row.get("data",{}).get("notificationId")!=p for row in rows)' \
  "$push_provider_log"

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$note_03c_target_path_id" -v "participant_id=$participant_id" <<'NOTE_03C_REJOIN_MEMBERSHIP'
INSERT INTO path_membership_models (path_id, user_id, role, joined_at)
SELECT id, :'participant_id', 'participant', CURRENT_TIMESTAMP
FROM path_models
WHERE id = :'path_id';
NOTE_03C_REJOIN_MEMBERSHIP
(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run TestSeedPathHTTPAcceptanceParticipant -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$note_03c_target_path_id" \
    -acceptance-path-participant "$participant_id"
)
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/paths/$note_03c_target_path_id/activities/$note_03c_activity_id" |
  ACTIVITY_ID="$note_03c_activity_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["activity"]["id"]==os.environ["ACTIVITY_ID"] and d["activity"]["note"]=="Retain this practice"'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$note_03c_event_id" |
  EVENT_ID="$note_03c_event_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)["data"]["id"]==os.environ["EVENT_ID"]'

note_03c_after_rejoin="$(curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
printf '%s' "$note_03c_after_rejoin" |
  INTERACTION_NOTIFICATION_ID="$note_03c_interaction_notification_id" UNRELATED_NOTIFICATION_ID="$note_03c_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=json.load(sys.stdin)["data"];assert all(i["id"]!=os.environ["INTERACTION_NOTIFICATION_ID"] for i in items);assert any(i["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for i in items)'
note_03c_rejoined_interaction_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/notifications/$note_03c_interaction_notification_id")"
[[ "$note_03c_rejoined_interaction_response" == "$note_03c_missing_notification_response" ]] || {
  echo "NOTE-03C rejoin resurrected exact notification access" >&2
  exit 1
}

note_03c_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$note_03c_target_path_id" -v "participant_id=$participant_id" \
  -v "activity_id=$note_03c_activity_id" -v "event_id=$note_03c_event_id" \
  -v "comment_id=$note_03c_comment_id" -v "interaction_notification_id=$note_03c_interaction_notification_id" \
  -v "manager_notification_id=$note_03c_manager_notification_id" \
  -v "unrelated_notification_id=$note_03c_unrelated_notification_id" <<'NOTE_03C_PERSISTENCE'
SELECT
  (SELECT count(*) FROM path_membership_models
   WHERE path_id = :'path_id' AND user_id = :'participant_id' AND role = 'participant'),
  (SELECT count(*) FROM recorded_activity_models
   WHERE id = :'activity_id' AND path_id = :'path_id' AND participant_id = :'participant_id'),
  (SELECT count(*) FROM social_feed_event_models
   WHERE id = :'event_id' AND path_id = :'path_id' AND participant_user_id = :'participant_id'),
  (SELECT count(*) FROM social_practice_comment_models
   WHERE id = :'comment_id' AND social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM notification_models
   WHERE id = :'interaction_notification_id' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'interaction_notification_id' AND deleted_at IS NOT NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'manager_notification_id' AND kind = 'path_member_left' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models
   WHERE id = :'unrelated_notification_id' AND kind = 'path_invitation_received' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_push_delivery_models
   WHERE notification_id = :'interaction_notification_id'),
  (SELECT count(*) FROM notification_push_delivery_models
   WHERE notification_id = :'interaction_notification_id'
     AND suppressed_at IS NOT NULL
     AND failure_code = 'path_access_revoked'
     AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL
     AND delivered_at IS NULL AND permanently_failed_at IS NULL);
NOTE_03C_PERSISTENCE
)"
[[ "$note_03c_persistence" == '1|1|1|1|0|1|1|1|1|1' ]] || {
  echo "NOTE-03C persistence mismatch: expected=1|1|1|1|0|1|1|1|1|1 actual=$note_03c_persistence" >&2
  exit 1
}
echo "NOTE-03C retained-activity interaction cleanup, unread convergence, opaque lookup, unrelated and manager-notice retention, push suppression, and non-resurrection acceptance passed"

# NOTE_03D_VISIBILITY_CONTRACTION_CLEANUP proves that narrowing a Path retires
# target-opening interaction state for a nonmember who loses effective access.
# The provider remains stopped from interaction creation through contraction so
# the exact pending delivery must be suppressed before it can be handed off.
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "owner_id=$owner_id" <<'NOTE_03D_PUBLIC_OWNER'
UPDATE user_models SET profile_visibility = 'public', updated_at = CURRENT_TIMESTAMP
WHERE id = :'owner_id';
NOTE_03D_PUBLIC_OWNER

note_03d_interaction_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03d-interaction-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Visibility contraction cleanup","visibility":"public"}' \
  http://localhost:8080/v1/paths)"
note_03d_interaction_path_id="$(printf '%s' "$note_03d_interaction_path_response" |
  python3 -c 'import json,sys;d=json.load(sys.stdin)["data"];assert d["name"]=="Visibility contraction cleanup" and d["visibility"]=="public";print(d["id"])')"
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$note_03d_interaction_path_id" -v "administrator_id=$administrator_id" \
  -v "participant_id=$participant_id" <<'NOTE_03D_ACTIVITY_MEMBERSHIP'
INSERT INTO path_membership_models (path_id, user_id, role, joined_at)
SELECT id, :'administrator_id', 'participant', created_at
FROM path_models
WHERE id = :'path_id';
INSERT INTO follow_models (follower_user_id, following_user_id, created_at)
VALUES (:'participant_id', :'administrator_id', CURRENT_TIMESTAMP)
ON CONFLICT DO NOTHING;
NOTE_03D_ACTIVITY_MEMBERSHIP
(
  cd apps/api
  GOWORK=off go test -count=1 ./internal/adapters/spicedb -run TestSeedPathHTTPAcceptanceParticipant -args \
    -spicedb-endpoint "127.0.0.1:${spicedb_host_port}" \
    -spicedb-token 'local-development-runtime-change-me' \
    -spicedb-insecure \
    -acceptance-path-id "$note_03d_interaction_path_id" \
    -acceptance-path-participant "$administrator_id"
)
note_03d_activity_id='note-03d-visibility-activity'
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "path_id=$note_03d_interaction_path_id" -v "activity_id=$note_03d_activity_id" \
  -v "administrator_id=$administrator_id" <<'NOTE_03D_ACTIVITY_SEED'
INSERT INTO recorded_activity_models (
  id, path_id, participant_id, started_at, ended_at, occurrence_time_zone,
  created_at, updated_at, note
) VALUES (
  :'activity_id', :'path_id', :'administrator_id',
  CURRENT_TIMESTAMP - INTERVAL '1 second', CURRENT_TIMESTAMP,
  'America/New_York', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
  'Visible before narrowing'
);
NOTE_03D_ACTIVITY_SEED
note_03d_event_id="practice:$note_03d_activity_id"
# Creation synchronously publishes the creator relationship; the public-viewer
# grant is delivered by the outbox worker. Wait for that fixture prerequisite.
note_03d_public_status=''
for _ in $(seq 1 40); do
  note_03d_public_status="$(curl --connect-timeout 1 --max-time 2 -sS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $participant_token" \
    "http://localhost:8080/v1/social/feed/$note_03d_event_id")"
  [[ "$note_03d_public_status" == 200 ]] && break
  [[ "$note_03d_public_status" == 404 ]] || {
    echo "NOTE-03D public fixture returned unexpected status $note_03d_public_status" >&2
    exit 1
  }
  sleep 0.1
done
[[ "$note_03d_public_status" == 200 ]] || {
  echo "NOTE-03D public visibility grant did not become effective" >&2
  exit 1
}
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$note_03d_event_id" |
  EVENT_ID="$note_03d_event_id" PATH_ID="$note_03d_interaction_path_id" OWNER_ID="$administrator_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["EVENT_ID"] and d["path"]["id"]==os.environ["PATH_ID"] and d["participant"]["userId"]==os.environ["OWNER_ID"]'

note_03d_unrelated_path_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03d-unrelated-path-create-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"name":"Visibility contraction unrelated control","visibility":"private"}' \
  http://localhost:8080/v1/paths)"
note_03d_unrelated_path_id="$(printf '%s' "$note_03d_unrelated_path_response" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')"
note_03d_unrelated_invitation_body="$(PARTICIPANT_ID="$participant_id" python3 -c \
  'import json,os;print(json.dumps({"username":"live.acceptance.participant","expectedRecipientUserId":os.environ["PARTICIPANT_ID"],"offeredRole":"supporter"},separators=(",",":")))')"
note_03d_unrelated_invitation_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03d-unrelated-invitation-key-01' \
  -H 'Content-Type: application/json' \
  --data "$note_03d_unrelated_invitation_body" \
  "http://localhost:8080/v1/paths/$note_03d_unrelated_path_id/invitations")"
note_03d_unrelated_invitation_id="$(printf '%s' "$note_03d_unrelated_invitation_response" |
  python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')"
note_03d_before_interaction="$(curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
note_03d_unrelated_notification_id="$(printf '%s' "$note_03d_before_interaction" |
  INVITATION_ID="$note_03d_unrelated_invitation_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i.get("invitationId")==os.environ["INVITATION_ID"]];assert len(items)==1 and items[0]["type"]=="path_invitation_received";print(items[0]["id"])')"
note_03d_unrelated_before="$(printf '%s' "$note_03d_before_interaction" |
  NOTIFICATION_ID="$note_03d_unrelated_notification_id" python3 -c \
    'import json,os,sys;items=[i for i in json.load(sys.stdin)["data"] if i["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1;print(json.dumps(items[0],sort_keys=True,separators=(",",":")))')"
wait_for_push "$note_03d_unrelated_notification_id" >/dev/null

docker compose stop push-provider
note_03d_comment_response="$(curl -fsS -X POST \
  -H "Authorization: Bearer $participant_token" \
  -H 'Idempotency-Key: note-03d-comment-create-key-00001' \
  -H 'Content-Type: application/json' \
  --data '{"text":"Visible interaction"}' \
  "http://localhost:8080/v1/social/feed/$note_03d_event_id/comments")"
note_03d_comment_id="$(printf '%s' "$note_03d_comment_response" |
  EVENT_ID="$note_03d_event_id" PARTICIPANT_ID="$participant_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["eventId"]==os.environ["EVENT_ID"] and d["authorUserId"]==os.environ["PARTICIPANT_ID"] and d["text"]=="Visible interaction";print(d["id"])')"
curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03d-comment-heart-key-000001' \
  "http://localhost:8080/v1/social/feed/$note_03d_event_id/comments/$note_03d_comment_id/heart" |
  COMMENT_ID="$note_03d_comment_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d=={"commentId":os.environ["COMMENT_ID"],"heartCount":1,"heartedByViewer":True}'
note_03d_interaction_notification_id="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "event_id=$note_03d_event_id" -v "comment_id=$note_03d_comment_id" \
  -v "owner_id=$owner_id" -v "participant_id=$participant_id" <<'NOTE_03D_INTERACTION_NOTIFICATION'
SELECT id FROM notification_models
WHERE kind = 'comment_heart'
  AND social_feed_event_id = :'event_id'
  AND comment_id = :'comment_id'
  AND actor_user_id = :'owner_id'
  AND recipient_user_id = :'participant_id'
  AND deleted_at IS NULL;
NOTE_03D_INTERACTION_NOTIFICATION
)"
[[ -n "$note_03d_interaction_notification_id" ]] || {
  echo "NOTE-03D comment heart did not create an interaction notification" >&2
  exit 1
}
note_03d_interaction_queued=''
for _ in $(seq 1 150); do
  note_03d_interaction_queued="$(docker compose exec -T postgres psql -At -U app -d app \
    -v "notification_id=$note_03d_interaction_notification_id" <<'NOTE_03D_INTERACTION_QUEUED'
SELECT count(*) FROM notification_push_delivery_models
WHERE notification_id = :'notification_id'
  AND delivered_at IS NULL AND suppressed_at IS NULL
  AND permanently_failed_at IS NULL;
NOTE_03D_INTERACTION_QUEUED
)"
  [[ "$note_03d_interaction_queued" == 1 ]] && break
  sleep 0.1
done
[[ "$note_03d_interaction_queued" == 1 ]] || {
  echo "NOTE-03D interaction push was not durably queued while the provider was unavailable" >&2
  exit 1
}
# NOTE_03D_INTERACTION_HISTORY_READY polls the authoritative history by the
# exact notification ID beyond the interaction notification's five-second
# eligibility boundary before asserting its complete presentation.
note_03d_after_interaction=''
for _ in $(seq 1 70); do
  note_03d_after_interaction="$(curl -fsS -H "Authorization: Bearer $participant_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  if printf '%s' "$note_03d_after_interaction" |
    NOTIFICATION_ID="$note_03d_interaction_notification_id" python3 -c \
      'import json,os,sys;d=json.load(sys.stdin);items=[item for item in d["data"] if item["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1 and items[0]["read"] is False and type(d["meta"]["unreadCount"]) is int' 2>/dev/null; then
    break
  fi
  sleep 0.1
done
printf '%s' "$note_03d_after_interaction" |
  NOTIFICATION_ID="$note_03d_interaction_notification_id" EVENT_ID="$note_03d_event_id" \
  COMMENT_ID="$note_03d_comment_id" OWNER_ID="$owner_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=[i for i in d["data"] if i["id"]==os.environ["NOTIFICATION_ID"]];assert len(items)==1 and type(d["meta"]["unreadCount"]) is int;item=items[0];assert item["type"]=="comment_heart" and item["socialFeedEventId"]==os.environ["EVENT_ID"] and item["commentId"]==os.environ["COMMENT_ID"] and item["actor"]["userId"]==os.environ["OWNER_ID"] and item["read"] is False'

note_03d_narrow_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03d-visibility-private-key-01' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"public","visibility":"private"}' \
  "http://localhost:8080/v1/paths/$note_03d_interaction_path_id/visibility")"
printf '%s' "$note_03d_narrow_response" |
  PATH_ID="$note_03d_interaction_path_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["PATH_ID"] and d["visibility"]=="private"'
expect_status 404 -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$note_03d_event_id"

note_03d_after_contraction=''
for _ in $(seq 1 40); do
  note_03d_after_contraction="$(curl -fsS -H "Authorization: Bearer $participant_token" \
    'http://localhost:8080/v1/notifications?limit=100')"
  if printf '%s' "$note_03d_after_contraction" |
    INTERACTION_NOTIFICATION_ID="$note_03d_interaction_notification_id" \
    UNRELATED_NOTIFICATION_ID="$note_03d_unrelated_notification_id" \
    python3 -c \
      'import json,os,sys;d=json.load(sys.stdin);items=d["data"];assert all(item["id"]!=os.environ["INTERACTION_NOTIFICATION_ID"] for item in items);assert any(item["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for item in items);assert type(d["meta"]["unreadCount"]) is int' 2>/dev/null; then
    break
  fi
  sleep 0.1
done
printf '%s' "$note_03d_after_contraction" |
  INTERACTION_NOTIFICATION_ID="$note_03d_interaction_notification_id" \
  UNRELATED_NOTIFICATION_ID="$note_03d_unrelated_notification_id" \
  python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=d["data"];assert all(item["id"]!=os.environ["INTERACTION_NOTIFICATION_ID"] for item in items);assert any(item["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for item in items);assert type(d["meta"]["unreadCount"]) is int'
note_03d_unrelated_after="$(printf '%s' "$note_03d_after_contraction" |
  NOTIFICATION_ID="$note_03d_unrelated_notification_id" python3 -c \
    'import json,os,sys;item=next(i for i in json.load(sys.stdin)["data"] if i["id"]==os.environ["NOTIFICATION_ID"]);print(json.dumps(item,sort_keys=True,separators=(",",":")))')"
[[ "$note_03d_unrelated_after" == "$note_03d_unrelated_before" ]] || {
  echo "NOTE-03D contraction changed the unrelated notification" >&2
  exit 1
}

note_03d_removed_interaction_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/notifications/$note_03d_interaction_notification_id")"
note_03d_missing_notification_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications/note-03d-nonexistent-notification')"
[[ "$note_03d_removed_interaction_response" == "$note_03d_missing_notification_response" &&
   "$note_03d_removed_interaction_response" == *'|404' ]] || {
  echo "NOTE-03D removed and nonexistent notification responses differed" >&2
  exit 1
}

docker compose start push-provider
note_03d_provider_health=''
for _ in $(seq 1 100); do
  note_03d_provider_health="$(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q push-provider)")"
  [[ "$note_03d_provider_health" == healthy ]] && break
  sleep 0.1
done
[[ "$note_03d_provider_health" == healthy ]] || {
  echo "NOTE-03D controlled push provider did not become healthy after restart" >&2
  exit 1
}
PUSH_NOTIFICATION_ID="$note_03d_interaction_notification_id" python3 -c \
  'import json,os,sys;p=os.environ["PUSH_NOTIFICATION_ID"];rows=[json.loads(line) for line in open(sys.argv[1],encoding="utf-8") if line.endswith("\n") and line.strip()];assert all(row.get("data",{}).get("notificationId")!=p for row in rows)' \
  "$push_provider_log"

note_03d_expand_response="$(curl -fsS -X PUT \
  -H "Authorization: Bearer $owner_token" \
  -H 'Idempotency-Key: note-03d-visibility-public-key-001' \
  -H 'Content-Type: application/json' \
  --data '{"confirmed":true,"expectedVisibility":"private","visibility":"public"}' \
  "http://localhost:8080/v1/paths/$note_03d_interaction_path_id/visibility")"
printf '%s' "$note_03d_expand_response" |
  PATH_ID="$note_03d_interaction_path_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin)["data"];assert d["id"]==os.environ["PATH_ID"] and d["visibility"]=="public"'
curl -fsS -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/social/feed/$note_03d_event_id" |
  EVENT_ID="$note_03d_event_id" python3 -c \
    'import json,os,sys;assert json.load(sys.stdin)["data"]["id"]==os.environ["EVENT_ID"]'
note_03d_after_expansion="$(curl -fsS -H "Authorization: Bearer $participant_token" \
  'http://localhost:8080/v1/notifications?limit=100')"
printf '%s' "$note_03d_after_expansion" |
  INTERACTION_NOTIFICATION_ID="$note_03d_interaction_notification_id" \
  UNRELATED_NOTIFICATION_ID="$note_03d_unrelated_notification_id" python3 -c \
    'import json,os,sys;d=json.load(sys.stdin);items=d["data"];assert all(item["id"]!=os.environ["INTERACTION_NOTIFICATION_ID"] for item in items);assert any(item["id"]==os.environ["UNRELATED_NOTIFICATION_ID"] for item in items);assert type(d["meta"]["unreadCount"]) is int'
note_03d_reexpanded_interaction_response="$(curl -sS -w '|%{http_code}' \
  -H "Authorization: Bearer $participant_token" \
  "http://localhost:8080/v1/notifications/$note_03d_interaction_notification_id")"
[[ "$note_03d_reexpanded_interaction_response" == "$note_03d_missing_notification_response" ]] || {
  echo "NOTE-03D re-expansion resurrected exact notification access" >&2
  exit 1
}

note_03d_persistence="$(docker compose exec -T postgres psql -At -F '|' -U app -d app \
  -v "path_id=$note_03d_interaction_path_id" -v "activity_id=$note_03d_activity_id" \
  -v "event_id=$note_03d_event_id" -v "comment_id=$note_03d_comment_id" \
  -v "interaction_notification_id=$note_03d_interaction_notification_id" \
  -v "unrelated_notification_id=$note_03d_unrelated_notification_id" <<'NOTE_03D_PERSISTENCE'
SELECT
  (SELECT count(*) FROM path_models WHERE id = :'path_id' AND visibility = 'public'),
  (SELECT count(*) FROM recorded_activity_models WHERE id = :'activity_id' AND path_id = :'path_id'),
  (SELECT count(*) FROM social_feed_event_models WHERE id = :'event_id' AND path_id = :'path_id'),
  (SELECT count(*) FROM social_practice_comment_models WHERE id = :'comment_id' AND social_feed_event_id = :'event_id'),
  (SELECT count(*) FROM social_practice_comment_heart_models WHERE comment_id = :'comment_id'),
  (SELECT count(*) FROM notification_models WHERE id = :'interaction_notification_id' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_models WHERE id = :'interaction_notification_id' AND deleted_at IS NOT NULL),
  (SELECT count(*) FROM notification_models WHERE id = :'unrelated_notification_id' AND deleted_at IS NULL),
  (SELECT count(*) FROM notification_push_delivery_models WHERE notification_id = :'interaction_notification_id'),
  (SELECT count(*) FROM notification_push_delivery_models
   WHERE notification_id = :'interaction_notification_id'
     AND suppressed_at IS NOT NULL AND failure_code = 'path_access_revoked'
     AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL
     AND delivered_at IS NULL AND permanently_failed_at IS NULL);
NOTE_03D_PERSISTENCE
)"
[[ "$note_03d_persistence" == '1|1|1|1|1|0|1|1|1|1' ]] || {
  echo "NOTE-03D persistence mismatch: expected=1|1|1|1|1|0|1|1|1|1 actual=$note_03d_persistence" >&2
  exit 1
}
echo "NOTE-03D visibility-contraction interaction cleanup, unread convergence, opaque lookup, unrelated retention, push suppression, and non-resurrection acceptance passed"

dead_letter_id='shared-relationship-dead-letter'
dead_letter_resource_id='authorization-recovery-live-resource'
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app \
  -v "dead_letter_id=$dead_letter_id" \
  -v "dead_letter_resource_id=$dead_letter_resource_id" \
  -v "owner_id=$owner_id" \
  -v "stranger_id=$stranger_id" \
  <<'AUTHORIZATION_RECOVERY_SEED'
INSERT INTO authorization_outbox_models (
  id,
  resource_type,
  resource_id,
  relation,
  subject_type,
  subject_id,
  owner_user_id,
  actor_user_id,
  operation,
  attempts,
  dead_lettered_at,
  failure_code,
  created_at
)
VALUES (
  :'dead_letter_id',
  'path',
  :'dead_letter_resource_id',
  'supporter',
  'user',
  :'stranger_id',
  :'owner_id',
  :'owner_id',
  'touch',
  5,
  CURRENT_TIMESTAMP,
  'dependency_failure',
  CURRENT_TIMESTAMP
);
AUTHORIZATION_RECOVERY_SEED

expect_status 401 http://localhost:8080/v1/authorization-dead-letters
expect_status 401 -H 'Authorization: Bearer malformed' http://localhost:8080/v1/authorization-dead-letters
curl -fsS -H "Authorization: Bearer $owner_token" http://localhost:8080/v1/authorization-dead-letters |
  DEAD_LETTER_ID="$dead_letter_id" STRANGER_ID="$stranger_id" python3 -c 'import json,os,sys; items=json.load(sys.stdin)["data"]; expected=[item for item in items if item["id"]==os.environ["DEAD_LETTER_ID"]]; assert len(expected)==1 and expected[0]["subjectId"]==os.environ["STRANGER_ID"] and expected[0]["attempts"]==5'
curl -fsS -H "Authorization: Bearer $stranger_token" http://localhost:8080/v1/authorization-dead-letters |
  DEAD_LETTER_ID="$dead_letter_id" python3 -c 'import json,os,sys; assert all(item["id"]!=os.environ["DEAD_LETTER_ID"] for item in json.load(sys.stdin)["data"])'
expect_status 404 -X POST -H "Authorization: Bearer $stranger_token" "http://localhost:8080/v1/authorization-dead-letters/$dead_letter_id/requeue"
expect_status 204 -X POST -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/authorization-dead-letters/$dead_letter_id/requeue"
curl -fsS -H "Authorization: Bearer $owner_token" http://localhost:8080/v1/authorization-dead-letters |
  DEAD_LETTER_ID="$dead_letter_id" python3 -c 'import json,os,sys; assert all(item["id"]!=os.environ["DEAD_LETTER_ID"] for item in json.load(sys.stdin)["data"])'

expect_status 401 "http://localhost:8080/v1/paths/$path_id"
expect_status 401 -H 'Authorization: Bearer malformed' "http://localhost:8080/v1/paths/$path_id"
expect_status 401 -H "Authorization: Bearer $expired_path_token" "http://localhost:8080/v1/paths/$path_id"
oidc_token="$(token hourpaths-web developer@example.com)"
expect_status 401 -H "Authorization: Bearer $oidc_token" "http://localhost:8080/v1/paths/$path_id"
expect_status 401 http://localhost:8080/v1/paths
expect_status 401 -H 'Authorization: Bearer malformed' http://localhost:8080/v1/paths
expect_status 401 -H "Authorization: Bearer $expired_path_token" http://localhost:8080/v1/paths
expect_status 401 -H "Authorization: Bearer $oidc_token" http://localhost:8080/v1/paths
denied_path_audit_before="$(denied_path_view_audit_count)"
[[ "$denied_path_audit_before" =~ ^[0-9]+$ ]] || {
  echo "denied Path-view audit baseline was not an integer: $denied_path_audit_before" >&2
  exit 1
}
denied_response="$(curl -sS -w '|%{http_code}' -H "Authorization: Bearer $stranger_token" "http://localhost:8080/v1/paths/$path_id")"
missing_response="$(curl -sS -w '|%{http_code}' -H "Authorization: Bearer $stranger_token" 'http://localhost:8080/v1/paths/nonexistent-path')"
[[ "$denied_response" == "$missing_response" && "$denied_response" == *'|404' ]] || {
  echo "denied and nonexistent private Path responses differed" >&2
  exit 1
}
denied_path_audit_after="$(denied_path_view_audit_count)"
[[ "$denied_path_audit_after" =~ ^[0-9]+$ ]] &&
  ((denied_path_audit_after - denied_path_audit_before == 1)) || {
  echo "one denied existing-Path read did not append exactly one denial audit: before=$denied_path_audit_before after=$denied_path_audit_after" >&2
  exit 1
}

wrong_audience_token="$(token hourpaths-wrong-audience developer@example.com)"
wrong_audience_body="$(IDENTITY_TOKEN="$wrong_audience_token" python3 -c 'import json,os;print(json.dumps({"identityToken":os.environ["IDENTITY_TOKEN"]}))')"
expect_status 401 -X POST -H 'Content-Type: application/json' --data-binary "$wrong_audience_body" http://localhost:8080/v1/sessions

curl -fsS -H "Authorization: Bearer $owner_token" http://localhost:8080/v1/audit-events |
  python3 -c 'import json,sys;assert any(e["action"]=="user.viewed" for e in json.load(sys.stdin)["data"])'

curl -fsS http://localhost:8080/openapi.json | python3 -c 'import json,sys;s=json.load(sys.stdin)["components"]["securitySchemes"]["oidc"];assert s["flows"]["authorizationCode"]["x-usePkce"]=="SHA-256"'
curl -fsS http://localhost:8080/docs/assets/scalar-api-reference-LICENSE.txt | grep -Fq 'Copyright (c) 2023-present Scalar'
discovery="$(curl -fsS http://localhost:8080/oidc/.well-known/openid-configuration)"
printf '%s' "$discovery" | API_BASE_URL="$api_base_url" python3 -c 'import json,os,sys;d=json.load(sys.stdin);assert d["token_endpoint"]==os.environ["API_BASE_URL"]+"/oidc/token"'

if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d app -c 'CREATE TABLE forbidden_runtime_schema_change(id text)'; then
  echo "runtime role changed schema" >&2; exit 1
fi
if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d app -c 'UPDATE schema_migrations SET dirty=true'; then
  echo "runtime role mutated migration ledger" >&2; exit 1
fi
if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app_migrator -d app -c 'TRUNCATE audit_event_models'; then
  echo "database owner bypassed append-only audit guard" >&2; exit 1
fi
user_viewed_audit_count="$(docker compose exec -T postgres psql -At -U app -d app -v "owner_id=$owner_id" <<'AUDIT_SQL'
SELECT count(*) FROM audit_event_models WHERE owner_user_id=:'owner_id' AND action='user.viewed';
AUDIT_SQL
)"
[[ "$user_viewed_audit_count" =~ ^[1-9][0-9]*$ ]] || {
  echo "owner user.viewed audit count was not positive: $user_viewed_audit_count" >&2
  exit 1
}
path_list_audit_count="$(docker compose exec -T postgres psql -At -U app -d app -v "owner_id=$owner_id" <<'PATH_LIST_AUDIT_SQL'
SELECT count(*)
FROM audit_event_models
WHERE owner_user_id = :'owner_id'
  AND actor_user_id = :'owner_id'
  AND action = 'resource.listed'
  AND target_type = 'path'
  AND target_id = 'path'
  AND outcome = 'succeeded';
PATH_LIST_AUDIT_SQL
)"
[[ "$path_list_audit_count" =~ ^[0-9]+$ ]] && ((path_list_audit_count >= 2)) || {
  echo "owner Path-list audit count was below two: $path_list_audit_count" >&2
  exit 1
}
authorization_recovery_counts="$(docker compose exec -T postgres psql -At -U app -d app \
  -v "dead_letter_id=$dead_letter_id" -v "dead_letter_resource_id=$dead_letter_resource_id" -v "owner_id=$owner_id" -v "stranger_id=$stranger_id" <<'AUTHORIZATION_RECOVERY_SQL'
SELECT
  (SELECT count(*)
   FROM authorization_outbox_models
   WHERE id = :'dead_letter_id'
     AND completed_at IS NOT NULL
     AND dead_lettered_at IS NULL
     AND attempts = 0
     AND failure_code = ''
  ),
  (SELECT count(*)
   FROM audit_event_models
   WHERE action = 'authorization.dead_letter_requeued'
     AND owner_user_id = :'owner_id'
     AND actor_user_id = :'owner_id'
     AND target_id = :'dead_letter_id'
  ),
  (SELECT count(*)
   FROM audit_event_models
   WHERE action = 'authorization.relationship_applied'
     AND owner_user_id = :'owner_id'
     AND actor_user_id = :'owner_id'
     AND target_type = 'path'
     AND target_id = :'dead_letter_resource_id'
  ),
  (SELECT count(*)
   FROM audit_event_models
   WHERE action = 'resource.access_denied'
     AND owner_user_id = :'stranger_id'
     AND actor_user_id = :'stranger_id'
     AND target_type = 'authorization_change'
     AND target_id = :'dead_letter_id'
  );
AUTHORIZATION_RECOVERY_SQL
)"
[[ "$authorization_recovery_counts" == '1|1|1|1' ]] || {
  echo "authorization recovery persistence mismatch: expected=1|1|1|1 actual=$authorization_recovery_counts" >&2
  exit 1
}

docker compose stop spicedb
expect_status 503 http://localhost:8080/readyz
expect_status 204 http://localhost:8080/livez
expect_status 401 -H "Authorization: Bearer $expired_path_token" "http://localhost:8080/v1/paths/$path_id"
expect_status 500 -H "Authorization: Bearer $owner_token" "http://localhost:8080/v1/paths/$path_id"
docker compose start spicedb
for _ in $(seq 1 60); do curl -fsS http://localhost:8080/readyz >/dev/null 2>&1 && break; sleep 1; done
echo "blank-app authentication, OIDC, authorization recovery, audit, role, and health acceptance passed"
