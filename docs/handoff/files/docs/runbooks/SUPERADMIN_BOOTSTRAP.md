# 超级管理员首次引导

仅在生产库尚无 `superadmin` 时，由数据库管理员执行一次。目标用户必须先通过真实微信登录创建账号；禁止使用 `mock_openid_%` 账号。

```sql
START TRANSACTION;
SELECT id, open_id, role, status FROM users WHERE open_id = '<verified-wechat-openid>' FOR UPDATE;
UPDATE users
SET role = 'superadmin', status = 'active', token_version = token_version + 1
WHERE open_id = '<verified-wechat-openid>' AND open_id NOT LIKE 'mock_openid_%';
COMMIT;
```

执行后立即删除终端历史中的真实 OpenID，并由该超级管理员重新登录。后续管理员和超级管理员变更必须使用后台受控接口，不再直接修改数据库。
