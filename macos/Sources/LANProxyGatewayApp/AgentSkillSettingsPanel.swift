import AppKit
import SwiftUI
import UniformTypeIdentifiers

struct AgentSkillSettingsPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var exporting = false
    @State private var exportedURL: URL?
    @State private var message = ""

    private var installPrompt: String {
        let archive = exportedURL?.path ?? "我提供的 lan-proxy-gateway-skill.zip"
        let executable = model.client.bundledEngineURL?.path ?? "gateway"
        return """
        请把 \(archive) 中的 lan-proxy-gateway 目录安装为你可使用的 Skill，保留 SKILL.md 和 references 目录。
        网关运行在这台电脑上，CLI 路径是：\(executable)
        安装后阅读 Skill，先检查版本并运行 agent snapshot，向我说明当前代理、连接和运行状态。先不要更改配置；后续按我的具体要求控制网关。
        """
    }

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                Text("外部 Agent Skill").sectionLabel()
                Text("让你自己的 AI Agent 查询网关状态、诊断连接并按需调整配置。应用无需配置模型或 API Key。")
                    .font(.caption).foregroundStyle(Theme.muted)
                Text("1. 导出 Skill ZIP\n2. 将 ZIP 和安装说明交给支持 SKILL.md 的 Agent\n3. 安装后，通过对话管理本机网关")
                    .font(.callout).fixedSize(horizontal: false, vertical: true)
                Text("Agent 需要能够在网关所在电脑执行命令。Skill 包不包含本机配置或密码，管理接口保持仅本机可访问。")
                    .font(.caption).foregroundStyle(Theme.muted)
                HStack {
                    Button(exporting ? "导出中…" : "导出 Skill ZIP") { export() }
                        .buttonStyle(ActionButtonStyle(tint: Theme.cyan)).disabled(exporting)
                    Button("复制安装说明") {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(installPrompt, forType: .string)
                        message = "安装说明已复制"
                    }.buttonStyle(StudioButtonStyle())
                    if let exportedURL {
                        Button("显示文件") { NSWorkspace.shared.activateFileViewerSelecting([exportedURL]) }
                            .buttonStyle(StudioButtonStyle())
                    }
                    Spacer()
                }
                if !message.isEmpty { Text(message).font(.caption).foregroundStyle(Theme.muted) }
            }
        }
    }

    private func export() {
        let panel = NSSavePanel()
        panel.title = "导出外部 Agent Skill"
        panel.nameFieldStringValue = "lan-proxy-gateway-skill.zip"
        panel.allowedContentTypes = [.zip]
        guard panel.runModal() == .OK, let url = panel.url else { return }
        exporting = true
        Task {
            defer { exporting = false }
            do {
                _ = try await model.client.exportAgentSkill(to: url)
                exportedURL = url
                message = "已导出。将 ZIP 和安装说明交给你的 Agent。"
            } catch { message = "导出失败：\(error.localizedDescription)" }
        }
    }
}
