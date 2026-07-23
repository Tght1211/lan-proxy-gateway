import AppKit
import Charts
import SwiftUI

struct ThemePalette: Identifiable {
    let id: String
    let name: String
    let isDark: Bool
    let canvas: Color
    let sidebar: Color
    let panel: Color
    let panelRaised: Color
    let border: Color
    let cyan: Color
    let coral: Color
    let lime: Color
    let yellow: Color
    let muted: Color

    static let light = ThemePalette(
        id: "light",
        name: "经典浅色",
        isDark: false,
        canvas: Color(red: 0.955, green: 0.961, blue: 0.969),
        sidebar: Color(red: 0.925, green: 0.937, blue: 0.945),
        panel: Color.white,
        panelRaised: Color(red: 0.969, green: 0.975, blue: 0.979),
        border: Color(red: 0.835, green: 0.855, blue: 0.875),
        cyan: Color(red: 0.08, green: 0.42, blue: 0.36),
        coral: Color(red: 0.72, green: 0.20, blue: 0.18),
        lime: Color(red: 0.20, green: 0.52, blue: 0.28),
        yellow: Color(red: 0.78, green: 0.47, blue: 0.08),
        muted: Color(nsColor: .secondaryLabelColor)
    )

    static let graphite = ThemePalette(
        id: "graphite",
        name: "石墨深色",
        isDark: true,
        canvas: Color(red: 0.082, green: 0.09, blue: 0.106),
        sidebar: Color(red: 0.104, green: 0.114, blue: 0.133),
        panel: Color(red: 0.125, green: 0.137, blue: 0.157),
        panelRaised: Color(red: 0.16, green: 0.175, blue: 0.20),
        border: Color(red: 0.235, green: 0.258, blue: 0.294),
        cyan: Color(red: 0.30, green: 0.76, blue: 0.66),
        coral: Color(red: 0.94, green: 0.45, blue: 0.40),
        lime: Color(red: 0.55, green: 0.79, blue: 0.40),
        yellow: Color(red: 0.94, green: 0.70, blue: 0.32),
        muted: Color(red: 0.60, green: 0.64, blue: 0.70)
    )

    static let ocean = ThemePalette(
        id: "ocean",
        name: "海雾蓝",
        isDark: false,
        canvas: Color(red: 0.928, green: 0.947, blue: 0.965),
        sidebar: Color(red: 0.885, green: 0.913, blue: 0.941),
        panel: Color.white,
        panelRaised: Color(red: 0.945, green: 0.960, blue: 0.975),
        border: Color(red: 0.775, green: 0.828, blue: 0.878),
        cyan: Color(red: 0.10, green: 0.36, blue: 0.65),
        coral: Color(red: 0.78, green: 0.23, blue: 0.22),
        lime: Color(red: 0.12, green: 0.50, blue: 0.44),
        yellow: Color(red: 0.79, green: 0.50, blue: 0.10),
        muted: Color(nsColor: .secondaryLabelColor)
    )

    static let cream = ThemePalette(
        id: "cream",
        name: "暖沙米",
        isDark: false,
        canvas: Color(red: 0.960, green: 0.943, blue: 0.912),
        sidebar: Color(red: 0.928, green: 0.903, blue: 0.862),
        panel: Color(red: 0.995, green: 0.986, blue: 0.968),
        panelRaised: Color(red: 0.963, green: 0.948, blue: 0.922),
        border: Color(red: 0.838, green: 0.798, blue: 0.732),
        cyan: Color(red: 0.62, green: 0.31, blue: 0.14),
        coral: Color(red: 0.74, green: 0.22, blue: 0.18),
        lime: Color(red: 0.35, green: 0.51, blue: 0.25),
        yellow: Color(red: 0.71, green: 0.49, blue: 0.10),
        muted: Color(nsColor: .secondaryLabelColor)
    )

    static let all: [ThemePalette] = [.light, .graphite, .ocean, .cream]

    static func named(_ id: String) -> ThemePalette {
        all.first { $0.id == id } ?? .light
    }
}

private enum Theme {
    static var palette = ThemePalette.light
    static var canvas: Color { palette.canvas }
    static var sidebar: Color { palette.sidebar }
    static var panel: Color { palette.panel }
    static var panelRaised: Color { palette.panelRaised }
    static var border: Color { palette.border }
    static var cyan: Color { palette.cyan }
    static var coral: Color { palette.coral }
    static var lime: Color { palette.lime }
    static var yellow: Color { palette.yellow }
    static var muted: Color { palette.muted }
}

struct ContentView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        // Swap the palette before subviews evaluate; .id forces a full rebuild
        // so every cached view picks up the new colors.
        Theme.palette = ThemePalette.named(model.themeID)
        return mainView
            .id(model.themeID)
            .preferredColorScheme(Theme.palette.isDark ? .dark : .light)
    }

    private var mainView: some View {
        NavigationSplitView {
            sidebar
        } detail: {
            VStack(spacing: 0) {
                TopBar()
                Divider().overlay(Theme.border)
                if model.coreUpgradeRecommended {
                    CoreCompatibilityBar()
                }
                detail
            }
            .background(Theme.canvas)
        }
        .tint(Theme.cyan)
        .toolbar(.hidden, for: .windowToolbar)
        .alert("操作失败", isPresented: Binding(
            get: { model.errorMessage != nil },
            set: { if !$0 { model.errorMessage = nil } }
        )) {
            Button("关闭", role: .cancel) { model.errorMessage = nil }
        } message: {
            Text(model.errorMessage ?? "未知错误")
        }
        .overlay(alignment: .bottom) {
            if let notice = model.notice {
                NoticeBar(text: notice).padding(.bottom, 18)
            }
        }
        .animation(.easeOut(duration: 0.2), value: model.notice)
    }

    private var sidebar: some View {
        VStack(spacing: 0) {
            HStack(spacing: 11) {
                ZStack {
                    RoundedRectangle(cornerRadius: 7).fill(Theme.cyan.opacity(0.16))
                    Image(systemName: "network").foregroundStyle(Theme.cyan).font(.system(size: 17, weight: .semibold))
                }
                .frame(width: 32, height: 32)
                VStack(alignment: .leading, spacing: 2) {
                    Text("旁路由").font(.system(size: 14, weight: .semibold))
                    Text("LAN Proxy Gateway").font(.system(size: 10)).foregroundStyle(Theme.muted)
                }
                Spacer()
            }
            .padding(.horizontal, 16)
            .frame(height: 68)

            List(AppSection.allCases, selection: $model.selectedSection) { section in
                Label(section.rawValue, systemImage: section.systemImage)
                    .font(.system(size: 13, weight: .medium))
                    .symbolRenderingMode(.hierarchical)
                    .tag(section)
                    .frame(minHeight: 27)
            }
            .scrollContentBackground(.hidden)
            .scrollIndicators(.hidden)
            .listStyle(.sidebar)

            VStack(alignment: .leading, spacing: 9) {
                HStack(spacing: 8) {
                    Circle().fill(model.isRunning ? Theme.lime : Theme.muted).frame(width: 7, height: 7)
                    Text(model.isRunning ? "服务运行中" : "服务已停止")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundStyle(model.isRunning ? Theme.lime : Theme.muted)
                }
                Text(model.status?.gateway.localIP.nonEmpty ?? "等待网络检测")
                    .font(.system(size: 12, design: .monospaced)).foregroundStyle(Theme.muted)
            }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.panel.opacity(0.72))
        }
        .background(Theme.sidebar)
        .navigationSplitViewColumnWidth(min: 172, ideal: 184, max: 200)
    }

    @ViewBuilder private var detail: some View {
        Group {
            switch model.selectedSection ?? .overview {
            case .overview: OverviewView()
            case .devices: DevicesView()
            case .connections: ConnectionsView()
            case .settings: SettingsView()
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
    }
}

private struct CoreCompatibilityBar: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: "arrow.triangle.2.circlepath.circle.fill").foregroundStyle(Theme.yellow)
            Text("检测到较早版本的核心服务，部分实时数据不可用。")
                .font(.system(size: 12, weight: .medium))
            Spacer()
            Button("使用当前核心重启") { model.restart() }
                .buttonStyle(ActionButtonStyle(tint: Theme.yellow))
                .disabled(model.isBusy)
        }
        .padding(.horizontal, 20)
        .frame(height: 44)
        .background(Theme.yellow.opacity(0.09))
        .overlay(alignment: .bottom) { Divider().overlay(Theme.border) }
    }
}

private struct TopBar: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        HStack(spacing: 14) {
            VStack(alignment: .leading, spacing: 2) {
                Text(model.selectedSection?.rawValue ?? "网络总览").font(.system(size: 18, weight: .semibold))
                Text(model.isRunning ? "网关服务正常 · \(model.status?.gateway.localIP.nonEmpty ?? "正在检测网络")" : "网关服务未运行")
                    .font(.system(size: 11)).foregroundStyle(Theme.muted)
            }
            Spacer()
            if model.isBusy { ProgressView().controlSize(.small) }
            Button { Task { await model.refresh() } } label: { Image(systemName: "arrow.clockwise") }
                .buttonStyle(IconButtonStyle()).help("刷新")
            if model.isRunning {
                Button { model.restart() } label: { Label("重启", systemImage: "arrow.triangle.2.circlepath") }
                    .buttonStyle(.bordered).help("重启核心服务")
                Button { model.stop() } label: { Label("停止核心", systemImage: "stop.fill") }
                    .buttonStyle(ActionButtonStyle(tint: Theme.coral))
            } else {
                Button { model.initializeAndStart() } label: {
                    Label(model.isConfigured ? "启动核心" : "初始化核心", systemImage: "play.fill")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.lime))
            }
        }
        .padding(.horizontal, 24)
        .frame(height: 58)
        .background(Theme.canvas)
        .disabled(model.isBusy)
    }
}

// Layout rule: every content page either uses ScrollPage (scrollable, panels
// stack freely) or is a full-height page where ONLY flexible views (Table,
// TextEditor) absorb remaining space. Fixed-height stacks that can outgrow
// the window overflow-center and shove the whole split view off screen.
private struct ScrollPage<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        ScrollView(.vertical, showsIndicators: false) {
            VStack(spacing: 16) { content }
                .padding(20)
                .frame(maxWidth: .infinity)
        }
    }
}

private struct OverviewView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollPage {
            if model.status?.configured == false {
                GettingStartedPanel()
            }
            GatewaySummary()
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 12), count: 4), spacing: 12) {
                MetricCard("实时下载", speed(model.stats?.relay.traffic.last?.down ?? 0), "arrow.down", Theme.cyan)
                MetricCard("实时上传", speed(model.stats?.relay.traffic.last?.up ?? 0), "arrow.up", Theme.yellow)
                MetricCard("活动连接", "\(model.stats?.relay.active.count ?? 0)", "point.3.connected.trianglepath.dotted", Theme.lime)
                MetricCard("活跃设备", "\(model.activeDeviceCount)", "desktopcomputer", Theme.coral)
            }
            TopologyPanel()
            ThroughputChart(compact: true).frame(minHeight: 270)
            RecentStrip().frame(minHeight: 210)
        }
    }
}

private struct TopologyPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var showRoutingEditor = false
    @State private var showProxyConfig = false

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    Text("流量拓扑").sectionLabel()
                    Spacer()
                    Text("设备 → 网关 → 规则 → 出口").font(.caption).foregroundStyle(Theme.muted)
                    Button { showRoutingEditor = true } label: {
                        Label("管理规则", systemImage: "slider.horizontal.3").font(.caption)
                    }
                    .buttonStyle(.bordered).controlSize(.small)
                    LiveBadge(active: model.isRunning && (model.stats?.health.healthy ?? false))
                }
                HStack(alignment: .top, spacing: 18) {
                    RouteDiagram(
                        onEditRules: { showRoutingEditor = true },
                        onEditProxy: { showProxyConfig = true }
                    )
                    .frame(maxWidth: .infinity)
                    VStack(spacing: 12) {
                        StabilitySummary()
                        StabilityChart(compact: true)
                    }
                    .frame(width: 320)
                }
            }
        }
        .sheet(isPresented: $showRoutingEditor) {
            RoutingRulesEditor(rules: model.status?.routing ?? [])
                .environmentObject(model)
        }
        .sheet(isPresented: $showProxyConfig) {
            ProxyConfigSheet().environmentObject(model)
        }
    }
}


// Compact entry for the fallback auto-learning feature: a status pill that
// lights up while learning is happening and expands into a detail popover.
private struct FallbackLearningBadge: View {
    @EnvironmentObject private var model: AppModel
    @State private var showDetail = false
    @State private var showRoutingEditor = false

    private var candidates: [FallbackCandidate] { model.stats?.fallback?.candidates ?? [] }
    private var learnedRules: [RoutingRule] { model.stats?.fallback?.learned ?? [] }
    private var isActive: Bool { !candidates.isEmpty || !learnedRules.isEmpty }

    var body: some View {
        Button { showDetail = true } label: {
            HStack(spacing: 6) {
                Image(systemName: "wand.and.stars")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(isActive ? Theme.yellow : Theme.muted)
                Text("智能学习")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(isActive ? Color.primary : Theme.muted)
                if !candidates.isEmpty {
                    Text("\(candidates.count)")
                        .font(.system(size: 10, weight: .bold, design: .monospaced))
                        .foregroundStyle(Theme.yellow)
                        .padding(.horizontal, 5).padding(.vertical, 1)
                        .background(Theme.yellow.opacity(0.14))
                        .clipShape(Capsule())
                }
                if !learnedRules.isEmpty {
                    HStack(spacing: 3) {
                        Image(systemName: "checkmark").font(.system(size: 7, weight: .bold))
                        Text("\(learnedRules.count)")
                            .font(.system(size: 10, weight: .bold, design: .monospaced))
                    }
                    .foregroundStyle(Theme.lime)
                    .padding(.horizontal, 5).padding(.vertical, 1)
                    .background(Theme.lime.opacity(0.13))
                    .clipShape(Capsule())
                }
                Image(systemName: "chevron.down")
                    .font(.system(size: 8, weight: .semibold))
                    .foregroundStyle(Theme.muted)
            }
            .padding(.horizontal, 10).padding(.vertical, 5)
            .background(isActive ? Theme.yellow.opacity(0.08) : Theme.panel)
            .overlay(Capsule().stroke(isActive ? Theme.yellow.opacity(0.45) : Theme.border, lineWidth: 0.8))
            .clipShape(Capsule())
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .help("代理拨号失败自动直连重试；多次成功后自动生成直连规则")
        .popover(isPresented: $showDetail, arrowEdge: .bottom) {
            FallbackLearningPopover(onEditRules: {
                showDetail = false
                showRoutingEditor = true
            })
            .environmentObject(model)
        }
        .sheet(isPresented: $showRoutingEditor) {
            RoutingRulesEditor(rules: model.status?.routing ?? [])
                .environmentObject(model)
        }
    }
}

private struct FallbackLearningPopover: View {
    @EnvironmentObject private var model: AppModel
    let onEditRules: () -> Void

    private var candidates: [FallbackCandidate] { model.stats?.fallback?.candidates ?? [] }
    private var learnedRules: [RoutingRule] { model.stats?.fallback?.learned ?? [] }
    private var threshold: Int { model.stats?.fallback?.threshold ?? 3 }
    private var windowHours: Int { model.stats?.fallback?.windowHours ?? 24 }
    private var isProxyEgress: Bool { model.status?.egress == "proxy" }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 10) {
                ZStack {
                    RoundedRectangle(cornerRadius: 7).fill(Theme.yellow.opacity(0.14))
                    Image(systemName: "wand.and.stars")
                        .font(.system(size: 14, weight: .semibold)).foregroundStyle(Theme.yellow)
                }
                .frame(width: 30, height: 30)
                VStack(alignment: .leading, spacing: 1) {
                    Text("智能回退学习").font(.system(size: 13, weight: .semibold))
                    Text("被代理误伤的国内域名，自动学回直连")
                        .font(.system(size: 10)).foregroundStyle(Theme.muted)
                }
                Spacer()
            }
            .padding(14)

            HStack(spacing: 0) {
                FlowStep(icon: "bolt.slash", text: "代理失败", color: Theme.coral)
                FlowArrow()
                FlowStep(icon: "arrow.uturn.down", text: "直连重试", color: Theme.cyan)
                FlowArrow()
                FlowStep(icon: "checkmark.circle", text: "\(windowHours)h 内 ×\(threshold)", color: Theme.yellow)
                FlowArrow()
                FlowStep(icon: "arrow.triangle.branch", text: "直连规则", color: Theme.lime)
            }
            .padding(.horizontal, 14).padding(.bottom, 12)

            Divider().overlay(Theme.border)

            VStack(alignment: .leading, spacing: 12) {
                if !isProxyEgress && candidates.isEmpty && learnedRules.isEmpty {
                    Label("当前为直连出口，切换到代理出口后开始工作", systemImage: "moon.zzz")
                        .font(.caption).foregroundStyle(Theme.muted)
                } else if candidates.isEmpty && learnedRules.isEmpty {
                    Label("正在守望：还没有域名触发回退", systemImage: "eye")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                if !candidates.isEmpty {
                    VStack(alignment: .leading, spacing: 7) {
                        Text("学习中 · \(candidates.count)").eyebrow()
                        ForEach(candidates.prefix(6)) { item in
                            HStack(spacing: 8) {
                                Text(item.host)
                                    .font(.system(size: 11, design: .monospaced)).lineLimit(1)
                                    .layoutPriority(1)
                                Spacer(minLength: 8)
                                GeometryReader { geo in
                                    ZStack(alignment: .leading) {
                                        Capsule().fill(Theme.panelRaised)
                                        Capsule().fill(Theme.yellow)
                                            .frame(width: geo.size.width * CGFloat(min(item.count, threshold)) / CGFloat(threshold))
                                    }
                                }
                                .frame(width: 54, height: 4)
                                Text("\(item.count)/\(threshold)")
                                    .font(.system(size: 10, weight: .bold, design: .monospaced))
                                    .foregroundStyle(Theme.yellow)
                                    .frame(width: 26, alignment: .trailing)
                            }
                            .help("最近回退成功：\(relativeTime(item.lastAt))")
                        }
                        if candidates.count > 6 {
                            Text("还有 \(candidates.count - 6) 个候选…").font(.caption2).foregroundStyle(Theme.muted)
                        }
                    }
                }
                if !learnedRules.isEmpty {
                    VStack(alignment: .leading, spacing: 7) {
                        Text("已生成直连规则 · \(learnedRules.count)").eyebrow()
                        ForEach(learnedRules.prefix(6)) { rule in
                            HStack(spacing: 8) {
                                Image(systemName: "checkmark.circle.fill")
                                    .font(.system(size: 10)).foregroundStyle(Theme.lime)
                                Text(rule.value)
                                    .font(.system(size: 11, design: .monospaced)).lineLimit(1)
                                Spacer(minLength: 0)
                            }
                        }
                        if learnedRules.count > 6 {
                            Text("还有 \(learnedRules.count - 6) 条…").font(.caption2).foregroundStyle(Theme.muted)
                        }
                    }
                }
            }
            .padding(14)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.canvas)

            Divider().overlay(Theme.border)
            HStack {
                Text("学习到的规则可在规则编辑器中随时删除")
                    .font(.system(size: 10)).foregroundStyle(Theme.muted)
                Spacer()
                Button(action: onEditRules) {
                    Label("管理规则", systemImage: "slider.horizontal.3").font(.caption)
                }
                .buttonStyle(.bordered).controlSize(.small)
            }
            .padding(.horizontal, 14).padding(.vertical, 10)
        }
        .frame(width: 330)
    }
}

private struct FlowStep: View {
    let icon: String
    let text: String
    let color: Color

    var body: some View {
        HStack(spacing: 4) {
            Image(systemName: icon).font(.system(size: 9, weight: .semibold)).foregroundStyle(color)
            Text(text).font(.system(size: 9, weight: .medium)).foregroundStyle(Color.primary.opacity(0.75))
        }
        .padding(.horizontal, 7).padding(.vertical, 4)
        .background(color.opacity(0.09))
        .overlay(Capsule().stroke(color.opacity(0.28), lineWidth: 0.7))
        .clipShape(Capsule())
        .fixedSize()
    }
}

private struct FlowArrow: View {
    var body: some View {
        Image(systemName: "chevron.compact.right")
            .font(.system(size: 9, weight: .semibold))
            .foregroundStyle(Theme.muted)
            .frame(maxWidth: .infinity)
    }
}

private func relativeTime(_ date: Date) -> String {
    let formatter = RelativeDateTimeFormatter()
    formatter.locale = Locale(identifier: "zh_CN")
    formatter.unitsStyle = .short
    return formatter.localizedString(for: date, relativeTo: Date())
}

private struct GatewaySummary: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            HStack(spacing: 20) {
                HStack(spacing: 12) {
                    ZStack {
                        Circle().fill((model.isRunning ? Theme.lime : Theme.coral).opacity(0.14))
                        Image(systemName: model.isRunning ? "checkmark.shield.fill" : "exclamationmark.shield.fill")
                            .font(.system(size: 24)).foregroundStyle(model.isRunning ? Theme.lime : Theme.coral)
                    }
                    .frame(width: 48, height: 48)
                    VStack(alignment: .leading, spacing: 3) {
                        Text(model.isRunning ? "旁路由正在工作" : "旁路由未启动")
                            .font(.system(size: 15, weight: .semibold))
                        Text(model.isRunning ? "局域网设备可以使用当前网关" : "启动后才会接管局域网设备流量")
                            .font(.caption).foregroundStyle(Theme.muted)
                    }
                }
                Spacer(minLength: 12)
                SummaryFact("网关地址", model.status?.gateway.localIP.nonEmpty ?? "--")
                SummaryFact("网络接口", model.status?.gateway.interface.nonEmpty ?? "--")
                SummaryFact("出口", model.status?.egress == "proxy" ? "代理" : "直连")
                SummaryFact("DNS", model.status?.dns.enabled == true ? "已启用" : "未启用")
                ExitIdentityFact(identity: model.stats?.health.egressIdentity)
                SummaryFact("运行时间", uptime(model.stats?.uptimeSec ?? 0))
            }
        }
    }
}

private struct ExitIdentityFact: View {
    let identity: EgressIdentity?
    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text("公网出口").font(.caption2).foregroundStyle(Theme.muted)
            Text(identity?.ip ?? "正在检测")
                .font(.system(size: 12, weight: .medium, design: .monospaced)).lineLimit(1)
            Text(egressLocation(identity)).font(.caption2).foregroundStyle(Theme.muted).lineLimit(1)
        }
        .frame(minWidth: 112, alignment: .leading)
    }
}

private struct SummaryFact: View {
    let label: String
    let value: String
    init(_ label: String, _ value: String) { self.label = label; self.value = value }
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label).font(.caption2).foregroundStyle(Theme.muted)
            Text(value).font(.system(size: 12, weight: .medium, design: .monospaced)).lineLimit(1)
        }
    }
}

private struct CoreHero: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 17) {
                HStack {
                    Text("核心状态").sectionLabel()
                    Spacer()
                    LiveBadge(active: model.isRunning)
                }
                Spacer()
                ZStack {
                    Circle().stroke(Theme.border, lineWidth: 10)
                    Circle().trim(from: 0, to: model.isRunning ? 0.92 : 0.08)
                        .stroke(model.isRunning ? Theme.lime : Theme.coral, style: StrokeStyle(lineWidth: 10, lineCap: .round))
                        .rotationEffect(.degrees(-90))
                    Image(systemName: model.isRunning ? "network.badge.shield.half.filled" : "network.slash")
                        .font(.system(size: 38, weight: .medium)).foregroundStyle(model.isRunning ? Theme.cyan : Theme.muted)
                }
                .frame(width: 112, height: 112).frame(maxWidth: .infinity)
                Spacer()
                HStack {
                    ValuePair(label: "出口", value: model.status?.egress == "proxy" ? "PROXY" : "DIRECT")
                    Spacer()
                    ValuePair(label: "接口", value: model.status?.gateway.interface.nonEmpty ?? "--")
                    Spacer()
                    ValuePair(label: "运行", value: uptime(model.stats?.uptimeSec ?? 0))
                }
            }
        }
    }
}

private struct GettingStartedPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var showProxyConfig = false

    var body: some View {
        Panel {
            HStack(spacing: 18) {
                ZStack {
                    RoundedRectangle(cornerRadius: 7).fill(Theme.cyan.opacity(0.14))
                    Image(systemName: "point.3.connected.trianglepath.dotted")
                        .font(.system(size: 25, weight: .semibold)).foregroundStyle(Theme.cyan)
                }
                .frame(width: 54, height: 54)
                VStack(alignment: .leading, spacing: 5) {
                    Text("首次使用").font(.system(size: 15, weight: .semibold))
                    Text("先填写 Clash、Mihomo 或 sing-box 提供的本机代理地址与端口，再启动网关。")
                        .font(.caption).foregroundStyle(Theme.muted).lineLimit(2)
                }
                Spacer(minLength: 12)
                Button {
                    showProxyConfig = true
                } label: {
                    Label("配置代理出口", systemImage: "arrow.right")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                Button {
                    model.initializeAndStart()
                } label: {
                    Label("使用直连启动", systemImage: "play.fill")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.lime))
            }
        }
        .frame(minHeight: 88)
        .sheet(isPresented: $showProxyConfig) {
            ProxyConfigSheet().environmentObject(model)
        }
    }
}

private struct ThroughputChart: View {
    @EnvironmentObject private var model: AppModel
    let compact: Bool

    var body: some View {
        let allPoints = model.stats?.relay.traffic ?? []
        let points = compact ? Array(allPoints.suffix(60)) : allPoints
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("实时吞吐").sectionLabel()
                    Spacer()
                    ChartLegend(color: Theme.cyan, text: "下载")
                    ChartLegend(color: Theme.yellow, text: "上传")
                }
                Chart(points) { point in
                    AreaMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5))
                        .foregroundStyle(LinearGradient(colors: [Theme.cyan.opacity(0.28), Theme.cyan.opacity(0.01)], startPoint: .top, endPoint: .bottom))
                        .interpolationMethod(.linear)
                    LineMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5))
                        .foregroundStyle(Theme.cyan).lineStyle(StrokeStyle(lineWidth: 2)).interpolationMethod(.linear)
                    LineMark(x: .value("时间", point.at), y: .value("上传", Double(point.up) / 5))
                        .foregroundStyle(Theme.yellow).lineStyle(StrokeStyle(lineWidth: 1.5)).interpolationMethod(.linear)
                }
                .chartXAxis(.automatic)
                .chartYAxis {
                    AxisMarks(position: .leading) { value in
                        AxisGridLine().foregroundStyle(Theme.border)
                        AxisValueLabel { if let bytes = value.as(Double.self) { Text(shortBytes(Int64(bytes)) + "/s") } }
                    }
                }
                .chartPlotStyle { $0.background(Theme.canvas.opacity(0.28)) }
            }
        }
    }
}

private struct ServiceUsagePanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var selectedDevice = "全部设备"

    var body: some View {
        let deviceGroups = model.stats?.relay.deviceServices ?? []
        let filtered = selectedDevice == "全部设备"
            ? (model.stats?.relay.services ?? [])
            : (deviceGroups.first { $0.device == selectedDevice }?.services ?? [])
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Text("服务流量").sectionLabel()
                    Spacer()
                    Text("\(filtered.count) 个服务 · \(shortBytes(filtered.reduce(0) { $0 + $1.total }))")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Picker("统计范围", selection: $selectedDevice) {
                    Text("全部设备").tag("全部设备")
                    ForEach(deviceGroups) { group in
                        Text(model.deviceLabel(for: group.device).nonEmpty.map { "\($0) · \(group.device)" } ?? group.device)
                            .tag(group.device)
                    }
                }
                .labelsHidden()
                if filtered.isEmpty {
                    EmptyTelemetry(icon: "square.stack.3d.up", text: "等待服务流量")
                } else {
                    let maximum = max(filtered.first?.total ?? 1, 1)
                    ScrollView(.vertical, showsIndicators: false) {
                        VStack(spacing: 10) {
                            ForEach(filtered) { item in
                                VStack(spacing: 5) {
                                    HStack {
                                        Text(item.displayName).font(.system(size: 12, weight: .medium)).lineLimit(1)
                                        Spacer()
                                        Text("\(item.connections) 次 · \(shortBytes(item.total))")
                                            .font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                                    }
                                    GeometryReader { geometry in
                                        Capsule().fill(Theme.cyan.opacity(0.7))
                                            .frame(width: max(3, geometry.size.width * CGFloat(Double(item.total) / Double(maximum))))
                                    }.frame(height: 4)
                                }
                            }
                        }
                    }
                }
            }
        }
        .frame(maxHeight: .infinity)
        .onChange(of: deviceGroups.map(\.device)) { devices in
            if selectedDevice != "全部设备", !devices.contains(selectedDevice) { selectedDevice = "全部设备" }
        }
    }
}

private struct DevicesView: View {
    @EnvironmentObject private var model: AppModel
    @State private var showOnboarding = false

    var body: some View {
        ScrollPage {
            DeviceAccessSummary { showOnboarding = true }
            LabeledDevicesStrip()
            HStack(alignment: .top, spacing: 16) {
                DeviceRanking().frame(maxWidth: .infinity)
                ServiceUsagePanel()
                    .frame(minWidth: 280, idealWidth: 360, maxWidth: 380)
                    .frame(height: 460)
            }
        }
        .sheet(isPresented: $showOnboarding) {
            DeviceOnboardingSheet().environmentObject(model)
        }
    }
}

private struct LabeledDevicesStrip: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let labels = model.deviceLabels.sorted { $0.key < $1.key }
        if !labels.isEmpty {
            HStack(spacing: 10) {
                Text("已备注设备").sectionLabel()
                ForEach(labels, id: \.key) { ip, label in
                    HStack(spacing: 7) {
                        Image(systemName: "tag.fill").foregroundStyle(Theme.cyan)
                        Text(label).font(.caption.weight(.semibold))
                        Text(ip).font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                        Button { model.setDeviceLabel("", for: ip) } label: { Image(systemName: "xmark") }
                            .buttonStyle(.plain).foregroundStyle(Theme.muted).help("移除备注")
                    }
                    .padding(.horizontal, 10).frame(height: 30)
                    .background(Theme.panel)
                    .overlay(RoundedRectangle(cornerRadius: 6).stroke(Theme.border, lineWidth: 0.7))
                    .clipShape(RoundedRectangle(cornerRadius: 6))
                }
                Spacer()
            }
        }
    }
}

private struct DeviceAccessSummary: View {
    @EnvironmentObject private var model: AppModel
    let onOnboard: () -> Void

    init(onOnboard: @escaping () -> Void) { self.onOnboard = onOnboard }

    var body: some View {
        Panel {
            HStack(spacing: 20) {
                ZStack {
                    RoundedRectangle(cornerRadius: 7).fill(Theme.lime.opacity(0.12))
                    Image(systemName: "desktopcomputer.and.macbook")
                        .font(.system(size: 22, weight: .medium)).foregroundStyle(Theme.lime)
                }
                .frame(width: 52, height: 52)
                VStack(alignment: .leading, spacing: 4) {
                    Text("\(model.stats?.relay.devices.count ?? 0) 台设备已接入")
                        .font(.system(size: 16, weight: .semibold))
                        .lineLimit(1).fixedSize(horizontal: true, vertical: false)
                    Text("其中 \(model.activeDeviceCount) 台正在产生连接")
                        .font(.caption).foregroundStyle(Theme.muted)
                        .lineLimit(1).fixedSize(horizontal: true, vertical: false)
                }
                Spacer()
                CompactSetupValue(label: "活动连接", value: "\(model.stats?.relay.active.count ?? 0)")
                CompactSetupValue(label: "已识别服务", value: "\(model.stats?.relay.services.count ?? 0)")
                CompactSetupValue(label: "网关与 DNS", value: model.status?.gateway.localIP.nonEmpty ?? "--", copyable: true)
                CompactSetupValue(label: "子网掩码", value: "255.255.255.0", copyable: true)
                Button(action: onOnboard) {
                    Label("接入设备", systemImage: "plus.circle.fill")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                .help("查看接入教程与推荐静态 IP")
            }
        }
    }
}

private struct DeviceOnboardingSheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var batch = 0
    @State private var candidates: [String] = []
    @State private var probing = false
    @State private var probed = false

    private var pool: [String] {
        suggestedDeviceIPPool(
            gateway: model.status?.gateway.localIP ?? "",
            occupied: Set(model.stats?.relay.devices.map(\.name) ?? [])
        )
    }

    var body: some View {
        let gateway = model.status?.gateway.localIP.nonEmpty ?? "--"
        VStack(spacing: 0) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("接入新设备").font(.title3.weight(.semibold))
                    Text("在设备上把网关和 DNS 指向本机，即可让流量经过旁路由。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                Button { dismiss() } label: { Image(systemName: "xmark") }
                    .buttonStyle(IconButtonStyle())
            }
            .padding(20)
            .background(Theme.panel)
            Divider().overlay(Theme.border)

            VStack(alignment: .leading, spacing: 16) {
                VStack(alignment: .leading, spacing: 0) {
                    OnboardingStep(number: "1", title: "打开设备的网络设置", detail: "Switch / PS5 / Apple TV / 手机的 Wi-Fi 或有线网络里，选择「手动 / 静态 IP」")
                    OnboardingStep(number: "2", title: "填写一个候选 IP", detail: probed ? "以下地址已 ping 探测未被占用，点击即可复制" : "以下地址未在设备列表中出现，点击即可复制")
                    HStack(spacing: 6) {
                        Text(suggestionPrefix(candidates.isEmpty ? pool : candidates))
                            .font(.system(size: 13, weight: .semibold, design: .monospaced))
                        ForEach(candidates, id: \.self) { address in
                            Button {
                                NSPasteboard.general.clearContents()
                                NSPasteboard.general.setString(address, forType: .string)
                            } label: {
                                Text(".\(address.split(separator: ".").last.map(String.init) ?? address)")
                                    .font(.system(size: 13, weight: .semibold, design: .monospaced))
                                    .foregroundStyle(Theme.cyan)
                                    .padding(.horizontal, 10).frame(height: 28)
                                    .background(Theme.cyan.opacity(0.08))
                                    .overlay(RoundedRectangle(cornerRadius: 5).stroke(Theme.cyan.opacity(0.35), lineWidth: 0.8))
                                    .clipShape(RoundedRectangle(cornerRadius: 5))
                            }
                            .buttonStyle(.plain)
                            .help("点击复制 \(address)")
                        }
                        if probing {
                            ProgressView().controlSize(.small)
                            Text("正在探测占用...").font(.caption2).foregroundStyle(Theme.muted)
                        } else if candidates.isEmpty {
                            Text("本批候选均被占用，请换一批").font(.caption).foregroundStyle(Theme.yellow)
                        }
                        Spacer(minLength: 8)
                        Button {
                            batch += 1
                            refreshCandidates()
                        } label: {
                            Label("换一批", systemImage: "arrow.triangle.2.circlepath").font(.caption)
                        }
                        .buttonStyle(.bordered).controlSize(.small)
                        .disabled(probing || pool.count <= 5)
                    }
                    .padding(.leading, 34).padding(.bottom, 14)
                    OnboardingStep(number: "3", title: "网关和 DNS 都填写本机地址", detail: "网关指向旁路由流量才会经过它；DNS 也指向旁路由才能识别域名、按域名分流。主路由无需任何改动")
                    HStack(spacing: 10) {
                        SetupValue("网关 / DNS", gateway)
                        SetupValue("子网掩码", "255.255.255.0")
                    }
                    .padding(.leading, 34).padding(.bottom, 14)
                    OnboardingStep(number: "4", title: "保存并测试网络", detail: "设备上的代理设置保持关闭或不填写；连通后会自动出现在设备列表中")
                }
                Label("候选地址请确认不在路由器 DHCP 分配范围内，避免地址冲突。", systemImage: "exclamationmark.triangle")
                    .font(.caption).foregroundStyle(Theme.yellow)
            }
            .padding(20)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)

            Divider().overlay(Theme.border)
            HStack {
                Spacer()
                Button("完成") { dismiss() }.buttonStyle(ActionButtonStyle(tint: Theme.cyan))
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
            .background(Theme.panel)
        }
        .frame(width: 580, height: 500)
        .background(Theme.canvas)
        .onAppear { refreshCandidates() }
    }

    private func refreshCandidates() {
        let all = pool
        guard !all.isEmpty else {
            candidates = []
            return
        }
        let batchCount = (all.count + 4) / 5
        let start = (batch % batchCount) * 5
        let slice = Array(all.dropFirst(start).prefix(5))
        candidates = slice
        probed = false
        probing = true
        Task {
            let occupied = await probeOccupiedAddresses(slice)
            candidates = slice.filter { !occupied.contains($0) }
            probing = false
            probed = true
        }
    }
}

private struct OnboardingStep: View {
    let number: String
    let title: String
    let detail: String

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Text(number).font(.caption.weight(.bold)).foregroundStyle(Color.white)
                .frame(width: 22, height: 22).background(Theme.cyan).clipShape(Circle())
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.system(size: 13, weight: .semibold))
                if !detail.isEmpty {
                    Text(detail).font(.caption).foregroundStyle(Theme.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            Spacer(minLength: 0)
        }
        .padding(.bottom, 12)
    }
}

private struct DeviceRanking: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let activeIPs = Set(model.stats?.relay.active.map(\.srcIP) ?? [])
        let devices = Array((model.stats?.relay.devices ?? []).prefix(20))
        Panel {
            VStack(alignment: .leading, spacing: 0) {
                HStack {
                    VStack(alignment: .leading, spacing: 3) {
                        Text("已接入设备").sectionLabel()
                        Text("按本次核心运行期间的流量排序").font(.caption).foregroundStyle(Theme.muted)
                    }
                    Spacer()
                }
                .padding(.bottom, 14)
                if devices.isEmpty {
                    EmptyTelemetry(icon: "desktopcomputer", text: "等待局域网设备接入")
                        .frame(minHeight: 240)
                } else {
                    let maximum = max(devices.first?.total ?? 1, 1)
                    HStack(spacing: 12) {
                        Text("设备地址").frame(maxWidth: .infinity, alignment: .leading)
                        Text("标签备注").frame(width: 130, alignment: .leading)
                        Text("状态").frame(width: 64, alignment: .leading)
                        Text("连接").frame(width: 64, alignment: .trailing)
                        Text("流量").frame(width: 84, alignment: .trailing)
                    }
                    .font(.caption2.weight(.medium)).foregroundStyle(Theme.muted)
                    .padding(.horizontal, 10).padding(.vertical, 8)
                    .background(Theme.panelRaised)
                    VStack(spacing: 0) {
                        ForEach(devices) { item in
                            VStack(spacing: 8) {
                                HStack(spacing: 12) {
                                    HStack(spacing: 9) {
                                        Image(systemName: "desktopcomputer")
                                            .foregroundStyle(activeIPs.contains(item.name) ? Theme.lime : Theme.muted)
                                        Text(item.name).font(.system(size: 13, weight: .medium, design: .monospaced))
                                            .lineLimit(1).fixedSize(horizontal: true, vertical: false)
                                    }
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                    TextField("例如：客厅 Switch", text: Binding(
                                        get: { model.deviceLabel(for: item.name) },
                                        set: { model.setDeviceLabel($0, for: item.name) }
                                    ))
                                    .textFieldStyle(.plain)
                                    .font(.caption)
                                    .frame(width: 130)
                                    Text(activeIPs.contains(item.name) ? "正在使用" : "最近使用")
                                        .font(.caption).foregroundStyle(activeIPs.contains(item.name) ? Theme.lime : Theme.muted)
                                        .frame(width: 64, alignment: .leading)
                                    Text("\(item.connections)").font(.system(.caption, design: .monospaced))
                                        .frame(width: 64, alignment: .trailing)
                                    Text(shortBytes(item.total)).font(.system(.caption, design: .monospaced))
                                        .frame(width: 84, alignment: .trailing)
                                }
                                GeometryReader { geometry in
                                    Capsule().fill(Theme.cyan.opacity(0.65))
                                        .frame(width: max(3, geometry.size.width * CGFloat(Double(item.total) / Double(maximum))))
                                }.frame(height: 3)
                            }
                            .padding(.horizontal, 10).padding(.vertical, 10)
                            if item.id != devices.last?.id { Divider().overlay(Theme.border.opacity(0.7)) }
                        }
                    }
                }
            }
        }
    }
}

private struct CompactSetupValue: View {
    let label: String
    let value: String
    var copyable = false
    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(label).font(.caption2).foregroundStyle(Theme.muted)
            HStack(spacing: 6) {
                Text(value).font(.system(size: 12, weight: .medium, design: .monospaced)).lineLimit(1)
                if copyable {
                    Button {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(value, forType: .string)
                    } label: { Image(systemName: "doc.on.doc") }
                    .buttonStyle(.plain).foregroundStyle(Theme.cyan).help("复制")
                }
            }
        }
        .frame(minWidth: 96, alignment: .leading)
    }
}

private struct StabilityChart: View {
    @EnvironmentObject private var model: AppModel
    var compact = false

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack { Text("出口延迟").sectionLabel(); Spacer(); LiveBadge(active: model.stats?.health.healthy == true) }
                Chart(Array((model.stats?.health.history ?? []).suffix(compact ? 60 : 120))) { point in
                    LineMark(x: .value("时间", point.at), y: .value("延迟", point.latencyMS))
                        .foregroundStyle(Theme.cyan).interpolationMethod(.catmullRom)
                    if !point.ok {
                        PointMark(x: .value("时间", point.at), y: .value("延迟", point.latencyMS))
                            .foregroundStyle(Theme.coral).symbolSize(65)
                    }
                }
                .chartXAxis {
                    AxisMarks(values: .stride(by: .minute, count: compact ? 3 : 5)) { _ in
                        AxisGridLine().foregroundStyle(Theme.border)
                        AxisValueLabel(format: .dateTime.hour().minute())
                    }
                }
                .chartYAxis {
                    AxisMarks(position: .leading) { value in
                        AxisGridLine().foregroundStyle(Theme.border)
                        AxisValueLabel { if let ms = value.as(Double.self) { Text("\(Int(ms)) ms") } }
                    }
                }
                .frame(minHeight: compact ? 110 : 0)
                .frame(maxHeight: compact ? .infinity : nil)
            }
        }
    }
}

private struct StabilitySummary: View {
    @EnvironmentObject private var model: AppModel
    @State private var showProxyConfig = false
    @State private var hovering = false

    var body: some View {
        Button { showProxyConfig = true } label: {
            Panel {
                VStack(alignment: .leading, spacing: 15) {
                    HStack {
                        Text("网络质量").sectionLabel()
                        Spacer()
                        Image(systemName: "slider.horizontal.3")
                            .font(.system(size: 11)).foregroundStyle(hovering ? Theme.cyan : Theme.muted)
                    }
                    HStack(alignment: .firstTextBaseline) {
                        Text(String(format: "%.1f%%", model.stats?.health.availability ?? 0))
                            .font(.system(size: 26, weight: .semibold, design: .rounded))
                        Text("可用率").font(.caption).foregroundStyle(Theme.muted)
                        Spacer()
                        LiveBadge(active: model.stats?.health.healthy == true)
                    }
                    ProgressView(value: min((model.stats?.health.availability ?? 0) / 100, 1))
                        .tint(Theme.lime)
                    QualityRow("平均延迟", formatMS(model.stats?.health.latencyMS), Theme.cyan)
                    QualityRow("平均抖动", formatMS(model.stats?.health.jitterMS), Theme.yellow)
                    Divider().overlay(Theme.border)
                    ProbeHistoryStrip(
                        points: Array((model.stats?.health.history ?? []).suffix(60)),
                        average: model.stats?.health.latencyMS ?? 0
                    )
                    if let error = model.stats?.health.lastError, !error.isEmpty {
                        Text(error).font(.caption).foregroundStyle(Theme.coral).lineLimit(2)
                    }
                }
            }
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(hovering ? Theme.cyan.opacity(0.5) : Color.clear, lineWidth: 1))
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .help("点击配置网络出口")
        .sheet(isPresented: $showProxyConfig) {
            ProxyConfigSheet().environmentObject(model)
        }
    }
}

private struct ProbeHistoryStrip: View {
    let points: [ProbePoint]
    let average: Double

    // Probes fire every 10s; 60 fixed slots cover the last 10 minutes.
    private let slotCount = 60

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("最近 10 分钟探测").font(.caption).foregroundStyle(Theme.muted)
                Spacer()
                Text("每 10 秒 · 实时更新").font(.caption2).foregroundStyle(Theme.muted)
            }
            HStack(spacing: 2) {
                ForEach(0..<slotCount, id: \.self) { slot in
                    let point = point(at: slot)
                    RoundedRectangle(cornerRadius: 1)
                        .fill(point.map(probeColor) ?? Theme.border.opacity(0.55))
                        .frame(maxWidth: .infinity)
                        .frame(height: 26)
                        .help(point.map { $0.ok ? formatMS($0.latencyMS) : "探测失败" } ?? "暂无数据")
                }
            }
            HStack {
                Text("过去"); Spacer(); Text("现在")
            }.font(.system(size: 9, weight: .medium)).foregroundStyle(Theme.muted)
        }
    }

    private func point(at slot: Int) -> ProbePoint? {
        let recent = points.suffix(slotCount)
        let missing = slotCount - recent.count
        guard slot >= missing else { return nil }
        return recent[recent.index(recent.startIndex, offsetBy: slot - missing)]
    }

    private func probeColor(_ point: ProbePoint) -> Color {
        if !point.ok { return Theme.coral }
        if average > 0, point.latencyMS > max(average * 1.8, 80) { return Theme.yellow }
        return Theme.lime
    }
}

private struct ConnectionsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var search = ""
    @State private var outcomeFilter = "全部"
    @State private var deviceFilter = "全部设备"
    @State private var routeFilter = "全部出口"
    @State private var unresolvedOnly = false

    // All records passing search/device/route filters; outcome chips filter on
    // top of this so their counts stay stable while one chip is selected.
    private var baseConnections: [ConnectionInfo] {
        let all = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
        return all.filter { item in
            let label = model.deviceLabel(for: item.srcIP)
            let matchesSearch = search.isEmpty || item.srcIP.localizedCaseInsensitiveContains(search) ||
                label.localizedCaseInsensitiveContains(search) || item.dstHost.localizedCaseInsensitiveContains(search) ||
                item.service.localizedCaseInsensitiveContains(search)
            let matchesDevice = deviceFilter == "全部设备" || item.srcIP == deviceFilter
            let matchesRoute: Bool
            switch routeFilter {
            case "代理": matchesRoute = item.viaProxy && !item.rejected
            case "直连": matchesRoute = !item.viaProxy && !item.rejected
            case "拒绝": matchesRoute = item.rejected
            default: matchesRoute = true
            }
            let matchesResolution = !unresolvedOnly || item.service == "未解析域名"
            return matchesSearch && matchesDevice && matchesRoute && matchesResolution
        }
    }

    private var connections: [ConnectionInfo] {
        baseConnections.filter { matchesOutcome($0) }
    }

    private func outcomeName(_ item: ConnectionInfo) -> String {
        switch item.outcome {
        case .active: return "活跃"
        case .success: return "成功"
        case .noData: return "无数据"
        case .failed: return "失败"
        case .rejected: return "拒绝"
        }
    }

    private func matchesOutcome(_ item: ConnectionInfo) -> Bool {
        switch outcomeFilter {
        case "全部": return true
        case "回退直连": return item.fallback
        default: return outcomeName(item) == outcomeFilter
        }
    }

    private func outcomeCount(_ name: String) -> Int {
        if name == "回退直连" { return baseConnections.filter(\.fallback).count }
        return baseConnections.filter { outcomeName($0) == name }.count
    }

    private var successRateText: String {
        let total = baseConnections.count
        guard total > 0 else { return "--" }
        let good = baseConnections.filter { $0.outcome == .active || $0.outcome == .success }.count
        return "\(good * 100 / total)%"
    }

    private var devices: [String] {
        Array(Set(((model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])).map(\.srcIP))).sorted()
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.muted)
                TextField("搜索设备、服务或域名", text: $search).textFieldStyle(.plain)
                    .frame(minWidth: 180)
                Divider().frame(height: 22)
                Picker("设备", selection: $deviceFilter) {
                    Text("全部设备").tag("全部设备")
                    ForEach(devices, id: \.self) { ip in
                        Text(model.deviceLabel(for: ip).nonEmpty ?? ip).tag(ip)
                    }
                }.labelsHidden().frame(width: 150)
                Picker("出口", selection: $routeFilter) {
                    Text("全部出口").tag("全部出口"); Text("代理").tag("代理"); Text("直连").tag("直连"); Text("拒绝").tag("拒绝")
                }.labelsHidden().frame(width: 120)
                Toggle("仅未解析域名", isOn: $unresolvedOnly).toggleStyle(.checkbox).font(.caption)
                Image(systemName: "info.circle").foregroundStyle(Theme.muted)
                    .help("设备直接连接 IP，或 DNS 映射不可用时无法还原域名。仍会记录目标 IP、端口、流量、时间和出口；HTTPS 加密下无法识别具体操作内容。")
                Spacer(minLength: 8)
                Text("\(connections.count) 条记录").font(.caption).foregroundStyle(Theme.muted)
            }
            .padding(.horizontal, 16).frame(height: 44).background(Theme.panel)
            Divider().overlay(Theme.border)

            VStack(spacing: 12) {
                HStack(spacing: 8) {
                    OutcomeChip(label: "全部", count: baseConnections.count, color: Theme.muted,
                                selected: outcomeFilter == "全部") { outcomeFilter = "全部" }
                    OutcomeChip(label: "活跃", count: outcomeCount("活跃"), color: Theme.cyan,
                                selected: outcomeFilter == "活跃") { outcomeFilter = "活跃" }
                    OutcomeChip(label: "成功", count: outcomeCount("成功"), color: Theme.lime,
                                selected: outcomeFilter == "成功") { outcomeFilter = "成功" }
                    OutcomeChip(label: "无数据", count: outcomeCount("无数据"), color: Theme.muted,
                                selected: outcomeFilter == "无数据") { outcomeFilter = "无数据" }
                    OutcomeChip(label: "失败", count: outcomeCount("失败"), color: Theme.yellow,
                                selected: outcomeFilter == "失败") { outcomeFilter = "失败" }
                    OutcomeChip(label: "拒绝", count: outcomeCount("拒绝"), color: Theme.coral,
                                selected: outcomeFilter == "拒绝") { outcomeFilter = "拒绝" }
                    OutcomeChip(label: "回退直连", count: outcomeCount("回退直连"), color: Theme.yellow,
                                selected: outcomeFilter == "回退直连") { outcomeFilter = "回退直连" }
                    Spacer(minLength: 8)
                    FallbackLearningBadge()
                    HStack(spacing: 5) {
                        Text("连接成功率").font(.caption2).foregroundStyle(Theme.muted)
                        Text(successRateText)
                            .font(.system(size: 13, weight: .bold, design: .monospaced))
                            .foregroundStyle(Theme.lime)
                    }
                    .help("成功率 = (活跃 + 成功) / 当前筛选范围内全部记录；仅统计连接层结果，HTTPS 下看不到应用层状态码")
                }

                Table(connections) {
                    TableColumn("设备") { item in
                        VStack(alignment: .leading, spacing: 1) {
                            if let label = model.deviceLabel(for: item.srcIP).nonEmpty {
                                Text(label).fontWeight(.medium)
                            }
                            Text(item.srcIP).font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                        }
                    }.width(min: 110, ideal: 140)
                    TableColumn("识别服务") { Text($0.service).fontWeight(.medium) }.width(min: 90, ideal: 120)
                    TableColumn("目标域名 / 地址") { item in
                        Text("\(item.dstHost):\(item.dstPort)")
                            .font(.system(.body, design: .monospaced))
                            .help("\(item.dstHost):\(item.dstPort)")
                    }.width(min: 220, ideal: 320)
                    TableColumn("出口") { item in
                        if item.fallback && !item.rejected {
                            Text("回退直连")
                                .foregroundStyle(Theme.yellow)
                                .help("代理拨号失败后自动回退到直连；同一域名 24 小时内回退成功 3 次会生成\"自动学习\"直连规则")
                        } else {
                            Text(item.rejected ? "REJECT" : (item.viaProxy ? "PROXY" : "DIRECT"))
                                .foregroundStyle(item.rejected ? Theme.coral : (item.viaProxy ? Theme.cyan : Theme.yellow))
                        }
                    }.width(72)
                    TableColumn("结果") { item in
                        HStack(spacing: 6) {
                            Circle().fill(outcomeColor(item.outcome)).frame(width: 6, height: 6)
                            Text(item.outcome.label)
                                .foregroundStyle(outcomeColor(item.outcome))
                                .lineLimit(1)
                        }
                        .help(outcomeHelp(item.outcome))
                    }.width(min: 80, ideal: 110)
                    TableColumn("流量") { Text(bytes($0.up + $0.down)) }.width(76)
                }
                .scrollContentBackground(.hidden)
                .background(Theme.panel)
                .overlay(RoundedRectangle(cornerRadius: 7).stroke(Theme.border, lineWidth: 0.7))
                .clipShape(RoundedRectangle(cornerRadius: 7))
                .shadow(color: Color.black.opacity(0.035), radius: 7, y: 2)
            }
            .padding(16)
        }
        .background(Theme.canvas)
    }

    private func outcomeColor(_ outcome: ConnectionOutcome) -> Color {
        switch outcome {
        case .active: return Theme.cyan
        case .success: return Theme.lime
        case .noData: return Theme.muted
        case .failed: return Theme.yellow
        case .rejected: return Theme.coral
        }
    }

    private func outcomeHelp(_ outcome: ConnectionOutcome) -> String {
        switch outcome {
        case .active: return "连接进行中"
        case .success: return "连接建立且有数据传输"
        case .noData: return "连接建立但没有数据传输，可能被远端关闭"
        case .failed(let reason): return "出口拨号失败：\(reason)。HTTPS 加密流量无法看到 404/502 等应用层状态码"
        case .rejected: return "被分流规则拒绝"
        }
    }
}

private struct OutcomeChip: View {
    let label: String
    let count: Int
    let color: Color
    let selected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Circle().fill(color).frame(width: 6, height: 6)
                Text(label).font(.system(size: 11, weight: .medium))
                Text("\(count)")
                    .font(.system(size: 11, weight: .bold, design: .monospaced))
                    .foregroundStyle(color)
            }
            .padding(.horizontal, 10).padding(.vertical, 5)
            .background(selected ? color.opacity(0.14) : Theme.panel)
            .overlay(RoundedRectangle(cornerRadius: 6).stroke(selected ? color.opacity(0.55) : Theme.border, lineWidth: 0.8))
            .clipShape(RoundedRectangle(cornerRadius: 6))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .help("点击筛选\(label == "全部" ? "全部记录" : "「\(label)」的记录")")
    }
}

private struct ProxyConfigSheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var mode = "socks5"
    @State private var isSaving = false
    @State private var isTesting = false
    @State private var saveError: String?
    @State private var testResult: (ok: Bool, message: String)?

    var body: some View {
        VStack(spacing: 0) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("网络出口").font(.title3.weight(.semibold))
                    Text(activeExitSubtitle).font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                LiveBadge(active: model.isRunning && (model.stats?.health.healthy ?? false))
                Button { dismiss() } label: { Image(systemName: "xmark") }
                    .buttonStyle(IconButtonStyle())
            }
            .padding(20)
            .background(Theme.panel)
            Divider().overlay(Theme.border)

            VStack(alignment: .leading, spacing: 18) {
                HStack(alignment: .top, spacing: 24) {
                    VStack(alignment: .leading, spacing: 5) {
                        Text("当前出口").fieldLabel()
                        Text(activeExitTitle)
                            .font(.system(size: 20, weight: .semibold, design: .rounded))
                            .lineLimit(1).minimumScaleFactor(0.72)
                    }
                    .frame(minWidth: 210, alignment: .leading)
                    Spacer(minLength: 0)
                    VStack(alignment: .leading, spacing: 5) {
                        Label("公网 IP", systemImage: "globe.asia.australia")
                            .font(.caption2).foregroundStyle(Theme.muted)
                        Text(model.stats?.health.egressIdentity?.ip ?? "正在检测")
                            .font(.system(size: 15, weight: .semibold, design: .monospaced))
                            .lineLimit(1)
                            .textSelection(.enabled)
                    }
                    VStack(alignment: .leading, spacing: 5) {
                        Label("出口地区", systemImage: "mappin.and.ellipse")
                            .font(.caption2).foregroundStyle(Theme.muted)
                        Text(egressLocation(model.stats?.health.egressIdentity))
                            .font(.system(size: 15, weight: .semibold))
                            .lineLimit(1)
                        if let isp = model.stats?.health.egressIdentity?.isp?.nonEmpty {
                            Text(isp).font(.caption2).foregroundStyle(Theme.muted).lineLimit(1)
                        }
                    }
                }
                Divider().overlay(Theme.border)
                VStack(alignment: .leading, spacing: 12) {
                    VStack(alignment: .leading, spacing: 3) {
                        Text("出口方式").sectionLabel()
                        Text("代理填写 Clash / Mihomo / sing-box 的本机混合端口，保存后立即应用到新连接")
                            .font(.caption).foregroundStyle(Theme.muted)
                    }
                    HStack(alignment: .bottom, spacing: 12) {
                        VStack(alignment: .leading, spacing: 7) {
                            Text("出口模式").fieldLabel()
                            Picker("", selection: $mode) {
                                Text("无代理 · 直连").tag("direct")
                                Text("SOCKS5 代理").tag("socks5")
                                Text("HTTP CONNECT 代理").tag("http")
                            }
                            .labelsHidden()
                            .frame(width: 190, height: 36)
                        }
                        VStack(alignment: .leading, spacing: 7) {
                            Text("代理地址").fieldLabel()
                            TextField("127.0.0.1", text: $model.proxyHost)
                                .textFieldStyle(DarkFieldStyle())
                                .disabled(mode == "direct")
                        }
                        .opacity(mode == "direct" ? 0.4 : 1)
                        VStack(alignment: .leading, spacing: 7) {
                            Text("端口").fieldLabel()
                            TextField("7897", value: $model.proxyPort, format: .number.grouping(.never))
                                .textFieldStyle(DarkFieldStyle())
                                .frame(width: 96)
                                .disabled(mode == "direct")
                        }
                        .opacity(mode == "direct" ? 0.4 : 1)
                        Button {
                            testProxy()
                        } label: {
                            if isTesting {
                                ProgressView().controlSize(.small).frame(width: 60)
                            } else {
                                Label("测试代理", systemImage: "bolt.horizontal")
                            }
                        }
                        .buttonStyle(.bordered)
                        .frame(height: 36)
                        .disabled(mode == "direct" || isTesting || isSaving)
                        .help("按当前填写的地址测试连通性，不会保存配置")
                    }
                    if let testResult {
                        Label(testResult.message, systemImage: testResult.ok ? "checkmark.circle.fill" : "xmark.circle.fill")
                            .font(.caption)
                            .foregroundStyle(testResult.ok ? Theme.lime : Theme.coral)
                    }
                }
            }
            .padding(20)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)

            Divider().overlay(Theme.border)
            HStack(spacing: 8) {
                if let saveError {
                    Label(saveError, systemImage: "exclamationmark.triangle.fill")
                        .font(.caption).foregroundStyle(Theme.coral).lineLimit(2)
                }
                Spacer()
                Button("取消") { dismiss() }.buttonStyle(.bordered).disabled(isSaving)
                Button {
                    apply()
                } label: {
                    if isSaving {
                        ProgressView().controlSize(.small).frame(width: 56)
                    } else {
                        Text(applyTitle)
                    }
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                .disabled(isSaving || isTesting || model.isBusy || applyDisabled)
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
            .background(Theme.panel)
        }
        .frame(width: 640, height: 400)
        .background(Theme.canvas)
        .onAppear {
            mode = model.status?.egress == "proxy" ? model.proxyType : "direct"
        }
    }

    private var applyTitle: String {
        mode == "direct" ? "切换直连" : (model.isConfigured ? "应用代理" : "保存配置")
    }

    private var applyDisabled: Bool {
        mode == "direct" && model.status?.egress != "proxy"
    }

    private func apply() {
        saveError = nil
        isSaving = true
        Task {
            let ok: Bool
            if mode == "direct" {
                ok = await model.useDirectConnectionAsync()
            } else {
                model.proxyType = mode
                ok = await model.applyProxyAsync()
            }
            isSaving = false
            if ok { dismiss() } else { saveError = model.errorMessage ?? "应用失败，请检查配置" }
        }
    }

    private func testProxy() {
        testResult = nil
        isTesting = true
        Task {
            model.proxyType = mode
            testResult = await model.testProxyAsync()
            isTesting = false
        }
    }

    private var activeExitTitle: String {
        model.status?.egress == "proxy"
            ? (model.status?.proxy?.nonEmpty ?? "代理未配置")
            : "DIRECT 直连"
    }

    private var activeExitSubtitle: String {
        guard model.status?.egress == "proxy" else { return "局域网流量不经过上游代理" }
        let count = model.status?.routing?.count ?? 0
        return count == 0 ? "未配置分流规则，所有新建 TCP 连接通过此上游代理" : "按顺序匹配 \(count) 条规则，未命中时使用此上游代理"
    }
}

private struct TopoAnchors: PreferenceKey {
    static var defaultValue: [String: Anchor<CGPoint>] = [:]
    static func reduce(value: inout [String: Anchor<CGPoint>], nextValue: () -> [String: Anchor<CGPoint>]) {
        value.merge(nextValue()) { $1 }
    }
}

private extension View {
    func topoPort(_ id: String, _ edge: UnitPoint) -> some View {
        transformAnchorPreference(key: TopoAnchors.self, value: .unitPoint(edge)) { $0[id] = $1 }
    }
}

private struct RouteDiagram: View {
    @EnvironmentObject private var model: AppModel
    let onEditRules: () -> Void
    let onEditProxy: () -> Void
    @State private var devicesExpanded = false

    var body: some View {
        let rules = model.status?.routing ?? []
        let egressProxy = model.status?.egress == "proxy"
        let allDevices = model.stats?.relay.devices ?? []
        let devices = devicesExpanded ? Array(allDevices.prefix(8)) : Array(allDevices.prefix(3))
        let upstream = egressProxy ? (model.status?.proxy?.nonEmpty ?? "代理未配置") : "未配置 · 等价于直连"
        let activeConns = model.stats?.relay.active ?? []
        let activeIPs = Set(activeConns.map(\.srcIP))
        let flowProxy = activeConns.contains { $0.viaProxy }
        let flowDirect = activeConns.contains { !$0.viaProxy }
        let recentRejectCutoff = Date().addingTimeInterval(-120)
        let flowReject = (model.stats?.relay.recent ?? []).contains { $0.rejected && $0.startedAt > recentRejectCutoff }

        VStack(spacing: 0) {
            TopoStage(portID: "router", icon: "wifi.router", tint: Theme.lime,
                      title: "主路由 · 互联网", detail: model.status?.gateway.router.nonEmpty ?? "光猫/路由器")
                .help("代理和直连的流量最终都经主路由访问互联网；设备只需把网关和 DNS 指向旁路由即可被接管")
            Spacer(minLength: 22)
            HStack(alignment: .top, spacing: 14) {
                TopoOutcome(icon: "cloud.fill", title: "上游代理", detail: upstream,
                            count: ruleCount(rules, "proxy"), color: Theme.cyan,
                            emphasized: egressProxy,
                            badge: egressProxy ? "默认" : "回退直连",
                            badgeColor: egressProxy ? Theme.cyan : Theme.yellow,
                            breathing: true,
                            help: egressProxy ? "点击配置网络出口" : "未配置上游代理，proxy 规则会回退为直连；点击配置",
                            action: onEditProxy)
                    .topoPort("out.proxy", .bottom)
                    .topoPort("out.proxy.t", .top)
                TopoOutcome(icon: "network", title: "本机直连", detail: "DIRECT",
                            count: ruleCount(rules, "direct"), color: Theme.lime,
                            emphasized: !egressProxy,
                            badge: egressProxy ? nil : "默认",
                            help: "点击查看命中直连的规则",
                            ruleList: rules.filter { $0.action == "direct" },
                            onEditRules: onEditRules, action: {})
                    .topoPort("out.direct", .bottom)
                    .topoPort("out.direct.t", .top)
                TopoOutcome(icon: "nosign", title: "拒绝", detail: "REJECT",
                            count: ruleCount(rules, "reject"), color: Theme.coral,
                            emphasized: false,
                            help: "点击查看被拒绝的规则",
                            ruleList: rules.filter { $0.action == "reject" },
                            onEditRules: onEditRules, action: {})
                    .topoPort("out.reject", .bottom)
            }
            Spacer(minLength: 24)
            Button(action: onEditRules) {
                TopoStage(
                    portID: "rules", icon: "arrow.triangle.branch", tint: Theme.yellow,
                    title: "规则判断",
                    detail: rules.isEmpty ? "点击配置分流规则" : "\(rules.count) 条 · 首条命中",
                    clickable: true
                )
            }
            .buttonStyle(.plain)
            .help("点击管理分流规则")
            Spacer(minLength: 24)
            TopoStage(portID: "gw", icon: "server.rack", tint: Theme.cyan,
                      title: "旁路由", detail: model.status?.gateway.localIP.nonEmpty ?? "--")
            Spacer(minLength: 24)
            HStack(spacing: 12) {
                if devices.isEmpty {
                    TopoDeviceChip(icon: "desktopcomputer", title: "等待设备接入", subtitle: "配置静态 IP 后自动出现", active: false)
                        .topoPort("dev.empty", .top)
                } else {
                    ForEach(devices) { device in
                        let label = model.deviceLabels[device.name]?.nonEmpty
                        TopoDeviceChip(
                            icon: deviceIcon(label: label),
                            title: label ?? device.name,
                            subtitle: label != nil ? device.name : "局域网设备",
                            active: activeIPs.contains(device.name)
                        )
                        .topoPort("dev.\(device.name)", .top)
                    }
                    if allDevices.count > 3 {
                        Button {
                            withAnimation(.easeInOut(duration: 0.2)) { devicesExpanded.toggle() }
                        } label: {
                            VStack(spacing: 2) {
                                Image(systemName: devicesExpanded ? "chevron.left" : "chevron.right")
                                    .font(.system(size: 10, weight: .bold))
                                Text(devicesExpanded ? "收起" : "+\(allDevices.count - 3)")
                                    .font(.system(size: 9, weight: .bold, design: .monospaced))
                            }
                            .foregroundStyle(Theme.muted)
                            .frame(width: 34, height: 40)
                            .background(Theme.panelRaised)
                            .clipShape(RoundedRectangle(cornerRadius: 7))
                        }
                        .buttonStyle(.plain)
                        .help(devicesExpanded ? "收起设备" : "展开全部设备")
                    }
                }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .backgroundPreferenceValue(TopoAnchors.self) { anchors in
            GeometryReader { geo in
                TopoLinkLayer(
                    points: anchors.mapValues { geo[$0] },
                    devicePorts: devices.isEmpty ? ["dev.empty"] : devices.map { "dev.\($0.name)" },
                    activeDevicePorts: Set(devices.filter { activeIPs.contains($0.name) }.map { "dev.\($0.name)" }),
                    active: model.isRunning,
                    flowProxy: flowProxy,
                    flowDirect: flowDirect,
                    flowReject: flowReject,
                    egressProxy: egressProxy
                )
            }
        }
    }

    private func ruleCount(_ rules: [RoutingRule], _ action: String) -> Int {
        rules.filter { $0.action == action }.count
    }
}

private func deviceIcon(label: String?) -> String {
    guard let label = label?.lowercased() else { return "desktopcomputer" }
    let phones = ["iphone", "手机", "phone", "一加", "oneplus", "xiaomi", "小米", "huawei", "华为", "oppo", "vivo", "pixel", "安卓", "android"]
    let consoles = ["switch", "ps5", "ps4", "xbox", "游戏", "主机"]
    let tvs = ["tv", "电视", "盒子"]
    let pads = ["ipad", "pad", "平板"]
    let laptops = ["mac", "笔记本", "电脑", "pc", "laptop"]
    if phones.contains(where: { label.contains($0) }) { return "iphone" }
    if consoles.contains(where: { label.contains($0) }) { return "gamecontroller.fill" }
    if tvs.contains(where: { label.contains($0) }) { return "appletv.fill" }
    if pads.contains(where: { label.contains($0) }) { return "ipad.landscape" }
    if laptops.contains(where: { label.contains($0) }) { return "laptopcomputer" }
    return "desktopcomputer"
}

private struct TopoDeviceChip: View {
    let icon: String
    let title: String
    let subtitle: String
    let active: Bool

    var body: some View {
        HStack(spacing: 8) {
            ZStack {
                RoundedRectangle(cornerRadius: 6).fill((active ? Theme.lime : Theme.muted).opacity(0.12))
                Image(systemName: icon)
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(active ? Theme.lime : Theme.muted)
            }
            .frame(width: 24, height: 24)
            VStack(alignment: .leading, spacing: 1) {
                Text(title)
                    .font(.system(size: 11, weight: .semibold, design: title.contains(".") ? .monospaced : .default))
                    .lineLimit(1)
                Text(subtitle)
                    .font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted).lineLimit(1)
            }
        }
        .padding(.horizontal, 10).frame(height: 40)
        .background(Theme.panel)
        .overlay(RoundedRectangle(cornerRadius: 7).stroke(active ? Theme.lime.opacity(0.35) : Theme.border, lineWidth: 0.7))
        .clipShape(RoundedRectangle(cornerRadius: 7))
    }
}

private struct TopoStage: View {
    let portID: String
    let icon: String
    let tint: Color
    let title: String
    let detail: String
    var clickable = false
    @State private var hovering = false

    var body: some View {
        HStack(spacing: 11) {
            ZStack {
                RoundedRectangle(cornerRadius: 10).fill(tint.opacity(hovering && clickable ? 0.17 : 0.10))
                Image(systemName: icon).font(.system(size: 16, weight: .semibold)).foregroundStyle(tint)
            }
            .frame(width: 42, height: 42)
            .overlay(RoundedRectangle(cornerRadius: 10).stroke(hovering && clickable ? tint.opacity(0.6) : Theme.border, lineWidth: 1))
            .overlay(alignment: .bottomTrailing) {
                if clickable {
                    Image(systemName: "pencil")
                        .font(.system(size: 7, weight: .bold)).foregroundStyle(Color.white)
                        .frame(width: 14, height: 14)
                        .background(tint).clipShape(Circle())
                        .offset(x: 4, y: 4)
                }
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.system(size: 12, weight: .semibold))
                Text(detail)
                    .font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                    .lineLimit(1).minimumScaleFactor(0.8)
            }
        }
        .padding(.horizontal, 13).frame(height: 58)
        .background(Theme.panel)
        .overlay(RoundedRectangle(cornerRadius: 9).stroke(hovering && clickable ? tint.opacity(0.5) : Theme.border, lineWidth: 0.8))
        .clipShape(RoundedRectangle(cornerRadius: 9))
        .topoPort("\(portID).t", .top)
        .topoPort("\(portID).b", .bottom)
        .onHover { hovering = $0 }
    }
}

private struct TopoOutcome: View {
    let icon: String
    let title: String
    let detail: String
    let count: Int
    let color: Color
    let emphasized: Bool
    var badge: String? = nil
    var badgeColor: Color? = nil
    var breathing = false
    var help = "点击配置网络出口"
    var ruleList: [RoutingRule]? = nil
    var onEditRules: (() -> Void)? = nil
    let action: () -> Void
    @State private var hovering = false
    @State private var showRules = false
    @State private var pulsing = false

    var body: some View {
        Button {
            if ruleList != nil {
                showRules.toggle()
            } else {
                action()
            }
        } label: {
            VStack(spacing: 3) {
                ZStack {
                    RoundedRectangle(cornerRadius: 6).fill(color.opacity(0.12))
                    Image(systemName: icon).font(.system(size: 11, weight: .semibold)).foregroundStyle(color)
                }
                .frame(width: 24, height: 24)
                HStack(spacing: 4) {
                    Text(title).font(.system(size: 11, weight: .semibold))
                    if let badge {
                        Text(badge)
                            .font(.system(size: 8, weight: .bold)).foregroundStyle(badgeColor ?? color)
                            .padding(.horizontal, 4).padding(.vertical, 1)
                            .background((badgeColor ?? color).opacity(0.12)).clipShape(Capsule())
                    }
                }
                Text(detail)
                    .font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted)
                    .lineLimit(1).minimumScaleFactor(0.75)
            }
            .padding(.horizontal, 9).padding(.vertical, 8)
            .frame(width: 148)
            .overlay(alignment: .topTrailing) {
                Text("\(count)")
                    .font(.system(size: 9, weight: .bold, design: .monospaced))
                    .foregroundStyle(count > 0 ? Color.white : Theme.muted)
                    .frame(minWidth: 16, minHeight: 16)
                    .background(count > 0 ? color : Theme.panelRaised)
                    .clipShape(Circle())
                    .offset(x: -5, y: 5)
                    .help("\(count) 条规则指向此出口")
            }
            .background(Theme.panel)
            .overlay(
                RoundedRectangle(cornerRadius: 8)
                    .stroke(strokeColor, lineWidth: emphasized ? 1.2 : 0.8)
            )
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .shadow(color: breathing ? color.opacity(pulsing ? 0.30 : 0.05) : (emphasized ? color.opacity(0.10) : .clear),
                    radius: breathing ? (pulsing ? 9 : 3) : 6, y: 1)
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .help(help)
        .onAppear {
            guard breathing else { return }
            withAnimation(.easeInOut(duration: 1.5).repeatForever(autoreverses: true)) {
                pulsing = true
            }
        }
        .popover(isPresented: $showRules, arrowEdge: .bottom) {
            RuleListPopover(title: title, color: color, rules: ruleList ?? []) {
                showRules = false
                onEditRules?()
            }
        }
    }

    private var strokeColor: Color {
        if hovering { return color.opacity(0.65) }
        if breathing { return color.opacity(pulsing ? 0.65 : 0.30) }
        if emphasized { return color.opacity(0.45) }
        return Theme.border
    }
}

private struct RuleListPopover: View {
    let title: String
    let color: Color
    let rules: [RoutingRule]
    let onEdit: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Circle().fill(color).frame(width: 7, height: 7)
                Text("命中「\(title)」的规则").font(.system(size: 12, weight: .semibold))
                Spacer()
                Text("\(rules.count) 条").font(.caption2).foregroundStyle(Theme.muted)
            }
            if rules.isEmpty {
                Text("暂无规则指向此出口，未命中规则的流量走默认出口。")
                    .font(.caption).foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
            } else {
                ScrollView(.vertical, showsIndicators: false) {
                    VStack(alignment: .leading, spacing: 6) {
                        ForEach(rules) { rule in
                            HStack(spacing: 8) {
                                Text(ruleTypeName(rule.type))
                                    .font(.system(size: 9, weight: .bold))
                                    .foregroundStyle(Theme.muted)
                                    .padding(.horizontal, 5).padding(.vertical, 2)
                                    .background(Theme.panelRaised)
                                    .clipShape(RoundedRectangle(cornerRadius: 4))
                                Text(rule.value)
                                    .font(.system(size: 11, design: .monospaced))
                                    .lineLimit(1)
                                if rule.learned {
                                    Text("自动学习")
                                        .font(.system(size: 8, weight: .bold))
                                        .foregroundStyle(Theme.yellow)
                                        .padding(.horizontal, 4).padding(.vertical, 1)
                                        .background(Theme.yellow.opacity(0.12))
                                        .clipShape(RoundedRectangle(cornerRadius: 3))
                                }
                                Spacer(minLength: 0)
                            }
                        }
                    }
                }
                .frame(maxHeight: 180)
            }
            Divider().overlay(Theme.border)
            Button(action: onEdit) {
                Label("编辑分流规则", systemImage: "slider.horizontal.3").font(.caption)
            }
            .buttonStyle(.bordered)
            .controlSize(.small)
        }
        .padding(14)
        .frame(width: 280)
    }

    private func ruleTypeName(_ type: String) -> String {
        switch type {
        case "domain": return "完整域名"
        case "domain-suffix": return "域名后缀"
        case "ip-cidr": return "IP-CIDR"
        default: return type
        }
    }
}

private struct TopoLinkLayer: View {
    let points: [String: CGPoint]
    let devicePorts: [String]
    let activeDevicePorts: Set<String>
    let active: Bool
    let flowProxy: Bool
    let flowDirect: Bool
    let flowReject: Bool
    let egressProxy: Bool
    @State private var isScrolling = false

    var body: some View {
        TimelineView(.animation(minimumInterval: 1.0 / 24.0, paused: !active || isScrolling)) { timeline in
            Canvas { ctx, _ in
                let t = timeline.date.timeIntervalSinceReferenceDate
                for (index, port) in devicePorts.enumerated() {
                    let deviceActive = activeDevicePorts.contains(port)
                    link(ctx, from: port, to: "gw.b", color: Theme.lime, phase: t,
                         offset: Double(index) * 0.37, strong: deviceActive, flow: deviceActive)
                }
                let anyFlow = flowProxy || flowDirect
                link(ctx, from: "gw.t", to: "rules.b", color: Theme.cyan, phase: t, offset: 0.15,
                     strong: anyFlow, flow: anyFlow)
                link(ctx, from: "out.proxy.t", to: "router.b", color: Theme.cyan, phase: t, offset: 0.25,
                     strong: egressProxy, flow: flowProxy)
                link(ctx, from: "out.direct.t", to: "router.b", color: Theme.lime, phase: t, offset: 0.55,
                     strong: !egressProxy, flow: flowDirect)
                link(ctx, from: "rules.t", to: "out.proxy", color: Theme.cyan, phase: t, offset: 0.4,
                     strong: egressProxy, flow: flowProxy)
                link(ctx, from: "rules.t", to: "out.direct", color: Theme.lime, phase: t, offset: 0.7,
                     strong: !egressProxy, flow: flowDirect)
                link(ctx, from: "rules.t", to: "out.reject", color: Theme.coral, phase: t, offset: 0.9,
                     strong: false, flow: flowReject)
            }
            .drawingGroup()
        }
        .allowsHitTesting(false)
        .onReceive(NotificationCenter.default.publisher(for: NSScrollView.willStartLiveScrollNotification)) { _ in
            isScrolling = true
        }
        .onReceive(NotificationCenter.default.publisher(for: NSScrollView.didEndLiveScrollNotification)) { _ in
            isScrolling = false
        }
    }

    private func link(_ ctx: GraphicsContext, from: String, to: String, color: Color,
                      phase: Double, offset: Double, strong: Bool, flow: Bool) {
        guard let a = points[from], let b = points[to] else { return }
        let (path, pts) = orthoPath(from: a, to: b)
        ctx.stroke(path, with: .color(color.opacity(strong ? 0.55 : 0.22)),
                   style: StrokeStyle(lineWidth: strong ? 2 : 1.3, lineCap: .round, lineJoin: .round))
        ctx.fill(Path(ellipseIn: CGRect(x: a.x - 2.5, y: a.y - 2.5, width: 5, height: 5)), with: .color(color.opacity(strong ? 0.85 : 0.4)))
        ctx.fill(Path(ellipseIn: CGRect(x: b.x - 2.5, y: b.y - 2.5, width: 5, height: 5)), with: .color(color.opacity(strong ? 0.85 : 0.4)))
        guard active, flow else { return }
        let progress = CGFloat(((phase / 1.7) + offset).truncatingRemainder(dividingBy: 1))
        let dot = point(at: progress, along: pts)
        ctx.fill(Path(ellipseIn: CGRect(x: dot.x - 6, y: dot.y - 6, width: 12, height: 12)), with: .color(color.opacity(0.16)))
        ctx.fill(Path(ellipseIn: CGRect(x: dot.x - 2.8, y: dot.y - 2.8, width: 5.6, height: 5.6)), with: .color(color))
    }

    // Orthogonal routing with rounded elbows: vertical run, shared horizontal
    // bus at the midpoint, vertical run into the target port.
    private func orthoPath(from a: CGPoint, to b: CGPoint) -> (Path, [CGPoint]) {
        var path = Path()
        path.move(to: a)
        if abs(a.x - b.x) < 2 {
            path.addLine(to: b)
            return (path, [a, b])
        }
        let midY = a.y + (b.y - a.y) * 0.5
        let radius = min(7, abs(b.x - a.x) / 2, abs(b.y - a.y) / 4)
        let dirY: CGFloat = b.y > a.y ? 1 : -1
        let dirX: CGFloat = b.x > a.x ? 1 : -1
        path.addLine(to: CGPoint(x: a.x, y: midY - radius * dirY))
        path.addQuadCurve(to: CGPoint(x: a.x + radius * dirX, y: midY),
                          control: CGPoint(x: a.x, y: midY))
        path.addLine(to: CGPoint(x: b.x - radius * dirX, y: midY))
        path.addQuadCurve(to: CGPoint(x: b.x, y: midY + radius * dirY),
                          control: CGPoint(x: b.x, y: midY))
        path.addLine(to: b)
        return (path, [a, CGPoint(x: a.x, y: midY), CGPoint(x: b.x, y: midY), b])
    }

    private func point(at fraction: CGFloat, along pts: [CGPoint]) -> CGPoint {
        var lengths: [CGFloat] = []
        var total: CGFloat = 0
        for i in 1..<pts.count {
            let d = hypot(pts[i].x - pts[i - 1].x, pts[i].y - pts[i - 1].y)
            lengths.append(d)
            total += d
        }
        guard total > 0 else { return pts[0] }
        var remaining = fraction * total
        for i in 1..<pts.count {
            let d = lengths[i - 1]
            if remaining <= d, d > 0 {
                let k = remaining / d
                return CGPoint(x: pts[i - 1].x + (pts[i].x - pts[i - 1].x) * k,
                               y: pts[i - 1].y + (pts[i].y - pts[i - 1].y) * k)
            }
            remaining -= d
        }
        return pts.last ?? .zero
    }
}

private struct RoutingRulesEditor: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var draft: [RoutingRule]
    @State private var isSaving = false
    @State private var saveError: String?
    @State private var editorMode = "list"
    @State private var text = ""
    @State private var parseNote: String?

    init(rules: [RoutingRule]) { _draft = State(initialValue: rules) }

    var body: some View {
        VStack(spacing: 0) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("分流规则").font(.title3.weight(.semibold))
                    Text("自上而下匹配，命中第一条即生效；拖拽行可调整优先级。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                Picker("", selection: $editorMode) {
                    Text("列表").tag("list")
                    Text("文本").tag("text")
                }
                .labelsHidden().pickerStyle(.segmented).frame(width: 130)
                .onChange(of: editorMode) { mode in
                    if mode == "text" {
                        text = draft.isEmpty ? ruleTemplateText : serializeRuleLines(draft)
                        parseNote = nil
                    } else {
                        syncTextToDraft()
                    }
                }
                if editorMode == "list" {
                    Button { addRule() } label: { Label("添加规则", systemImage: "plus") }
                        .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                }
            }
            .padding(.horizontal, 20).padding(.vertical, 16)
            .background(Theme.panel)
            Divider().overlay(Theme.border)

            if editorMode == "text" {
                VStack(alignment: .leading, spacing: 8) {
                    Text("每行一条：类型,值,动作。支持 Clash 风格（DOMAIN / DOMAIN-SUFFIX / IP-CIDR；DIRECT / REJECT，其他目标视为代理）。# 开头为注释；行尾 “# 自动学习” 备注表示由回退学习自动生成；PROCESS-NAME 等不支持的类型会被跳过。")
                        .font(.caption).foregroundStyle(Theme.muted)
                        .fixedSize(horizontal: false, vertical: true)
                    TextEditor(text: $text)
                        .font(.system(size: 12, design: .monospaced))
                        .scrollContentBackground(.hidden)
                        .scrollIndicators(.hidden)
                        .padding(8)
                        .background(Theme.panel)
                        .overlay(RoundedRectangle(cornerRadius: 7).stroke(Theme.border, lineWidth: 0.7))
                        .clipShape(RoundedRectangle(cornerRadius: 7))
                    if let parseNote {
                        Text(parseNote).font(.caption2).foregroundStyle(Theme.yellow)
                    }
                }
                .padding(20)
            } else if draft.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "arrow.triangle.branch").font(.system(size: 28)).foregroundStyle(Theme.muted)
                    Text("暂无规则，全部流量使用默认出口").font(.callout).foregroundStyle(Theme.muted)
                    Button { addRule() } label: { Label("添加第一条规则", systemImage: "plus") }.buttonStyle(.bordered)
                    Button { editorMode = "text" } label: { Label("粘贴文本规则", systemImage: "doc.on.clipboard") }.buttonStyle(.plain).font(.caption).foregroundStyle(Theme.cyan)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List {
                    HStack(spacing: 10) {
                        Text("优先级").frame(width: 50, alignment: .leading)
                        Text("类型").frame(width: 116, alignment: .leading)
                        Text("匹配值").frame(maxWidth: .infinity, alignment: .leading)
                        Text("动作").frame(width: 128, alignment: .leading)
                        Color.clear.frame(width: 24)
                    }
                    .font(.system(size: 10, weight: .bold)).foregroundStyle(Theme.muted)
                    .padding(.horizontal, 12)
                    .listRowSeparator(.hidden)
                    .listRowBackground(Color.clear)
                    .listRowInsets(EdgeInsets(top: 10, leading: 20, bottom: 2, trailing: 20))
                    ForEach(Array($draft.enumerated()), id: \.element.id) { index, $rule in
                        HStack(spacing: 10) {
                            HStack(spacing: 6) {
                                Image(systemName: "line.3.horizontal")
                                    .font(.system(size: 10)).foregroundStyle(Theme.muted)
                                Text("\(index + 1)")
                                    .font(.system(size: 11, weight: .bold, design: .monospaced))
                                    .foregroundStyle(Theme.muted)
                                    .frame(width: 22, height: 22)
                                    .background(Theme.panelRaised)
                                    .clipShape(RoundedRectangle(cornerRadius: 5))
                            }
                            .frame(width: 50, alignment: .leading)
                            Picker("类型", selection: $rule.type) {
                                Text("完整域名").tag("domain")
                                Text("域名后缀").tag("domain-suffix")
                                Text("IP-CIDR").tag("ip-cidr")
                            }.labelsHidden().frame(width: 116)
                            TextField(placeholder(for: rule.type), text: $rule.value)
                                .textFieldStyle(DarkFieldStyle())
                                .font(.system(.body, design: .monospaced))
                            if rule.learned {
                                Text("自动学习")
                                    .font(.system(size: 9, weight: .bold))
                                    .foregroundStyle(Theme.yellow)
                                    .padding(.horizontal, 5).padding(.vertical, 2)
                                    .background(Theme.yellow.opacity(0.12))
                                    .clipShape(RoundedRectangle(cornerRadius: 4))
                                    .help("代理拨号失败后回退直连多次成功，自动生成的规则；可随时删除")
                            }
                            HStack(spacing: 6) {
                                Circle().fill(actionColor(rule.action)).frame(width: 7, height: 7)
                                Picker("动作", selection: $rule.action) {
                                    Text("上游代理").tag("proxy")
                                    Text("本机直连").tag("direct")
                                    Text("拒绝").tag("reject")
                                }.labelsHidden()
                            }.frame(width: 128)
                            Button { draft.removeAll { $0.id == rule.id } } label: {
                                Image(systemName: "trash")
                                    .font(.system(size: 11))
                                    .foregroundStyle(Theme.muted)
                            }
                            .buttonStyle(.plain)
                            .frame(width: 24)
                            .help("删除此规则")
                        }
                        .padding(.horizontal, 12).frame(height: 52)
                        .background(Theme.panel)
                        .overlay(RoundedRectangle(cornerRadius: 7).stroke(Theme.border, lineWidth: 0.7))
                        .clipShape(RoundedRectangle(cornerRadius: 7))
                        .listRowSeparator(.hidden)
                        .listRowBackground(Color.clear)
                        .listRowInsets(EdgeInsets(top: 4, leading: 20, bottom: 4, trailing: 20))
                    }
                    .onMove { indices, offset in
                        draft.move(fromOffsets: indices, toOffset: offset)
                    }
                }
                .listStyle(.plain)
                .scrollContentBackground(.hidden)
                .scrollIndicators(.hidden)
            }

            Divider().overlay(Theme.border)
            HStack(spacing: 8) {
                if let saveError {
                    Label(saveError, systemImage: "exclamationmark.triangle.fill")
                        .font(.caption).foregroundStyle(Theme.coral).lineLimit(2)
                } else {
                    RuleCountChip(label: "代理", count: count("proxy"), color: Theme.cyan)
                    RuleCountChip(label: "直连", count: count("direct"), color: Theme.lime)
                    RuleCountChip(label: "拒绝", count: count("reject"), color: Theme.coral)
                }
                Spacer()
                Button("取消") { dismiss() }.buttonStyle(.bordered).disabled(isSaving)
                Button {
                    save()
                } label: {
                    if isSaving {
                        ProgressView().controlSize(.small).frame(width: 56)
                    } else {
                        Text("应用规则")
                    }
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                .disabled((editorMode == "list" && hasInvalidRule) || isSaving || model.isBusy)
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
            .background(Theme.panel)
        }
        .frame(width: 760, height: 520).background(Theme.canvas)
    }

    private func save() {
        if editorMode == "text" { syncTextToDraft() }
        saveError = nil
        isSaving = true
        Task {
            let ok = await model.applyRoutingRules(draft)
            isSaving = false
            if ok {
                dismiss()
            } else {
                saveError = model.errorMessage ?? "保存失败，请检查规则"
            }
        }
    }

    private var ruleTemplateText: String {
        """
        # 每行一条规则：类型,值,动作（删掉行首 # 即可启用）
        # 动作：PROXY 走上游代理 / DIRECT 本机直连 / REJECT 拒绝

        # == 走代理的例子 ==
        # DOMAIN-SUFFIX,youtube.com,PROXY
        # DOMAIN-SUFFIX,googlevideo.com,PROXY
        # DOMAIN-SUFFIX,netflix.com,PROXY
        # DOMAIN-SUFFIX,openai.com,PROXY

        # == 国内服务直连的例子 ==
        # DOMAIN-SUFFIX,bilibili.com,DIRECT
        # DOMAIN-SUFFIX,aliyun.com,DIRECT
        # DOMAIN-SUFFIX,qq.com,DIRECT

        # == 屏蔽的例子 ==
        # DOMAIN-SUFFIX,doubleclick.net,REJECT
        # IP-CIDR,203.0.113.0/24,REJECT
        """
    }

    private func syncTextToDraft() {
        let result = parseRuleLines(text)
        draft = result.rules
        parseNote = result.skipped.isEmpty
            ? nil
            : "已跳过 \(result.skipped.count) 行不支持的规则：\(result.skipped.prefix(3).joined(separator: "；"))\(result.skipped.count > 3 ? " …" : "")"
    }

    private func placeholder(for type: String) -> String {
        type == "ip-cidr" ? "例如 192.168.1.0/24" : "例如 openai.com"
    }

    private func actionColor(_ action: String) -> Color {
        switch action {
        case "proxy": return Theme.cyan
        case "reject": return Theme.coral
        default: return Theme.lime
        }
    }

    private var hasInvalidRule: Bool { draft.contains { $0.value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty } }
    private func count(_ action: String) -> Int { draft.filter { $0.action == action }.count }
    private func addRule() { draft.append(RoutingRule(type: "domain-suffix", value: "", action: "proxy")) }
}

private struct RuleCountChip: View {
    let label: String
    let count: Int
    let color: Color
    var body: some View {
        HStack(spacing: 5) {
            Circle().fill(color).frame(width: 6, height: 6)
            Text("\(label) \(count)")
        }
        .font(.system(size: 10, weight: .semibold))
        .foregroundStyle(color)
        .padding(.horizontal, 8).frame(height: 22)
        .background(color.opacity(0.09))
        .clipShape(RoundedRectangle(cornerRadius: 5))
    }
}

private struct SettingsView: View {
    @EnvironmentObject private var model: AppModel

    private var content: some View {
        Group {
            Panel {
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text("外观主题").font(.system(size: 13, weight: .semibold))
                            Text("即时生效，自动记住选择").font(.caption2).foregroundStyle(Theme.muted)
                        }
                        Spacer()
                    }
                    HStack(spacing: 12) {
                        ForEach(ThemePalette.all) { palette in
                            ThemeCard(palette: palette, selected: model.themeID == palette.id) {
                                model.themeID = palette.id
                            }
                        }
                        Spacer(minLength: 0)
                    }
                }
            }
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
                    Button("安装 CLI") { model.installCLI() }.buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                }
                Divider().overlay(Theme.border)
                SettingsRow(title: "开机自启", detail: "\(model.serviceStatus) · 使用 /usr/local/bin/gateway") {
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

private struct LiveLogView: View {
    let text: String
    private let bottomID = "log-bottom"

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView([.horizontal, .vertical], showsIndicators: false) {
                VStack(alignment: .leading, spacing: 0) {
                    Text(text)
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(Color.primary.opacity(0.78))
                        .textSelection(.enabled)
                        .fixedSize(horizontal: true, vertical: true)
                    Color.clear.frame(height: 1).id(bottomID)
                }
                .frame(maxWidth: .infinity, alignment: .topLeading)
            }
            .onAppear { scrollToBottom(proxy, animated: false) }
            .onChange(of: text) { _ in scrollToBottom(proxy, animated: true) }
        }
        .frame(minHeight: 180, maxHeight: .infinity)
        .clipped()
    }

    private func scrollToBottom(_ proxy: ScrollViewProxy, animated: Bool) {
        DispatchQueue.main.async {
            if animated {
                withAnimation(.easeOut(duration: 0.22)) { proxy.scrollTo(bottomID, anchor: .bottom) }
            } else {
                proxy.scrollTo(bottomID, anchor: .bottom)
            }
        }
    }
}

// MARK: - Components

private struct Panel<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        VStack(alignment: .leading, spacing: 0) { content }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .background(Theme.panel)
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(Theme.border, lineWidth: 0.7))
            .clipShape(RoundedRectangle(cornerRadius: 7))
            .shadow(color: Color.black.opacity(0.035), radius: 7, y: 2)
    }
}

private struct MetricCard: View {
    let label: String, value: String, icon: String
    let color: Color
    init(_ label: String, _ value: String, _ icon: String, _ color: Color) { self.label = label; self.value = value; self.icon = icon; self.color = color }
    var body: some View {
        HStack(spacing: 11) {
            ZStack { Circle().fill(color.opacity(0.11)); Image(systemName: icon).foregroundStyle(color).font(.system(size: 15, weight: .semibold)) }.frame(width: 34, height: 34)
            VStack(alignment: .leading, spacing: 3) { Text(label).font(.caption).foregroundStyle(Theme.muted); Text(value).font(.system(size: 18, weight: .semibold, design: .rounded)).lineLimit(1).minimumScaleFactor(0.7) }
            Spacer(minLength: 0)
        }.padding(.horizontal, 13).frame(minHeight: 66).background(Theme.panel).overlay(RoundedRectangle(cornerRadius: 7).stroke(Theme.border, lineWidth: 0.7)).clipShape(RoundedRectangle(cornerRadius: 7))
    }
}

private struct LiveBadge: View {
    let active: Bool
    var body: some View { HStack(spacing: 6) { Circle().fill(active ? Theme.lime : Theme.coral).frame(width: 6, height: 6); Text(active ? "正常" : "异常") }.font(.system(size: 10, weight: .semibold)).foregroundStyle(active ? Theme.lime : Theme.coral).padding(.horizontal, 8).frame(height: 24).background((active ? Theme.lime : Theme.coral).opacity(0.09)).clipShape(RoundedRectangle(cornerRadius: 5)) }
}

private struct ValuePair: View {
    let label: String, value: String
    var body: some View { VStack(alignment: .leading, spacing: 3) { Text(label.uppercased()).font(.system(size: 8, weight: .bold)).foregroundStyle(Theme.muted); Text(value).font(.system(size: 12, weight: .semibold, design: .monospaced)).lineLimit(1) } }
}

private struct ChartLegend: View {
    let color: Color, text: String
    var body: some View { HStack(spacing: 5) { Capsule().fill(color).frame(width: 13, height: 3); Text(text).font(.caption2).foregroundStyle(Theme.muted) } }
}

private struct RecentStrip: View {
    @EnvironmentObject private var model: AppModel
    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                HStack { Text("最近连接").sectionLabel(); Spacer(); Button("查看全部") { model.selectedSection = .connections }.buttonStyle(.plain).foregroundStyle(Theme.cyan).font(.caption) }
                ForEach(Array((model.stats?.relay.recent ?? []).prefix(5))) { item in
                    HStack(spacing: 12) {
                        Circle().fill(serviceColor(item.service)).frame(width: 7, height: 7)
                        Text(item.service).fontWeight(.medium).frame(width: 105, alignment: .leading)
                        Text(item.dstHost).font(.system(.caption, design: .monospaced)).foregroundStyle(Theme.muted).lineLimit(1)
                        Spacer()
                        Text(item.srcIP).font(.system(.caption, design: .monospaced)).foregroundStyle(Theme.muted)
                        Text(bytes(item.up + item.down)).font(.caption).frame(width: 70, alignment: .trailing)
                    }.frame(height: 25)
                }
                if model.stats?.relay.recent.isEmpty != false { EmptyTelemetry(icon: "clock", text: "等待访问记录") }
            }
        }
    }
}

private struct EmptyTelemetry: View {
    let icon: String, text: String
    var body: some View { VStack(spacing: 9) { Image(systemName: icon).font(.title2).foregroundStyle(Theme.muted); Text(text).font(.caption).foregroundStyle(Theme.muted) }.frame(maxWidth: .infinity, maxHeight: .infinity).padding(24) }
}

private struct SetupValue: View {
    let label: String, value: String
    init(_ label: String, _ value: String) { self.label = label; self.value = value }
    var body: some View { VStack(alignment: .leading, spacing: 6) { Text(label).font(.caption).foregroundStyle(Theme.muted); HStack(spacing: 7) { Text(value).font(.system(.body, design: .monospaced)); Button { NSPasteboard.general.clearContents(); NSPasteboard.general.setString(value, forType: .string) } label: { Image(systemName: "doc.on.doc") }.buttonStyle(.plain).foregroundStyle(Theme.cyan) } } }
}

private struct QualityRow: View {
    let label: String, value: String, color: Color
    init(_ label: String, _ value: String, _ color: Color) { self.label = label; self.value = value; self.color = color }
    var body: some View { HStack { Text(label).foregroundStyle(Theme.muted); Spacer(); Text(value).foregroundStyle(color).fontWeight(.semibold) }.font(.caption) }
}

private struct ExitFact: View {
    let label: String, value: String, icon: String
    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: icon).foregroundStyle(Theme.cyan).frame(width: 22)
            VStack(alignment: .leading, spacing: 3) {
                Text(label).font(.caption2).foregroundStyle(Theme.muted)
                Text(value).font(.system(size: 12, weight: .semibold, design: .rounded))
                    .lineLimit(1).minimumScaleFactor(0.72)
            }
            Spacer(minLength: 8)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 10)
    }
}

private struct SettingsRow<Actions: View>: View {
    let title: String, detail: String
    @ViewBuilder let actions: Actions
    var body: some View { HStack { VStack(alignment: .leading, spacing: 4) { Text(title).fontWeight(.semibold); Text(detail).font(.caption).foregroundStyle(Theme.muted).lineLimit(1) }; Spacer(); HStack { actions } }.padding(.vertical, 10) }
}

// Miniature app mock-up rendered in a palette's own colors, used as the
// theme switcher preview so users see the skin before applying it.
private struct ThemeCard: View {
    let palette: ThemePalette
    let selected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(spacing: 8) {
                ZStack {
                    RoundedRectangle(cornerRadius: 7).fill(palette.canvas)
                    HStack(spacing: 5) {
                        RoundedRectangle(cornerRadius: 2.5)
                            .fill(palette.sidebar)
                            .frame(width: 22)
                            .overlay(alignment: .top) {
                                VStack(spacing: 3) {
                                    Capsule().fill(palette.cyan.opacity(0.85)).frame(width: 14, height: 3)
                                    Capsule().fill(palette.muted.opacity(0.5)).frame(width: 14, height: 3)
                                    Capsule().fill(palette.muted.opacity(0.5)).frame(width: 14, height: 3)
                                }
                                .padding(.top, 7)
                            }
                        VStack(spacing: 4) {
                            RoundedRectangle(cornerRadius: 2.5)
                                .fill(palette.panel)
                                .overlay(
                                    HStack(spacing: 3) {
                                        Circle().fill(palette.cyan).frame(width: 5, height: 5)
                                        Circle().fill(palette.lime).frame(width: 5, height: 5)
                                        Circle().fill(palette.yellow).frame(width: 5, height: 5)
                                        Circle().fill(palette.coral).frame(width: 5, height: 5)
                                    }
                                )
                                .overlay(RoundedRectangle(cornerRadius: 2.5).stroke(palette.border, lineWidth: 0.5))
                            RoundedRectangle(cornerRadius: 2.5)
                                .fill(palette.panel)
                                .overlay(alignment: .bottomLeading) {
                                    HStack(alignment: .bottom, spacing: 2.5) {
                                        ForEach(0..<7, id: \.self) { i in
                                            Capsule()
                                                .fill(palette.cyan.opacity(0.75))
                                                .frame(width: 3, height: [7, 11, 5, 13, 9, 15, 6][i])
                                        }
                                    }
                                    .padding(5)
                                }
                                .overlay(RoundedRectangle(cornerRadius: 2.5).stroke(palette.border, lineWidth: 0.5))
                        }
                    }
                    .padding(6)
                }
                .frame(width: 128, height: 82)
                .overlay(
                    RoundedRectangle(cornerRadius: 7)
                        .stroke(selected ? Theme.cyan : Theme.border, lineWidth: selected ? 1.8 : 0.7)
                )
                .overlay(alignment: .topTrailing) {
                    if selected {
                        Image(systemName: "checkmark.circle.fill")
                            .font(.system(size: 13))
                            .foregroundStyle(Theme.cyan)
                            .background(Circle().fill(palette.panel))
                            .offset(x: 5, y: -5)
                    }
                }
                Text(palette.name)
                    .font(.system(size: 11, weight: selected ? .semibold : .regular))
                    .foregroundStyle(selected ? Color.primary : Theme.muted)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

private struct NoticeBar: View {
    let text: String
    var body: some View { Label(text, systemImage: "checkmark.circle.fill").font(.subheadline.weight(.medium)).foregroundStyle(Color.black).padding(.horizontal, 14).frame(minHeight: 38).background(Theme.lime).clipShape(RoundedRectangle(cornerRadius: 6)).shadow(color: Color.black.opacity(0.4), radius: 10, y: 4) }
}

private struct IconButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View { configuration.label.foregroundStyle(Color.primary.opacity(0.8)).frame(width: 30, height: 30).background(configuration.isPressed ? Theme.sidebar : Theme.panel).overlay(RoundedRectangle(cornerRadius: 6).stroke(Theme.border, lineWidth: 0.7)).clipShape(RoundedRectangle(cornerRadius: 6)) }
}

private struct ActionButtonStyle: ButtonStyle {
    let tint: Color
    func makeBody(configuration: Configuration) -> some View { configuration.label.font(.system(size: 12, weight: .semibold)).foregroundStyle(Color.white).padding(.horizontal, 13).frame(minHeight: 30).background(tint.opacity(configuration.isPressed ? 0.72 : 0.92)).clipShape(RoundedRectangle(cornerRadius: 6)) }
}

private struct DarkFieldStyle: TextFieldStyle {
    func _body(configuration: TextField<Self._Label>) -> some View { configuration.padding(.horizontal, 11).frame(height: 36).background(Theme.panelRaised).overlay(RoundedRectangle(cornerRadius: 6).stroke(Theme.border, lineWidth: 0.8)).clipShape(RoundedRectangle(cornerRadius: 6)) }
}

private extension Text {
    func eyebrow() -> some View { self.font(.system(size: 10, weight: .bold)).foregroundStyle(Theme.muted) }
    func sectionLabel() -> some View { self.font(.system(size: 12, weight: .semibold)).foregroundStyle(Color.primary.opacity(0.82)) }
    func fieldLabel() -> some View { self.font(.system(size: 11, weight: .medium)).foregroundStyle(Theme.muted) }
}

private extension String { var nonEmpty: String? { isEmpty ? nil : self } }

private func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
private func shortBytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
private func speed(_ fiveSecondBytes: Int64) -> String { bytes(fiveSecondBytes / 5) + "/s" }
private func uptime(_ seconds: Int64) -> String { seconds > 3600 ? "\(seconds / 3600)H" : "\(max(seconds / 60, 0))M" }
private func formatMS(_ value: Double?) -> String { guard let value, value > 0 else { return "--" }; return String(format: "%.1f ms", value) }
private func serviceColor(_ service: String) -> Color { [Theme.cyan, Theme.lime, Theme.coral, Theme.yellow][Int(service.hashValue.magnitude % 4)] }
private func egressLocation(_ identity: EgressIdentity?) -> String {
    guard let identity else { return "地区待检测" }
    let country = identity.countryCode.flatMap {
        Locale(identifier: "zh-Hans").localizedString(forRegionCode: $0)
    }
    let place = identity.city?.nonEmpty ?? identity.region?.nonEmpty
    return [country, place].compactMap { $0 }.uniqued().joined(separator: " · ").nonEmpty ?? "地区未知"
}

// Parses Clash-style rule lines ("DOMAIN-SUFFIX,example.com,DIRECT").
// Unsupported types (PROCESS-NAME, ...) are skipped; unknown targets map to
// proxy. A trailing "# 自动学习" comment restores the learned marker.
private let learnedRuleComment = "自动学习"

private func parseRuleLines(_ text: String) -> (rules: [RoutingRule], skipped: [String]) {
    var rules: [RoutingRule] = []
    var skipped: [String] = []
    for rawLine in text.split(separator: "\n", omittingEmptySubsequences: true) {
        var line = rawLine.trimmingCharacters(in: .whitespaces)
        guard !line.isEmpty else { continue }
        if line.hasPrefix("#") || line.hasPrefix("//") { continue }
        var learned = false
        if let hashIndex = line.firstIndex(of: "#") {
            let comment = line[line.index(after: hashIndex)...]
            learned = comment.contains(learnedRuleComment)
            line = String(line[..<hashIndex])
        }
        line = line.trimmingCharacters(in: CharacterSet(charactersIn: "\"'，,"))
            .trimmingCharacters(in: .whitespaces)
        guard !line.isEmpty else { continue }
        let parts = line.replacingOccurrences(of: "，", with: ",")
            .split(separator: ",", omittingEmptySubsequences: true)
            .map { $0.trimmingCharacters(in: .whitespaces) }
        guard parts.count >= 2 else {
            skipped.append(String(rawLine.prefix(40)))
            continue
        }
        let type: String
        switch parts[0].uppercased() {
        case "DOMAIN": type = "domain"
        case "DOMAIN-SUFFIX": type = "domain-suffix"
        case "IP-CIDR", "IP-CIDR6": type = "ip-cidr"
        default:
            skipped.append(String(rawLine.prefix(40)))
            continue
        }
        let action: String
        if parts.count < 3 {
            action = "proxy"
        } else {
            switch parts[2].uppercased() {
            case "DIRECT": action = "direct"
            case "REJECT", "REJECT-DROP", "BLOCK": action = "reject"
            default: action = "proxy"
            }
        }
        rules.append(RoutingRule(type: type, value: parts[1], action: action, learned: learned))
    }
    return (rules, skipped)
}

private func serializeRuleLines(_ rules: [RoutingRule]) -> String {
    rules.map { rule in
        let type: String
        switch rule.type {
        case "domain": type = "DOMAIN"
        case "domain-suffix": type = "DOMAIN-SUFFIX"
        case "ip-cidr": type = "IP-CIDR"
        default: type = rule.type.uppercased()
        }
        let action: String
        switch rule.action {
        case "direct": action = "DIRECT"
        case "reject": action = "REJECT"
        default: action = "PROXY"
        }
        return "\(type),\(rule.value),\(action)\(rule.learned ? " # \(learnedRuleComment)" : "")"
    }.joined(separator: "\n")
}

private func suggestedDeviceIPPool(gateway: String, occupied: Set<String>) -> [String] {
    let octets = gateway.split(separator: ".").compactMap { Int($0) }
    guard octets.count == 4, octets.allSatisfy({ (0...255).contains($0) }) else { return [] }
    let prefix = "\(octets[0]).\(octets[1]).\(octets[2])"
    return (201...254)
        .map { "\(prefix).\($0)" }
        .filter { $0 != gateway && !occupied.contains($0) }
}

// Best-effort liveness probe: one ping with a 300ms reply window per address.
private func probeOccupiedAddresses(_ addresses: [String]) async -> Set<String> {
    await withTaskGroup(of: (String, Bool).self) { group in
        for address in addresses {
            group.addTask {
                let process = Process()
                process.executableURL = URL(fileURLWithPath: "/sbin/ping")
                process.arguments = ["-c", "1", "-W", "300", "-q", address]
                process.standardOutput = FileHandle.nullDevice
                process.standardError = FileHandle.nullDevice
                do { try process.run() } catch { return (address, false) }
                process.waitUntilExit()
                return (address, process.terminationStatus == 0)
            }
        }
        var occupied = Set<String>()
        for await (address, alive) in group where alive { occupied.insert(address) }
        return occupied
    }
}

private func suggestedDeviceIPs(gateway: String, occupied: Set<String>) -> [String] {
    let octets = gateway.split(separator: ".").compactMap { Int($0) }
    guard octets.count == 4, octets.allSatisfy({ (0...255).contains($0) }) else { return [] }
    let prefix = "\(octets[0]).\(octets[1]).\(octets[2])"
    return (201...254)
        .map { "\(prefix).\($0)" }
        .filter { $0 != gateway && !occupied.contains($0) }
        .prefix(5)
        .map { $0 }
}

private func suggestionsRange(_ suggestions: [String]) -> String {
    guard let first = suggestions.first else { return "暂不可用" }
    guard let last = suggestions.last, last != first else { return first }
    let lastOctet = last.split(separator: ".").last.map(String.init) ?? last
    return "\(first)–\(lastOctet)"
}

private func suggestionPrefix(_ suggestions: [String]) -> String {
    guard let first = suggestions.first else { return "--" }
    return first.split(separator: ".").dropLast().joined(separator: ".")
}

private extension Array where Element == String {
    func uniqued() -> [String] {
        reduce(into: []) { result, value in
            if !result.contains(value) { result.append(value) }
        }
    }
}
