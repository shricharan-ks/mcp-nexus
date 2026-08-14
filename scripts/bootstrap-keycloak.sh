#!/usr/bin/env bash
set -euo pipefail

# bootstrap-keycloak.sh
#
# Bootstraps Keycloak for the MCP Gateway project:
#   1. Waits for Keycloak to become healthy
#   2. Creates the "mcp-gateway" realm
#   3. Creates confidential clients with service-account grants
#   4. Creates realm roles
#
# The script is idempotent -- it checks for existing resources before creating.

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8080}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin}"
REALM="${REALM:-mcp-gateway}"

MAX_WAIT="${MAX_WAIT:-120}"

# --------------------------------------------------------------------------
# Helpers
# --------------------------------------------------------------------------

log() {
  echo "==> $*"
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

# kc_api performs a request against the Keycloak Admin REST API and returns
# the HTTP status code. Response body is written to stdout.
kc_api() {
  local method="$1"
  local path="$2"
  local token="$3"
  local data="${4:-}"

  local args=(
    -s
    -o /dev/stdout
    -w "\n%{http_code}"
    -X "${method}"
    -H "Authorization: Bearer ${token}"
    -H "Content-Type: application/json"
  )

  if [[ -n "${data}" ]]; then
    args+=(-d "${data}")
  fi

  curl "${args[@]}" "${KEYCLOAK_URL}${path}"
}

# --------------------------------------------------------------------------
# 1. Wait for Keycloak to be ready
# --------------------------------------------------------------------------

log "Waiting for Keycloak at ${KEYCLOAK_URL} (timeout ${MAX_WAIT}s)..."
elapsed=0
until curl -sf "${KEYCLOAK_URL}/health/ready" > /dev/null 2>&1; do
  if (( elapsed >= MAX_WAIT )); then
    fail "Keycloak did not become ready within ${MAX_WAIT}s"
  fi
  sleep 2
  elapsed=$(( elapsed + 2 ))
done
log "Keycloak is ready."

# --------------------------------------------------------------------------
# 2. Obtain admin token from the master realm
# --------------------------------------------------------------------------

log "Obtaining admin token..."
TOKEN_RESPONSE=$(curl -sf \
  -X POST \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" \
  -d "username=${ADMIN_USER}" \
  -d "password=${ADMIN_PASS}" \
  "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token") \
  || fail "Failed to obtain admin token"

ADMIN_TOKEN=$(echo "${TOKEN_RESPONSE}" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
[[ -n "${ADMIN_TOKEN}" ]] || fail "Admin token is empty"
log "Admin token obtained."

# --------------------------------------------------------------------------
# 3. Create realm
# --------------------------------------------------------------------------

log "Checking if realm '${REALM}' exists..."
REALM_RESPONSE=$(kc_api GET "/admin/realms/${REALM}" "${ADMIN_TOKEN}")
REALM_STATUS=$(echo "${REALM_RESPONSE}" | tail -1)

if [[ "${REALM_STATUS}" == "200" ]]; then
  log "Realm '${REALM}' already exists, skipping."
else
  log "Creating realm '${REALM}'..."
  CREATE_RESPONSE=$(kc_api POST "/admin/realms" "${ADMIN_TOKEN}" "$(cat <<EOF
{
  "realm": "${REALM}",
  "enabled": true,
  "displayName": "MCP Gateway"
}
EOF
  )")
  CREATE_STATUS=$(echo "${CREATE_RESPONSE}" | tail -1)
  if [[ "${CREATE_STATUS}" != "201" ]]; then
    fail "Failed to create realm '${REALM}' (HTTP ${CREATE_STATUS})"
  fi
  log "Realm '${REALM}' created."
fi

# --------------------------------------------------------------------------
# 4. Create clients
# --------------------------------------------------------------------------

create_client() {
  local client_id="$1"
  local agent_id="$2"

  log "Checking if client '${client_id}' exists..."
  EXISTING=$(kc_api GET "/admin/realms/${REALM}/clients?clientId=${client_id}" "${ADMIN_TOKEN}")
  EXISTING_STATUS=$(echo "${EXISTING}" | tail -1)
  EXISTING_BODY=$(echo "${EXISTING}" | sed '$d')

  # Check if client already exists (non-empty JSON array)
  if [[ "${EXISTING_STATUS}" == "200" ]] && echo "${EXISTING_BODY}" | python3 -c "import sys,json; data=json.load(sys.stdin); sys.exit(0 if len(data)>0 else 1)" 2>/dev/null; then
    log "Client '${client_id}' already exists, skipping."
    return 0
  fi

  log "Creating client '${client_id}'..."
  CLIENT_PAYLOAD=$(cat <<EOF
{
  "clientId": "${client_id}",
  "enabled": true,
  "publicClient": false,
  "serviceAccountsEnabled": true,
  "standardFlowEnabled": false,
  "clientAuthenticatorType": "client-secret",
  "directAccessGrantsEnabled": false,
  "protocol": "openid-connect",
  "protocolMappers": [
    {
      "name": "agent_id",
      "protocol": "openid-connect",
      "protocolMapper": "oidc-hardcoded-claim-mapper",
      "config": {
        "claim.name": "agent_id",
        "claim.value": "${agent_id}",
        "jsonType.label": "String",
        "id.token.claim": "true",
        "access.token.claim": "true",
        "userinfo.token.claim": "true"
      }
    },
    {
      "name": "audience",
      "protocol": "openid-connect",
      "protocolMapper": "oidc-audience-mapper",
      "config": {
        "included.client.audience": "${client_id}",
        "id.token.claim": "false",
        "access.token.claim": "true"
      }
    }
  ]
}
EOF
  )

  CREATE_RESPONSE=$(kc_api POST "/admin/realms/${REALM}/clients" "${ADMIN_TOKEN}" "${CLIENT_PAYLOAD}")
  CREATE_STATUS=$(echo "${CREATE_RESPONSE}" | tail -1)

  if [[ "${CREATE_STATUS}" != "201" ]]; then
    fail "Failed to create client '${client_id}' (HTTP ${CREATE_STATUS})"
  fi
  log "Client '${client_id}' created."
}

create_client "mcp-gateway-proxy" "mcp-gateway-proxy"
create_client "test-agent" "test-agent"

# --------------------------------------------------------------------------
# 5. Create realm roles
# --------------------------------------------------------------------------

create_role() {
  local role_name="$1"

  log "Checking if role '${role_name}' exists..."
  ROLE_RESPONSE=$(kc_api GET "/admin/realms/${REALM}/roles/${role_name}" "${ADMIN_TOKEN}")
  ROLE_STATUS=$(echo "${ROLE_RESPONSE}" | tail -1)

  if [[ "${ROLE_STATUS}" == "200" ]]; then
    log "Role '${role_name}' already exists, skipping."
    return 0
  fi

  log "Creating role '${role_name}'..."
  ROLE_PAYLOAD=$(cat <<EOF
{
  "name": "${role_name}",
  "description": "Role for ${role_name}"
}
EOF
  )

  CREATE_RESPONSE=$(kc_api POST "/admin/realms/${REALM}/roles" "${ADMIN_TOKEN}" "${ROLE_PAYLOAD}")
  CREATE_STATUS=$(echo "${CREATE_RESPONSE}" | tail -1)

  if [[ "${CREATE_STATUS}" != "201" ]]; then
    fail "Failed to create role '${role_name}' (HTTP ${CREATE_STATUS})"
  fi
  log "Role '${role_name}' created."
}

create_role "mcp-agent"
create_role "mcp-admin"

# --------------------------------------------------------------------------
# 6. Print verification info
# --------------------------------------------------------------------------

echo ""
echo "========================================"
echo " Keycloak Bootstrap Complete"
echo "========================================"
echo ""
echo " Realm:     ${REALM}"
echo " JWKS URL:  ${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/certs"
echo " Token URL: ${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/token"
echo " Clients:   mcp-gateway-proxy, test-agent"
echo " Roles:     mcp-agent, mcp-admin"
echo ""
