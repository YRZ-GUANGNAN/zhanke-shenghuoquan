# 超级管理员首次引导

后台管理员账号已与小程序用户解耦：首个超级管理员由运维 CLI 直接创建，**不再**需要先用微信登录、再修改 `users.role`。

```bash
sudo -u campus /opt/campus-circle/current/bin/admin-credential bootstrap \
  --username administrator --role superadmin --display-name 超级管理员
```

- 命令必须在交互式终端执行，密码输入两次且不会回显（14–72 个字符）。
- 可以重复执行：同名账号已存在时仅重置其密码。
- 创建成功即可在管理后台登录；后续新增管理员请在后台「账号与权限 → 管理员账号」面板操作，不再直接修改数据库。
- 不要通过 `UPDATE users SET role='superadmin'` 授予后台访问权限——`users.role` 仅影响小程序站内权限（如「仅管理员可发帖」）。
