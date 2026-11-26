# Sub2API Magic

基于 [sub2api](https://github.com/Wei-Shaw/sub2api) 的个人魔改版，主要优化界面、账号管理和用量统计。感谢上游作者及贡献者。

## 魔改项目

- 黑白灰主题适配深浅色，开关保持绿色，卡片去边框和阴影
- 统一 `4px` 圆角、`40px` 控件和 `64px` 顶部高度，优化分页与分隔线
- 主题切换移至右上角图标并保存偏好，图表配色实时跟随
- 滚动条按需显示、空闲 `0.8s` 后隐藏，优化页面越界和长表格滚动
- 设置页保存按钮移至顶部，与标签栏固定在滚动内容之外
- 渠道管理随“可用渠道”开关显隐，管理员个人菜单增加 `/dashboard`
- 分组逐行换行，平台名称不折行，批量栏仅在勾选账号后显示
- OpenAI Free 显示月度用量，付费账号保留 5h / 7d，各窗口增加“预计”额度
- 账号用量合并简写，隐藏用户费用、点数和邀请，保留配额查询与重置
- 统计支持精确时间与最近 24 小时范围，缓存率统一按全部输入 Token 计算

## 构建与同步

2026-11-26 同步至官方 `main` 的 [`81ce8356d`](https://github.com/Wei-Shaw/sub2api/commit/81ce8356d9a289411987bdefeddc9710340d6d52)，版本号为 `v0.2.25`。

```bash
docker build --push -t ikxin/sub2api-magic:latest .
```

部署参考 [部署文档](deploy/README.md)，使用自行构建的 Magic 镜像。后续通过单文件 [SKILL.md](skills/sub2api-upstream-sync/SKILL.md) 将 sub2api 上游更新合并到魔改版，保留上述修改。

## 开源许可

沿用上游 **LGPL v3.0 或更高版本**，见 [LICENSE](LICENSE)。保留上游版权和 [CLA](CLA.md)，再分发遵守上游及第三方依赖的许可。本分支独立维护，上述魔改于 2026-11-17 整理。

Copyright (c) 2026 Wesley Liddick
