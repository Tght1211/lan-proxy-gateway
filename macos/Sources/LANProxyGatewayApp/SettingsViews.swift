import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct SettingsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var category = "features"
    @State private var logFilter = ""
    @State private var paused = false
    @State private var frozenLog = ""

    private var displayedLog: String {
        let source = paused ? frozenLog : model.logText
        guard !logFilter.isEmpty else { return source }
        return source.components(separatedBy: .newlines).filter { $0.localizedCaseInsensitiveContains(logFilter) }.joined(separator: "\n")
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            StudioTabs(title: "设置分类", selection: $category, items: [
                ("features", "功能与接入", "settings"), ("general", "通用", ""),
                ("updates", "版本与更新", "arrow.clockwise"), ("agent", "CLI 与 Agent", ""), ("logs", "实时日志", "list.bullet.rectangle")
            ]).padding(.horizontal,22).padding(.top,16)
            ScrollPage {
                switch category {
                case "features":
                    Panel {
                        SettingsRow(title: "热点流量接管", detail: model.hotspotControlDetail) {
                            Toggle("热点流量接管", isOn: Binding(get: { model.hotspot?.enabled == true }, set: { enabled in
                                Task { await model.configureHotspot(enabled ? "enable" : "disable") }
                            })).labelsHidden().toggleStyle(.switch).disabled(model.isBusy || model.hotspotOperation != nil || !model.isConfigured)
                        }
                        Divider()
                        SettingsRow(title: "静态网关接入", detail: "局域网入口：\(model.status?.gateway.localIP ?? "—")") {
                            Button(model.status?.accessMode == "hotspot" ? "切换到局域网接管" : "正在使用局域网接管") {
                                Task { await model.configureHotspot("use-lan") }
                            }.disabled(model.status?.accessMode != "hotspot" || model.isBusy)
                        }
                        Text("当前核心的热点接管与静态网关接管使用同一套转发模式；HTTP / PAC 可同时开启。").font(.caption).foregroundStyle(Theme.muted)
                    }
                    LANHTTPProxySettingsPanel()
                case "general":
                    ThemeSelectorPanel()
                    Panel {
                        SettingsRow(title: "开机自启", detail: model.serviceStatus) {
                            Toggle("开机自启", isOn: Binding(get: { model.isServiceInstalled }, set: { model.setServiceEnabled($0) }))
                                .labelsHidden().toggleStyle(.switch).disabled(model.isBusy)
                        }
                        Divider()
                        SettingsRow(title: "流光色系", detail: "流光随真实连接自动播放；系统减少动态效果时显示静态结果。慢响应为黄色，错误为红色。") {
                            NetworkFlowPalettePicker()
                        }
                        Divider()
                        SettingsRow(title: "配置文件", detail: model.status?.configFile ?? "尚未初始化") {
                            if let path = model.status?.configFile { Button("在访达中显示") { NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: path)]) } }
                        }
                        Divider()
                        SettingsRow(title: "开源项目", detail: "github.com/Tght1211/lan-proxy-gateway") {
                            Link("GitHub ↗", destination: URL(string: "https://github.com/Tght1211/lan-proxy-gateway")!)
                        }
                    }
                case "updates":
                    Panel {
                        SettingsRow(title: "LAN Proxy Gateway", detail: model.updateStatus ?? model.buildVersion.detail) {
                            Button(model.isCheckingUpdate ? "检查中…" : "检查更新") { model.checkForUpdates() }.disabled(model.isCheckingUpdate)
                            if model.updateAvailable { Link("下载新版本 ↗", destination: URL(string: "https://github.com/Tght1211/lan-proxy-gateway/releases/latest")!) }
                        }
                        Text("新版本通过项目发布页安装。").font(.caption).foregroundStyle(Theme.muted)
                    }
                case "agent":
                    Panel {
                        SettingsRow(title: "命令行工具", detail: "/usr/local/bin/gateway") {
                            Button(model.isInstallingCLI ? "安装中…" : "安装 CLI") { model.installCLI() }.disabled(model.isInstallingCLI)
                        }
                        if model.cliNeedsCoreRestart {
                            SettingsRow(title: "新版核心已安装", detail: "重启后使用新版；已连接设备会短暂断网") {
                                Button("重启核心") { model.restartForNewCLI() }.disabled(model.isBusy)
                            }
                        }
                    }
                    AgentSkillSettingsPanel()
                default:
                    Panel {
                        HStack {
                            Text("运行日志").font(.headline)
                            Spacer()
                            TextField("过滤日志", text: $logFilter).textFieldStyle(StudioFieldStyle()).frame(width: 220)
                            Button(paused ? "继续更新" : "暂停更新") {
                                if !paused { frozenLog = model.logText }; paused.toggle()
                            }
                            Button("刷新") { model.reloadLog() }.disabled(paused)
                            Button("显示文件") { model.revealLog() }.disabled(model.status?.logFile == nil)
                        }
                        Text(model.status?.logFile ?? "等待日志地址").font(.caption.monospaced()).foregroundStyle(Theme.muted).textSelection(.enabled)
                        LiveLogView(text: displayedLog).frame(height: 430)
                    }
                }
            }
        }.task { model.updateServiceStatus(); model.reloadLog(); await model.refreshHotspot() }
    }
}

struct LiveLogView: View {
    let text: String
    private let bottomID = "log-bottom"

    var body: some View {
        // Two-axis ScrollViews center content narrower than the viewport;
        // pin the log block to at least viewport width, leading-aligned.
        GeometryReader { geo in
            ScrollViewReader { proxy in
                ScrollView([.horizontal, .vertical], showsIndicators: true) {
                    VStack(alignment: .leading, spacing: 0) {
                        Text(text)
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(Color.primary.opacity(0.78))
                            .textSelection(.enabled)
                            .multilineTextAlignment(.leading)
                            .fixedSize(horizontal: true, vertical: true)
                        Color.clear.frame(height: 1).id(bottomID)
                    }
                    .frame(minWidth: geo.size.width, alignment: .topLeading)
                }
                .onAppear { scrollToBottom(proxy, animated: false) }
                .onChange(of: text) { _ in scrollToBottom(proxy, animated: true) }
            }
        }
        .frame(minHeight: 180, maxHeight: .infinity)
        .clipped()
    }

    private func scrollToBottom(_ proxy: ScrollViewProxy, animated: Bool) {
        DispatchQueue.main.async {
            if animated {
                withAnimation(.easeOut(duration: 0.22)) { proxy.scrollTo(bottomID, anchor: .bottomLeading) }
            } else {
                proxy.scrollTo(bottomID, anchor: .bottomLeading)
            }
        }
    }
}
