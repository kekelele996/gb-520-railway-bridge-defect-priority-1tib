#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
set -a
if [ -f .env ]; then . ./.env; else . ./.env.example; fi
set +a

(command -v jq >/dev/null 2>&1) || { echo "jq is required for API validation" >&2; exit 1; }

cleanup() { docker compose down -v --remove-orphans; }
docker compose down -v --remove-orphans
if [ "${KEEP_RUNNING:-0}" = "1" ]; then
	trap cleanup INT TERM
else
	trap cleanup EXIT INT TERM
fi

(cd backend && go test ./... && go vet ./... && go build ./...)
(cd frontend && npm ci --no-audit --no-fund && npm run typecheck && npm run build)
docker compose config --quiet
docker compose up -d --build

i=0
until curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19520}/healthz" >/dev/null; do
	i=$((i+1))
	[ "$i" -lt 60 ] || { docker compose logs; exit 1; }
	sleep 2
done
curl -fsS "http://127.0.0.1:${FRONTEND_PORT:-18520}/" >/dev/null

login_token() {
	curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19520}/api/auth/login" \
		-H 'Content-Type: application/json' \
		-d "{\"username\":\"$1\",\"password\":\"Admin123!\"}" | jq -er '.data.token'
}

viewer_token=$(login_token viewer)
operator_token=$(login_token operator)
reviewer_token=$(login_token reviewer)
admin_token=$(login_token admin)

curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/session" -H "Authorization: Bearer $viewer_token" | jq -e '.data.role == "viewer"' >/dev/null
viewer_write_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/bridges" -H "Authorization: Bearer $viewer_token" -H 'Content-Type: application/json' -d '{}')
[ "$viewer_write_status" = "403" ]
viewer_audit_status=$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/audits" -H "Authorization: Bearer $viewer_token")
[ "$viewer_audit_status" = "403" ]
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits?page=1&pageSize=20" -H "Authorization: Bearer $reviewer_token" | jq -e '.data | type == "array"' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/runtime" -H "Authorization: Bearer $admin_token" | jq -e '.data.appName and .data.databaseDriver and (.data.requestLimit > 0)' >/dev/null

now=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
code="PD-SMOKE-$(date +%s)"
create_payload=$(jq -n --arg code "$code" --arg now "$now" '{code:$code,name:"空卷验收优先级决定",description:"验证不可变版本链",facility:"K42 桥梁作业区",owner:"现场处置组",category:"结构缺陷",riskLevel:"critical",metricValue:88,metricUnit:"score",effectiveAt:$now,evidence:"裂缝照片与量测记录 v1",relatedCode:"DF-001"}')
created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities" -H "Authorization: Bearer $operator_token" -H 'X-Request-ID: smoke-create' -H 'Content-Type: application/json' -d "$create_payload")
priority_id=$(printf '%s' "$created" | jq -er '.data.id')
printf '%s' "$created" | jq -e '.data.status == "draft" and .data.version == 1 and .data.preparedBy == "operator" and (.data.revisions | length == 1)' >/dev/null

update_payload=$(jq -n --arg now "$now" '{expectedVersion:1,name:"空卷验收优先级决定",description:"复核前补充量测证据",facility:"K42 桥梁作业区",owner:"现场处置组",category:"结构缺陷",riskLevel:"critical",metricValue:93,metricUnit:"score",effectiveAt:$now,evidence:"裂缝照片、量测记录与复测记录 v2",relatedCode:"DF-001"}')
updated=$(curl -fsS -X PUT "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$priority_id" -H "Authorization: Bearer $operator_token" -H 'X-Request-ID: smoke-update' -H 'Content-Type: application/json' -d "$update_payload")
printf '%s' "$updated" | jq -e '.data.version == 2 and (.data.revisions | length == 2)' >/dev/null

transition_payload='{"status":"urgent","expectedVersion":2,"reason":"独立复核确认需立即处置"}'
operator_final_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$priority_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$transition_payload")
[ "$operator_final_status" = "403" ]

self_code="PD-SELF-$(date +%s)"
self_payload=$(printf '%s' "$create_payload" | jq --arg code "$self_code" '.code = $code')
self_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "$self_payload")
self_id=$(printf '%s' "$self_created" | jq -er '.data.id')
self_final_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$self_id/transition" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d '{"status":"observe","expectedVersion":1,"reason":"不得自行复核自己的决定"}')
[ "$self_final_status" = "422" ]

curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$priority_id/transition" -H "Authorization: Bearer $reviewer_token" -H 'X-Request-ID: smoke-review' -H 'Content-Type: application/json' -d "$transition_payload" | jq -e '.data.status == "urgent" and .data.version == 3' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$priority_id" -H "Authorization: Bearer $reviewer_token" | jq -e '
	.data.status == "urgent" and
	(.data.revisions | length == 3) and
	([.data.revisions[].evidence] == ["裂缝照片与量测记录 v1","裂缝照片、量测记录与复测记录 v2","裂缝照片、量测记录与复测记录 v2"]) and
	([.data.revisions[].actor] == ["operator","operator","reviewer"]) and
	([.data.revisions[].requestId] == ["smoke-create","smoke-update","smoke-review"])' >/dev/null

locked_status=$(curl -sS -o /dev/null -w '%{http_code}' -X PUT "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$priority_id" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$(printf '%s' "$update_payload" | jq '.expectedVersion = 3')")
[ "$locked_status" = "422" ]
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audit-summary?windowHours=24" -H "Authorization: Bearer $reviewer_token" | jq -e '.data.total >= 3 and .data.transitions >= 1' >/dev/null

# 复测到期控制：种子 PD-004 为十天前定稿的 urgent/critical，按定稿时间补算截止后已逾期，原决定保留
pd004=$(curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/priorities?search=PD-004" -H "Authorization: Bearer $reviewer_token")
pd004_id=$(printf '%s' "$pd004" | jq -er '.data[0].id')
pd004_version=$(printf '%s' "$pd004" | jq -er '.data[0].version')
printf '%s' "$pd004" | jq -e '.data[0].status == "urgent" and .data[0].retestOverdue == true and (.data[0].retestDeadline != null) and (.data[0].retestConclusion == "")' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/defects?search=DF-003" -H "Authorization: Bearer $reviewer_token" | jq -e '.data[0].retestOverdue == true' >/dev/null

# 同一缺陷逾期未复测时不得新建定稿决定
blocked_code="PD-BLOCKED-$(date +%s)"
blocked_id=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$(printf '%s' "$create_payload" | jq --arg code "$blocked_code" '.code = $code | .relatedCode = "DF-003"')" | jq -er '.data.id')
blocked_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$blocked_id/transition" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d '{"status":"restrict","expectedVersion":1,"reason":"逾期缺陷不得新建定稿决定"}')
[ "$blocked_status" = "422" ]

# 复测结论只能由非拟制人的复核员登记：operator 403，拟制人 422
operator_retest_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$pd004_id/retest" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$pd004_version,\"conclusion\":\"操作员不得登记复测结论\"}")
[ "$operator_retest_status" = "403" ]
self_retest_code="PD-RETEST-$(date +%s)"
self_retest_id=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "$(printf '%s' "$create_payload" | jq --arg code "$self_retest_code" '.code = $code | .relatedCode = "DF-002"')" | jq -er '.data.id')
curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$self_retest_id/transition" -H "Authorization: Bearer $admin_token" -H 'Content-Type: application/json' -d '{"status":"urgent","expectedVersion":1,"reason":"管理员独立复核定稿"}' | jq -e '.data.status == "urgent" and (.data.retestDeadline != null)' >/dev/null
self_retest_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$self_retest_id/retest" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d '{"expectedVersion":2,"conclusion":"拟制人不得自行登记复测结论"}')
[ "$self_retest_status" = "422" ]

# 非拟制人复核员登记复测结论：逾期清除、原决定保留、版本链追加；之后同一缺陷可再次定稿；重复登记 422
curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$pd004_id/retest" -H "Authorization: Bearer $reviewer_token" -H 'X-Request-ID: smoke-retest' -H 'Content-Type: application/json' -d "{\"expectedVersion\":$pd004_version,\"conclusion\":\"复测合格，位移已收敛\"}" | jq -e '.data.retestConclusion == "复测合格，位移已收敛" and .data.retestReviewedBy == "reviewer" and .data.retestOverdue == false and .data.status == "urgent"' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/defects?search=DF-003" -H "Authorization: Bearer $reviewer_token" | jq -e '.data[0].retestOverdue == false' >/dev/null
curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$blocked_id/transition" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d '{"status":"restrict","expectedVersion":1,"reason":"复测登记后允许定稿"}' | jq -e '.data.status == "restrict" and (.data.retestDeadline != null)' >/dev/null
duplicate_retest_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/priorities/$pd004_id/retest" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$((pd004_version + 1)),\"conclusion\":\"重复登记应被拒绝\"}")
[ "$duplicate_retest_status" = "422" ]

docker compose ps
if [ "${KEEP_RUNNING:-0}" = "1" ]; then
	echo "KEEP_RUNNING=1: containers left running for browser validation"
else
	cleanup
	trap - EXIT INT TERM
fi
