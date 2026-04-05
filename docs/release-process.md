# 发布流程

## 版本策略

- 破坏性变更: 升主版本
- 兼容的新能力: 升次版本
- bugfix / 文档修正: 升补丁版本

当前计划版本:

- `v2.2.10`

## 发布前检查

1. 确认 `README.md`、`README_EN.md`、`CHANGELOG.md` 已更新
2. 确认 `docs/releases/latest.md` 已写好
3. 跑完整测试:

```bash
go test ./...
```

4. 构建 release 资产:

```bash
make build-all VERSION=v2.2.10
```

## Release 资产

`make build-all` 会生成:

- 原始二进制
- 压缩包
- `SHA256SUMS`
- `release-notes.md`

## 触发正式发布

```bash
git tag v2.2.10
git push origin main
git push origin v2.2.10
```

GitHub Actions 会自动:

1. 构建多平台资产
2. 上传到 GitHub Release
3. 使用 `docs/releases/latest.md` 作为 release 说明
4. 对应 workflow 是 `.github/workflows/release.yml` 里的 `Release`

如果 release 被手动删掉、或者资产不完整，不需要重新打新版本，直接在 GitHub Actions 里重新运行该 tag 对应的 `Release` workflow 即可补发。

## 发布后检查

1. GitHub Release 页面是否包含全部资产
2. Windows / macOS / Linux 文件名是否正确
3. `gateway update` 是否能拉到新版本
4. README 中的下载与安装说明是否和 release 对得上
