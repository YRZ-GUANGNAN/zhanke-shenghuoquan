# 身份切换与配送单剪贴板导入实施计划

## 实施目标

完成身份切换弹窗、商家发布入口图标和配送单剪贴板导入，并确保现有审核、订单、支付、路由和真实数据逻辑不变。

## 任务一：建立可测试的剪贴板解析器

**文件：**

- 新增 `apps/miniapp/src/utils/delivery-order-clipboard.ts`
- 新增 `scripts/check-delivery-order-clipboard.cjs`
- 修改 `package.json`

**步骤：**

1. 定义配送表单字段、解析结果、字段标签和分段上下文。
2. 实现换行、全角冒号、明确标签、分段通用标签和续行解析。
3. 保证空内容、无标签内容和孤立数字不会产生字段。
4. 用真实执行的 Node 回归脚本覆盖标准模板、别名、分段、续行和无效内容。
5. 将回归脚本接入根目录 `verify`。

## 任务二：接入剪贴板读取与导入预览

**文件：**

- 修改 `apps/miniapp/src/services/taro.ts`
- 修改 `apps/miniapp/src/pages/merchant-publish/index.tsx`
- 修改 `apps/miniapp/src/pages/merchant-publish/index.scss`

**步骤：**

1. 在 Taro 门面增加 `getClipboardData` 类型。
2. 在表单顶部增加批量导入卡片与复制模板入口。
3. 读取剪贴板并调用纯解析器；为空、失败、无识别字段时给出明确反馈。
4. 使用现有 `AppActionSheet` 展示字段预览，标记新增或更新。
5. 用户确认后仅覆盖已识别非空字段；取消不改变表单。
6. 保持发布、支付、测试费用和重定向逻辑原样。

## 任务三：升级身份切换面板

**文件：**

- 修改 `apps/miniapp/src/components/IdentitySwitchModal.tsx`
- 修改 `apps/miniapp/src/components/IdentitySwitchModal.scss`

**步骤：**

1. 将标题改为动作式文案并压缩说明。
2. 使用现有角色图标构建当前/目标双卡片和方向连接。
3. 调整卡片层级、状态标签、间距和窄屏适配。
4. 保持确认、取消、加载和错误接口完全不变。

## 任务四：重绘商家发布配送单图标

**文件：**

- 修改 `apps/miniapp/src/pages/merchant-home/icons/action-publish.svg`

**步骤：**

1. 移除叠加圆形加号。
2. 使用白色统一线宽绘制配送单据和路线箭头。
3. 在现有蓝色快捷入口容器中检查清晰度和居中比例。

## 任务五：验证与交付候选

**步骤：**

1. 运行剪贴板解析器回归。
2. 运行 miniapp 类型检查与相关 UI/业务静态检查。
3. 运行 `npm run build:miniapp`。
4. 运行全量 `npm run verify`。
5. 检查 `git diff`、工作区状态和未跟踪用户文件，确保未混入无关内容。
6. 提交实现代码。
7. 生成新的 miniapp 上传候选目录和 ZIP，计算 SHA-256；不执行微信上传。
