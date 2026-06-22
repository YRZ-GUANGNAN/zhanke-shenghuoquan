# P0-4 Module ⑧ Acceptance Evidence — 2026-06-21

## Environment

- OS: Windows / PowerShell
- Database: MySQL 8.4.0（真实环境）
- Object storage: Tencent Cloud COS（真实 Bucket，Versioning Enabled）
- Secrets: 未记录；日志仅包含脱敏指纹

## Commands and results

### Module ⑧ MySQL / state / concurrency suite

```powershell
cd backend
go test ./internal/module/media -run '^(TestA|TestGC|TestMediaGC|TestTencentGC|TestZZMediaGC)' -count=1 -v
```

Result: PASS。

覆盖：migration/rollback、MySQL 8.4 schema、reference facts、错误缓存对照、审核/保留窗口、执行前重验、lease 并发、stale CAS、崩溃恢复、staging 清理、辅助记录 lease/CAS 清理、零残留。

### Real COS fixed-version deletion

```powershell
cd backend
$env:MEDIA_MODULE8_RUN_REAL_COS='1'
go test ./internal/module/media -run '^TestMediaGCDeletesOnlyFixedVersionAgainstRealCOS$' -count=1 -v
```

Result: PASS。

脱敏输出：

```text
PASS real COS versioning=Enabled
fixedDelete=MTg0...wMDI
survivor=MTg0...zMDA
request=NmEz...NmFi
repeatedDeleteAccepted=true
```

测试清理函数再次枚举该 exact key 的 versions 与 delete markers；残留为 0。

### Full backend regression

```powershell
cd backend
go test ./... -count=1
```

Result: PASS（所有 backend package）。

### Project verification

```powershell
npm run verify
```

Result: PASS。

```text
check:boundaries  PASS
admin typecheck  PASS
miniapp typecheck PASS
backend go test  PASS
```

## Frozen baseline hashes

以下 SHA-256 与 Module ⑧开发前记录完全一致：

```text
009 D4C23ECAB20A3A327E47730652BE165C719414A2C36159E180D87EACC50DE5DE
010 60D1316E5776176A130C0955F306188A003D58A4D8E36286E0BB6ADD7022B870
011 7C1058A57EFE07D23DF76B7E73B0682714344C2BD93A2236C5493E658FC9D34A
012 C98AB592F7C99B01870E13589E9BB528B7411EABF4D90A5D5ED52F855CA94C35
013 A39B8FF80F7FFAF1F137A9EF8D6A2EDFADCCA0E6D1F0A7DD21CFA524531E151B
consumer_types 038944BDEFB48D693C35710CC22DB24DCA06DAAEB59EBEAEF1D01B71BC461995
consumer_state_machine 08791E9D68C8B0252F827EA87BA4FB21CD74A93892CABC079F296EEC15A68161
consumer_repository 041E97B1DF752444D78C4D20D4DB43413B83ED41EFDCEE6EDE6037468AE007DC
consumer_service 8866344A1E5914E16DA4CEDAE2CF9DBEA7AC46C031B5C3DDFF2E7A3B8DE821A4
gateway_types 16BE47AD5A40A20E8D3EAC9BB4AED26406443F7A39E299A8E163132D1E35CFC5
gateway_repository C699C30CFD7C02C2A9971E1EEEF804413CB2AE2EAC305AA52F7D9158C7831C00
gateway_service 1AD2444433AB248ECF3DC5F878220AAE9CB66EFBDF7F0ADE53D23E3F7C282EAB
```

## Residue

- Database test owners: 0 assets and no dependent Module ⑧ rows.
- COS acceptance exact key: 0 versions, 0 delete markers.
- No HTTP route, scheduler, admin, miniapp, business-domain integration, Module ⑨ or Module ⑩ artifact added.
