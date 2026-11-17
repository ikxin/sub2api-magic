---
name: sub2api-upstream-sync
description: "将 sub2api 官方上游更新合并到 Sub2API Magic，保留魔改并验证。用于同步或评估上游分支、标签的更新。"
---

# 同步 sub2api 到 Sub2API Magic

- 先读项目 [README 魔改项目](../../README.md) 和当前差异，确认需要保留的修改。
- 检查 Git 状态和远程，保全暂存、未暂存及未跟踪文件。直接在本地 `main` 分支同步，不为同步创建新分支或 worktree；若当前不在 `main`，先保全当前工作再切换。
- 官方上游为 `https://github.com/Wei-Shaw/sub2api.git`，使用名为 `upstream` 的远程。未配置时执行 `git remote add upstream https://github.com/Wei-Shaw/sub2api.git`；已配置时核对地址，避免把 Magic 的 `origin` 当作官方上游。
- 执行 `git fetch upstream`，默认以 `upstream/main` 为合并目标并记录目标 SHA；用户指定其他上游分支或标签时使用指定目标。
- 在本地 `main` 上合并 `upstream/main`（或用户指定的上游目标），逐项保留 README 中的魔改；逐段处理冲突，不整批覆盖。上游已有等价修复时优先复用，并更新魔改项目。
- 坚持最小改动，优先修改现有文件、复用现有组件；保持上游功能、接口和配置兼容。
- 按当前 Makefile / CI 执行相关测试、类型检查和构建，重点验证主题与滚动、条件显隐、账号配额、时间范围和缓存率；如实说明未验证的部分。
- 保留 Magic README、上游许可证、版权和 CLA，简要报告同步范围、魔改保留情况及验证结果。提交、推送和部署按用户已有授权执行。
