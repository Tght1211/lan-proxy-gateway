# Release 文案维护

对外下载说明以 [最新 Release](https://github.com/Tght1211/lan-proxy-gateway/releases/latest) 的真实附件为准。普通用户看 [下载与安装](../download.md)。

## 每次发布都会带下载表

`.github/workflows/release.yml` 在构建附件、生成校验和后，运行 `scripts/render-release-notes.py`。生成的正文由两部分组成：

1. `download-guide.md`：面向新手的安装包对照表，自动替换标签和仓库地址；放在最前面。
2. `latest.md`：本次版本的更新、升级和验证说明，由发布者在打标签前更新。

更新说明的标题必须匹配当前标签，表中链接的文件必须出现在本次实际构建目录；版本不匹配或缺少附件时生成失败，避免发布错误下载链接。不要把开发机生成的单架构 DMG 冒充 universal 包。

```bash
python3 scripts/render-release-notes.py \
  --tag v4.5.6 \
  --repo Tght1211/lan-proxy-gateway \
  --assets-dir dist \
  --output dist/release-notes.md
```

这只生成文案，不创建 Release、推送标签或上传附件。版本号要与实际附件及 `latest.md` 一致。

## 已发布版本的说明修订

已有 Release 可以仅补充安装表，保留原有更新记录和全部附件。使用 GitHub API/CLI 读取真实的 `tagName,body,assets`，备份原文，在检查生成文案后更新正文；不要重新构建或替换安装包。`--release-json` 可使用 `gh release view --json tagName,body,assets` 的结果核对文件名。

`v4.5.6.md` 保存 v4.5.6 的下载说明和原发布记录。主分支当时的源码/`latest.md` 仍对应 v4.5.2；文档更新并未合并 v4.5.3–v4.5.6 的功能代码。维护者发布下个版本时，应先确认目标提交与版本说明一致，不能仅因存在新手手册就认为主分支已包含最新功能。

准备 v4.5.7 时已将 v4.5.6 标签及其中的历史功能提交合入主分支。v4.5.7 在该运行代码基线上发布图文文档、下载表和 Docker 验证工具；Linux 已知缺陷见对应发布说明，不能因版本号更新就认为已经修复。
