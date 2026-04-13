#!/usr/bin/env bash
#
# E2E integration tests for kubectl-cwide.
# Requires a reachable Kubernetes cluster and kubectl on PATH.
#
# Usage:
#   ./hack/e2e-test.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="cwide-e2e-test"
TMPDIR_E2E="$(mktemp -d)"
BINARY="${TMPDIR_E2E}/kubectl-cwide"
TPL_DIR="${TMPDIR_E2E}/templates"
SYNC_DIR="${TMPDIR_E2E}/sync-out"
RULES_FILE="${TMPDIR_E2E}/deploy-stack.yaml"
CM_NAME="cwide-e2e-templates"

PASS_COUNT=0
FAIL_COUNT=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  echo -e "  ${GREEN}PASS${NC} $1"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  echo -e "  ${RED}FAIL${NC} $1"
}

assert_contains() {
  local output="$1" expected="$2" name="$3"
  if echo "${output}" | grep -qF "${expected}"; then
    pass "${name}"
  else
    fail "${name} (expected '${expected}' in output)"
    echo "    output: ${output}" | head -5
  fi
}

assert_not_contains() {
  local output="$1" unexpected="$2" name="$3"
  if echo "${output}" | grep -qF "${unexpected}"; then
    fail "${name} (unexpected '${unexpected}' in output)"
  else
    pass "${name}"
  fi
}

header() {
  echo ""
  echo -e "${BOLD}=== $1 ===${NC}"
}

# ---------------------------------------------------------------------------
# Cleanup (runs on EXIT)
# ---------------------------------------------------------------------------

cleanup() {
  header "Cleanup"
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=false 2>/dev/null || true
  rm -rf "${TMPDIR_E2E}"
  echo "  Temp dir and namespace removed."
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------

header "Build"
cd "${REPO_ROOT}"
go build -o "${BINARY}" ./cmd/
echo "  Binary: ${BINARY}"

header "Cluster setup"

# Verify cluster connectivity
if ! kubectl cluster-info >/dev/null 2>&1; then
  echo -e "${RED}ERROR: Cannot reach Kubernetes cluster. Ensure kubectl is configured.${NC}"
  exit 1
fi
echo "  Cluster reachable."

kubectl create namespace "${NAMESPACE}" 2>/dev/null || true

kubectl apply -n "${NAMESPACE}" -f - <<'MANIFESTS'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-e2e
  labels:
    app: nginx-e2e
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx-e2e
  template:
    metadata:
      labels:
        app: nginx-e2e
    spec:
      containers:
      - name: nginx
        image: nginx:1.25
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-e2e-svc
  labels:
    app: nginx-e2e
spec:
  selector:
    app: nginx-e2e
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-e2e-config
  labels:
    app: nginx-e2e
data:
  index.html: "<h1>e2e test</h1>"
MANIFESTS

echo "  Waiting for deployment rollout..."
kubectl -n "${NAMESPACE}" rollout status deployment/nginx-e2e --timeout=120s

echo "  Test resources ready."

# Prepare a minimal template directory for get/push tests
mkdir -p "${TPL_DIR}/pod--v1"
cat > "${TPL_DIR}/pod--v1/default.yaml" <<'TMPL'
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    template: '{{ .status.phase }}'
TMPL

# =========================================================================
# Test 1: Typo suggestions & aliases
# =========================================================================

header "Test 1: Typo suggestions & aliases"

out=$("${BINARY}" geet 2>&1 || true)
assert_contains "${out}" "get" "geet suggests get"

out=$("${BINARY}" fetch 2>&1 || true)
assert_contains "${out}" "get" "fetch suggests get (via SuggestFor)"

for alias_cmd in g cm mp tpl cfg t ls al; do
  out=$("${BINARY}" "${alias_cmd}" --help 2>&1 || true)
  assert_not_contains "${out}" "unknown command" "alias '${alias_cmd}' resolves"
done

# =========================================================================
# Test 2: Custom functions
# =========================================================================

header "Test 2: Custom functions"

# 2a: Basic custom functions
cat > "${TPL_DIR}/pod--v1/cfunc-test.yaml" <<'TMPL'
funcs:
  shortName: '{{ .metadata.name | trunc 20 }}'
  statusBadge: '{{ if eq .status.phase "Running" }}[OK]{{ else }}[!!]{{ end }} {{ .status.phase }}'
columns:
  - header: SHORT_NAME
    template: '{{ shortName . }}'
  - header: STATUS_BADGE
    template: '{{ statusBadge . }}'
  - header: NAMESPACE
    template: '{{ .metadata.namespace }}'
TMPL

out=$("${BINARY}" get pods -n "${NAMESPACE}" -t cfunc-test --template-path "${TPL_DIR}" 2>&1)
assert_contains "${out}" "SHORT_NAME" "custom funcs: header rendered"
assert_contains "${out}" "[OK] Running" "custom funcs: statusBadge works"
assert_contains "${out}" "${NAMESPACE}" "custom funcs: namespace column"

# 2b: Func calling func
cat > "${TPL_DIR}/pod--v1/cfunc-chain.yaml" <<'TMPL'
funcs:
  readyCount: '{{ $r := 0 }}{{ range .status.containerStatuses }}{{ if .ready }}{{ $r = add1 $r }}{{ end }}{{ end }}{{ $r }}'
  totalCount: '{{ len .spec.containers }}'
  readiness: '{{ readyCount . }}/{{ totalCount . }}'
columns:
  - header: NAME
    template: '{{ .metadata.name }}'
  - header: READINESS
    template: '{{ readiness . }}'
TMPL

out=$("${BINARY}" get pods -n "${NAMESPACE}" -t cfunc-chain --template-path "${TPL_DIR}" 2>&1)
assert_contains "${out}" "1/1" "func-calling-func: readiness shows 1/1"

# =========================================================================
# Test 3: Tree command
# =========================================================================

header "Test 3: Tree command"

# 3a: ownerRef inline (deployment -> replicasets -> pods)
out=$("${BINARY}" tree deployment/nginx-e2e -n "${NAMESPACE}" \
  --related=replicasets:ownerRef \
  --related=pods:ownerRef:replicasets 2>&1)
assert_contains "${out}" "Deployment/nginx-e2e" "tree ownerRef: root deployment"
assert_contains "${out}" "ReplicaSet/" "tree ownerRef: replicaset child"
assert_contains "${out}" "Pod/" "tree ownerRef: pod grandchild"

# 3b: labelSelector (service -> pods)
out=$("${BINARY}" tree service/nginx-e2e-svc -n "${NAMESPACE}" \
  --related=pods:labelSelector 2>&1)
assert_contains "${out}" "Service/nginx-e2e-svc" "tree labelSelector: root service"
assert_contains "${out}" "Pod/" "tree labelSelector: pod child"

# 3c: YAML rules file
cat > "${RULES_FILE}" <<'RULES'
relations:
  - resource: replicasets
    bind:
      type: ownerRef
  - resource: pods
    bind:
      type: ownerRef
      parent: replicasets
  - resource: services
    bind:
      type: labelSelector
RULES

out=$("${BINARY}" tree deployment/nginx-e2e -n "${NAMESPACE}" -f "${RULES_FILE}" 2>&1)
assert_contains "${out}" "Deployment/nginx-e2e" "tree rules file: root"
assert_contains "${out}" "ReplicaSet/" "tree rules file: replicaset"
assert_contains "${out}" "Pod/" "tree rules file: pods"
assert_contains "${out}" "Service/" "tree rules file: service"

# 3d: Mixed mode (rules file + inline --related)
out=$("${BINARY}" tree deployment/nginx-e2e -n "${NAMESPACE}" \
  -f "${RULES_FILE}" --related=configmaps:labelSelector 2>&1)
assert_contains "${out}" "ConfigMap/" "tree mixed mode: configmap via inline"
assert_contains "${out}" "Service/" "tree mixed mode: service via rules"

# =========================================================================
# Test 4: ConfigMap push & sync
# =========================================================================

header "Test 4: ConfigMap push & sync"

# 4a: Push
out=$("${BINARY}" configmap push \
  --name "${CM_NAME}" --cm-namespace "${NAMESPACE}" \
  --template-path "${TPL_DIR}" -r pod 2>&1)
assert_contains "${out}" "template(s)" "push: templates uploaded"

# Verify the ConfigMap exists
kubectl -n "${NAMESPACE}" get configmap "${CM_NAME}" >/dev/null 2>&1
pass "push: ConfigMap exists in cluster"

# 4b: Sync to fresh directory
mkdir -p "${SYNC_DIR}"
out=$("${BINARY}" configmap sync \
  --name "${CM_NAME}" --cm-namespace "${NAMESPACE}" \
  --template-path "${SYNC_DIR}" --force 2>&1)
assert_contains "${out}" "Synced" "sync: templates downloaded"
assert_not_contains "${out}" "Synced 0" "sync: at least one template synced"

# Check files exist
if [ -d "${SYNC_DIR}/pod--v1" ] && [ "$(ls "${SYNC_DIR}/pod--v1"/*.yaml 2>/dev/null | wc -l)" -gt 0 ]; then
  pass "sync: pod--v1 templates exist locally"
else
  fail "sync: expected pod--v1 templates in ${SYNC_DIR}"
fi

# 4c: Re-sync without --force should skip (local priority)
out=$("${BINARY}" configmap sync \
  --name "${CM_NAME}" --cm-namespace "${NAMESPACE}" \
  --template-path "${SYNC_DIR}" 2>&1)
assert_contains "${out}" "skipped" "sync priority: existing files skipped"
assert_contains "${out}" "Synced 0" "sync priority: 0 synced (local wins)"

# =========================================================================
# Test 5: List all API resources
# =========================================================================

header "Test 5: List all API resources"

# 5a: Default (namespaced resources)
out=$("${BINARY}" list all 2>&1)
assert_contains "${out}" "NAME" "list all: header present"
assert_contains "${out}" "SHORTNAMES" "list all: SHORTNAMES header"
assert_contains "${out}" "APIVERSION" "list all: APIVERSION header"
assert_contains "${out}" "NAMESPACED" "list all: NAMESPACED header"
assert_contains "${out}" "KIND" "list all: KIND header"
assert_contains "${out}" "pods" "list all: pods listed (namespaced)"
assert_contains "${out}" "deployments" "list all: deployments listed (namespaced)"

# 5b: Cluster-scoped resources (-A flag)
out=$("${BINARY}" list all -A 2>&1)
assert_contains "${out}" "nodes" "list all -A: nodes listed (cluster-scoped)"
assert_contains "${out}" "namespaces" "list all -A: namespaces listed (cluster-scoped)"

# 5c: No-headers flag
out=$("${BINARY}" list all --no-headers 2>&1)
assert_not_contains "${out}" "SHORTNAMES" "list all --no-headers: no header row"
assert_contains "${out}" "pods" "list all --no-headers: still shows data"

# 5d: Alias 'ls' works
out=$("${BINARY}" ls all 2>&1)
assert_contains "${out}" "pods" "list alias 'ls': works"

# =========================================================================
# Test 6: Resource aliases
# =========================================================================

header "Test 6: Resource aliases"

# 6a: Set an alias
out=$("${BINARY}" alias set pd pods 2>&1)
assert_contains "${out}" "pd" "alias set: output contains alias"
assert_contains "${out}" "pods" "alias set: output contains resource"

# 6b: List aliases
out=$("${BINARY}" alias list 2>&1)
assert_contains "${out}" "ALIAS" "alias list: header present"
assert_contains "${out}" "pd" "alias list: alias shown"
assert_contains "${out}" "pods" "alias list: resource shown"

# 6c: Set another alias
out=$("${BINARY}" alias set vw validatingwebhookconfigurations 2>&1)
assert_contains "${out}" "vw" "alias set vw: output contains alias"

# 6d: List shows both aliases
out=$("${BINARY}" alias list 2>&1)
assert_contains "${out}" "pd" "alias list: pd still present"
assert_contains "${out}" "vw" "alias list: vw present"

# 6e: Overwrite warning
out=$("${BINARY}" alias set pd services 2>&1)
assert_contains "${out}" "Warning" "alias overwrite: warning printed"
assert_contains "${out}" "pd" "alias overwrite: output contains alias"

# 6f: Reset pd to pods for get test
"${BINARY}" alias set pd pods >/dev/null 2>&1

# 6g: Get using alias (pd → pods)
out=$("${BINARY}" get pd -n "${NAMESPACE}" --template-path "${TPL_DIR}" 2>&1)
assert_contains "${out}" "NAME" "get with alias: resolves pd to pods"

# 6h: Delete alias
out=$("${BINARY}" alias delete vw 2>&1)
assert_contains "${out}" "deleted" "alias delete: confirmation"

# 6i: Deleted alias no longer listed
out=$("${BINARY}" alias list 2>&1)
assert_not_contains "${out}" "vw" "alias delete: vw removed"

# 6j: Delete non-existent alias fails
out=$("${BINARY}" alias delete nonexistent 2>&1 || true)
assert_contains "${out}" "not found" "alias delete: error for missing alias"

# Cleanup: remove test alias
"${BINARY}" alias delete pd >/dev/null 2>&1 || true

# =========================================================================
# Summary
# =========================================================================

header "Results"
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo -e "  ${GREEN}${PASS_COUNT} passed${NC}, ${RED}${FAIL_COUNT} failed${NC} out of ${TOTAL} checks."

if [ "${FAIL_COUNT}" -gt 0 ]; then
  echo -e "\n${RED}SOME TESTS FAILED${NC}"
  exit 1
fi

echo -e "\n${GREEN}ALL TESTS PASSED${NC}"
