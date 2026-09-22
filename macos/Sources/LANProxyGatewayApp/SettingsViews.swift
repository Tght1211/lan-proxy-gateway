import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct SettingsView: View {
    @EnvironmentObject private var model: AppModel

    private var content: some View {
        Group {
            LANHTTPProxySettingsPanel()
            AgentSkillSettingsPanel()
            ThemeSelectorPanel()
            Panel {
                SettingsRow(title: "版本与更新", detail: model.updateStatus ?? "当前版本 v\(model.appVersion)") {
                    if model.updateAvailable {
                        Button {
                            NSWorkspace.shared.open(URL(string: "https://github.com/Tght1211/lan-proxy-gateway/releases/latest")!)
                        } label: { Label("下载新版本", systemImage: "arrow.down.circle") }
                        .buttonStyle(ActionButtonStyle(tint: Theme.lime))
                    }
                    Button {
                        model.checkForUpdates()
                    } label: {
                        if model.isCheckingUpdate {
                            ProgressView().controlSize(.small).frame(width: 60)
                        } else {
                            Label("检查更新", systemImage: "arrow.triangle.2.circlepath")
                        }
                    }
                    .buttonStyle(.bordered)
                    .disabled(model.isCheckingUpdate)
                }
                Divider().overlay(Theme.border)
                SettingsRow(title: "命令行工具", detail: "/usr/local/bin/gateway") {
                    HStack(spacing: 10) {
                        if model.cliNeedsCoreRestart {
                            Label("新版核心需重启后生效", systemImage: "exclamationmark.triangle")
                                .font(.caption).foregroundStyle(Theme.yellow)
                            Button("立即重启核心") { model.restartForNewCLI() }
                                .buttonStyle(ActionButtonStyle(tint: Theme.yellow))
                                .disabled(model.isBusy)
                        }
                        Button {
                            model.installCLI()
                        } label: {
                            if model.isInstallingCLI {
                                HStack(spacing: 6) {
                                    ProgressView().controlSize(.small)
                                    Text("安装中…")
                                }
                            } else {
                                Text("安装 CLI")
                            }
                        }
                        .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                        .disabled(model.isInstallingCLI)
                    }
                }
                Divider().overlay(Theme.border)
                SettingsRow(title: "开机自启", detail: "\(model.serviceStatus) · 使用 /usr/local/bin/gateway") {
                    HStack(spacing: 10) {
                        Text(model.isServiceInstalled ? "已开启" : "已关闭")
                            .font(.caption.weight(.medium))
                            .foregroundStyle(model.isServiceInstalled ? Theme.lime : Color.secondary)
                        Toggle("", isOn: Binding(
                            get: { model.isServiceInstalled },
                            set: { model.setServiceEnabled($0) }
                        ))
                        .labelsHidden()
                        .toggleStyle(.switch)
                        .tint(Theme.lime)
                        .disabled(model.isBusy)
                        .help(model.isServiceInstalled ? "关闭开机自启" : "启用开机自启")
                    }
                }
                Divider().overlay(Theme.border)
                SettingsRow(title: "配置文件", detail: model.status?.configFile ?? "--") { EmptyView() }
                Divider().overlay(Theme.border)
                SettingsRow(title: "开源项目", detail: "github.com/Tght1211/lan-proxy-gateway") {
                    Button {
                        NSWorkspace.shared.open(URL(string: "https://github.com/Tght1211/lan-proxy-gateway")!)
                    } label: { Label("GitHub", systemImage: "arrow.up.right.square") }
                    .buttonStyle(.bordered)
                }
            }
            Panel {
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Text("运行日志").sectionLabel()
                        Text("自动更新").font(.caption2).foregroundStyle(Theme.lime)
                        Spacer()
                        Button("刷新") { model.reloadLog() }.buttonStyle(.bordered)
                        Button("在访达中显示") { model.revealLog() }.buttonStyle(.bordered)
                    }
                    LiveLogView(text: model.logText)
                        .frame(height: 320)
                }
            }
        }
    }

    var body: some View {
        ScrollPage {
            content
        }
        .task { model.updateServiceStatus(); model.reloadLog() }
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
