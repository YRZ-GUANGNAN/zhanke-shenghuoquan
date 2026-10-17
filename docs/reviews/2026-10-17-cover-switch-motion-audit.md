# 默认分享封面切换与旧动效审查

基线：`fcb8e55`。工作区：`codex/loading-feedback-polish-20261017`。本轮只改隔离分支，未修改原项目、线上配置、后端接口、权限或业务开关。

## 默认封面

后台“微信分享卡片 → 分享封面”现在有两个明确来源：

- **内置默认封面**：保存 `defaultImageUrl: ""`，使用小程序包内的品牌 PNG。
- **自定义图片**：填写并预览经过原有校验的公开 HTTPS JPG/PNG；只允许本站或腾讯云 COS 域名，不接受临时签名地址。

切换回内置时，自定义地址仍保存在当前草稿；再次切回自定义不会丢失输入。自定义地址为空或不合法时不能保存，预览会显示内置图并提示原因。保存仍调用原有 `PUT /admin/share-settings`、CSRF、管理员权限和 `expectedRowVersion` CAS；没有新增上传接口，也没有把本地预览冒充上传结果。

## 动效自检与修正

根据项目现有动效检查和参考库 `ffbefc4` 的手势/动效静态研究，保留约 120–260ms 的局部过渡、原生横滑、边缘返回、评论面板拖拽、正文测量展开和刷新生命周期；没有复制第三方资源或添加整页动画。

本轮修正：

1. 弹层在退出动画尚未完成时重新打开，会直接反转到已打开状态，不再回到入场起点造成闪烁；退出定时器会被清理。
2. 对话框进入退出阶段或 `visible=false` 时不再响应确认/取消点击。
3. 后台加载状态移除旧版父标记旋转，保留新版内部旋转指示器，避免双层旋转。
4. 图标按钮、资料头像、设置开关补充 `prefers-reduced-motion` 规则；移除弹层和图标按钮的长期 `will-change` 提示，降低常驻合成层开销。

骑手进度、校园服务加载、骨架、刷新、正文/回复、点赞、二手评论、图集计时器等已有动效均保留既有业务语义和取消清理；本轮未扩大为页面重做。

## 验证

本轮已通过：

- `npm.cmd run check:admin-share-default`
- `npm.cmd run check:layer-presence`
- `npm.cmd run check:content-motion`
- `npm.cmd run check:refresh-feedback`
- `npm.cmd run check:tactile-navigation`
- `npm.cmd run check:profile-edge-back`
- `npm.cmd run check:secondhand-presentation`
- `npm.cmd run check:gallery-presentation`
- `npm.cmd run check:channel-pager`
- `npm.cmd run check:share-cards`
- `npm.cmd run check:feature-gate-feedback`
- `npm.cmd run check:admin-ui-system`
- `npm.cmd run check:miniapp-ui-system`
- `npm.cmd run check:home-real-tools-contract`
- `npm.cmd run typecheck`
- `npm.cmd run build:admin`
- `npm.cmd run build:miniapp`
- `git diff --check`

构建产物检查确认正式小程序仍为 30 个业务页面，未包含 `feedback-review` 检查页。构建仍报告基线已有的 CSS 顺序和 `vendors.js` 体积建议警告，不影响生成产物。

本轮尝试用隔离检查工程连接微信开发者工具自动化通道，重载后通道在页面查询阶段超时，未形成新的原生交互通过证据；因此没有把它写成已通过。减少动效、真机触感、微信最终分享卡片和原生面板拖动仍需在开发者工具/真机中人工复核。

本轮没有部署、上传微信体验版、推送 GitHub 或修改线上封面。
