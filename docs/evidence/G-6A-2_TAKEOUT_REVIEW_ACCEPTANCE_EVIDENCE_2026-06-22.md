# G-⑥a-2 Acceptance Evidence

## 1. 基线与门禁

```text
master=5eb3f7a6 docs(G-6a-1): record takeout admin acceptance
g6a1-takeout-admin-baseline=present
g5-rider-baseline=present
automigrate_count=0
main_worktree_clean=PASS
```

## 2. 真实接口快照

```text
review.Submitter.Submit(context.Context, Submission) (Task, error)
review.Target.ObjectType() string
review.Target.SetReviewStatus(context.Context, uint, review.Status) error
review.Status=submitted|reviewing|approved|rejected
F ordering=setTarget first, repository.Save second, no shared transaction/CAS
```

## 3. 真实 MySQL + HTTP

执行：

```text
go test ./internal/module/admin/takeoutconsole/reviewtarget \
  -run '^TestTakeoutReviewTargetsAgainstRealHTTPAndMySQL84$' -count=1 -v
```

脱敏结果：

```text
real_mysql_version=8.4.0
auth no_token=401 ordinary_user=403 admin=200
happy_path=PASS merchant approved/rejected; dish approved/rejected(draft retained)
F_non_atomicity=EXPOSED
http_status=400
target_error="apply takeout merchant review decision: merchant changed concurrently"
task_status=reviewing
merchant_status=pending_review
residue review_tasks=0 merchants=0 dishes=0 users=0
```

测试使用真实 `review.Service`、真实 GORM Repository、真实 takeout Service、真实用户 JWT 中间件、真实 Gin 路由和 `httptest.NewServer`。未使用 Mock DB，也未直接调用 HTTP Handler。

## 4. 回归证据

```text
go test ./... -count=1 = PASS
go vet ./... = PASS
go build ./... = PASS
npm run check:boundaries = PASS
npm run verify = PASS
npm run build = PASS
```

MiniApp build 仅出现基线已有的 `vendors.js` 推荐大小警告，不影响退出码。

## 5. 边界证据

```text
takeout_core_F_or_HTTP_imports=0
automigrate_count=0
migration_changes=0
review_package_changes=0
check_boundaries_changes=0
```

## 6. 实现提交

```text
761c63e267a655f39e803be883357605dc53c967
```
