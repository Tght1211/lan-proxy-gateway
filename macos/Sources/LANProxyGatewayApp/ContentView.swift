import AppKit
import Charts
import SwiftUI

private enum Theme {
    static let canvas = Color(red: 0.035, green: 0.043, blue: 0.047)
    static let sidebar = Color(red: 0.055, green: 0.064, blue: 0.068)
    static let panel = Color(red: 0.075, green: 0.086, blue: 0.09)
    static let panelRaised = Color(red: 0.095, green: 0.108, blue: 0.112)
    static let border = Color.white.opacity(0.09)
    static let cyan = Color(red: 0.20, green: 0.86, blue: 0.82)
    static let coral = Color(red: 1.0, green: 0.38, blue: 0.31)
    static let lime = Color(red: 0.62, green: 0.91, blue: 0.30)
    static let yellow = Color(red: 1.0, green: 0.78, blue: 0.24)
    static let muted = Color.white.opacity(0.55)
}

struct ContentView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
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
        .preferredColorScheme(.dark)
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
                    RoundedRectangle(cornerRadius: 7).fill(Theme.cyan)
                    Image(systemName: "network").foregroundStyle(Color.black).font(.system(size: 18, weight: .bold))
                }
                .frame(width: 34, height: 34)
                VStack(alignment: .leading, spacing: 1) {
                    Text("LAN GATEWAY").font(.system(size: 13, weight: .bold))
                    Text("NETWORK CORE").font(.system(size: 9, weight: .semibold)).foregroundStyle(Theme.cyan)
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
            .listStyle(.sidebar)

            VStack(alignment: .leading, spacing: 9) {
                HStack(spacing: 8) {
                    Circle().fill(model.isRunning ? Theme.lime : Theme.muted).frame(width: 7, height: 7)
                    Text(model.isRunning ? "CORE ONLINE" : "CORE OFFLINE")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundStyle(model.isRunning ? Theme.lime : Theme.muted)
                }
                Text(model.status?.gateway.localIP.nonEmpty ?? "等待网络检测")
                    .font(.system(size: 12, design: .monospaced)).foregroundStyle(Theme.muted)
            }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Color.black.opacity(0.16))
        }
        .background(Theme.sidebar)
        .navigationSplitViewColumnWidth(min: 190, ideal: 210, max: 230)
    }

    @ViewBuilder private var detail: some View {
        switch model.selectedSection ?? .overview {
        case .overview: OverviewView()
        case .traffic: TrafficView()
        case .services: ServicesView()
        case .devices: DevicesView()
        case .stability: StabilityView()
        case .connections: ConnectionsView()
        case .proxy: ProxyView()
        case .settings: SettingsView()
        }
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
                Text(Date.now, style: .time).font(.system(size: 11, design: .monospaced)).foregroundStyle(Theme.muted)
            }
            Spacer()
            if model.isBusy { ProgressView().controlSize(.small) }
            Button { Task { await model.refresh() } } label: { Image(systemName: "arrow.clockwise") }
                .buttonStyle(IconButtonStyle()).help("刷新")
            if model.isRunning {
                Button { model.restart() } label: { Image(systemName: "bolt.fill") }
                    .buttonStyle(IconButtonStyle()).help("重启核心")
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
        .frame(height: 64)
        .background(Theme.canvas)
        .disabled(model.isBusy)
    }
}

private struct OverviewView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                if model.status?.configured == false {
                    GettingStartedPanel()
                }
                HStack(spacing: 16) {
                    CoreHero().frame(width: 310)
                    ThroughputChart(compact: true).frame(minWidth: 480, minHeight: 230)
                }
                LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 12), count: 4), spacing: 12) {
                    MetricCard("实时下载", speed(model.stats?.relay.traffic.last?.down ?? 0), "arrow.down", Theme.cyan)
                    MetricCard("实时上传", speed(model.stats?.relay.traffic.last?.up ?? 0), "arrow.up", Theme.yellow)
                    MetricCard("活动连接", "\(model.stats?.relay.active.count ?? 0)", "point.3.connected.trianglepath.dotted", Theme.lime)
                    MetricCard("活跃设备", "\(model.activeDeviceCount)", "desktopcomputer", Theme.coral)
                }
                HStack(alignment: .top, spacing: 16) {
                    ServiceRanking(limit: 6).frame(maxWidth: .infinity)
                    StabilitySummary().frame(width: 330)
                }
                RecentStrip().frame(minHeight: 210)
            }
            .padding(20)
        }
    }
}

private struct CoreHero: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 17) {
                HStack {
                    Text("CORE STATUS").eyebrow()
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
                    model.selectedSection = .proxy
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
    }
}

private struct ThroughputChart: View {
    @EnvironmentObject private var model: AppModel
    let compact: Bool

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("LIVE THROUGHPUT").eyebrow()
                    Spacer()
                    ChartLegend(color: Theme.cyan, text: "下载")
                    ChartLegend(color: Theme.yellow, text: "上传")
                }
                Chart(model.stats?.relay.traffic ?? []) { point in
                    AreaMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5))
                        .foregroundStyle(LinearGradient(colors: [Theme.cyan.opacity(0.28), Theme.cyan.opacity(0.01)], startPoint: .top, endPoint: .bottom))
                        .interpolationMethod(.catmullRom)
                    LineMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5))
                        .foregroundStyle(Theme.cyan).lineStyle(StrokeStyle(lineWidth: 2)).interpolationMethod(.catmullRom)
                    LineMark(x: .value("时间", point.at), y: .value("上传", Double(point.up) / 5))
                        .foregroundStyle(Theme.yellow).lineStyle(StrokeStyle(lineWidth: 1.5)).interpolationMethod(.catmullRom)
                }
                .chartXAxis(compact ? .hidden : .automatic)
                .chartYAxis {
                    AxisMarks(position: .leading) { value in
                        AxisGridLine().foregroundStyle(Theme.border)
                        AxisValueLabel { if let bytes = value.as(Double.self) { Text(shortBytes(Int64(bytes)) + "/s") } }
                    }
                }
                .chartPlotStyle { $0.background(Color.black.opacity(0.14)) }
            }
        }
    }
}

private struct TrafficView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                HStack(spacing: 12) {
                    MetricCard("会话下载", bytes(model.stats?.relay.downTotal ?? 0), "arrow.down.circle.fill", Theme.cyan)
                    MetricCard("会话上传", bytes(model.stats?.relay.upTotal ?? 0), "arrow.up.circle.fill", Theme.yellow)
                    MetricCard("连接总数", "\(model.stats?.relay.services.reduce(0) { $0 + $1.connections } ?? 0)", "link", Theme.lime)
                }
                ThroughputChart(compact: false).frame(minHeight: 440)
            }.padding(20)
        }
    }
}

private struct ServicesView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                HStack(spacing: 12) {
                    MetricCard("已识别服务", "\(model.stats?.relay.services.count ?? 0)", "square.stack.3d.up.fill", Theme.cyan)
                    MetricCard("访问最多", model.stats?.relay.services.first?.name ?? "--", "crown.fill", Theme.yellow)
                    MetricCard("服务流量", bytes(model.stats?.relay.services.reduce(0) { $0 + $1.total } ?? 0), "chart.bar.fill", Theme.coral)
                }
                ServiceRanking(limit: 14).frame(minHeight: 500)
            }.padding(20)
        }
    }
}

private struct ServiceRanking: View {
    @EnvironmentObject private var model: AppModel
    let limit: Int

    var body: some View {
        let data = Array((model.stats?.relay.services ?? []).prefix(limit))
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("SERVICE TRAFFIC").eyebrow()
                    Spacer()
                    Text("域名级识别").font(.caption).foregroundStyle(Theme.muted)
                }
                if data.isEmpty {
                    EmptyTelemetry(icon: "square.stack.3d.up", text: "等待服务流量")
                } else {
                    Chart(data) { item in
                        BarMark(x: .value("流量", item.total), y: .value("服务", item.name))
                            .foregroundStyle(by: .value("服务", item.name)).cornerRadius(3)
                            .annotation(position: .trailing) { Text(shortBytes(item.total)).font(.caption2).foregroundStyle(Theme.muted) }
                    }
                    .chartLegend(.hidden)
                    .chartXAxis(.hidden)
                    .chartYAxis { AxisMarks { AxisValueLabel().foregroundStyle(Color.white.opacity(0.8)) } }
                }
            }
        }
    }
}

private struct DevicesView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                HStack(spacing: 12) {
                    MetricCard("当前活跃", "\(model.activeDeviceCount)", "desktopcomputer", Theme.lime)
                    MetricCard("网关地址", model.status?.gateway.localIP.nonEmpty ?? "--", "network", Theme.cyan)
                    MetricCard("默认路由", model.status?.gateway.router.nonEmpty ?? "--", "wifi.router", Theme.yellow)
                }
                DeviceRanking().frame(minHeight: 330)
                DeviceSetupPanel()
            }.padding(20)
        }
    }
}

private struct DeviceRanking: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("DEVICE TRAFFIC").eyebrow()
                    Spacer()
                    Text("本次运行会话").font(.caption).foregroundStyle(Theme.muted)
                }
                let devices = Array((model.stats?.relay.devices ?? []).prefix(12))
                if devices.isEmpty {
                    EmptyTelemetry(icon: "desktopcomputer", text: "等待局域网设备接入")
                } else {
                    Chart(devices) { item in
                        BarMark(x: .value("设备", item.name), y: .value("流量", item.total))
                            .foregroundStyle(Theme.lime.gradient).cornerRadius(3)
                    }
                    .chartYAxis { AxisMarks { AxisGridLine().foregroundStyle(Theme.border); AxisValueLabel() } }
                }
            }
        }
    }
}

private struct DeviceSetupPanel: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                Text("DEVICE ONBOARDING").eyebrow()
                HStack(spacing: 28) {
                    SetupValue("网关 / 路由器", model.status?.gateway.localIP.nonEmpty ?? "--")
                    SetupValue("首选 DNS", model.status?.gateway.localIP.nonEmpty ?? "--")
                    SetupValue("子网掩码", "255.255.255.0")
                    SetupValue("前缀长度", "24")
                    SetupValue("代理", "无")
                }
            }
        }
    }
}

private struct StabilityView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                HStack(spacing: 12) {
                    MetricCard("平均延迟", formatMS(model.stats?.health.latencyMS), "timer", Theme.cyan)
                    MetricCard("网络抖动", formatMS(model.stats?.health.jitterMS), "waveform.path", Theme.yellow)
                    MetricCard("可用率", String(format: "%.1f%%", model.stats?.health.availability ?? 0), "checkmark.shield.fill", Theme.lime)
                    MetricCard("连续失败", "\(model.stats?.health.failCount ?? 0)", "exclamationmark.triangle.fill", Theme.coral)
                }
                StabilityChart().frame(minHeight: 420)
                StabilitySummary()
            }.padding(20)
        }
    }
}

private struct StabilityChart: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack { Text("LATENCY MONITOR").eyebrow(); Spacer(); LiveBadge(active: model.stats?.health.healthy == true) }
                Chart(model.stats?.health.history ?? []) { point in
                    LineMark(x: .value("时间", point.at), y: .value("延迟", point.latencyMS))
                        .foregroundStyle(Theme.cyan).interpolationMethod(.catmullRom)
                    PointMark(x: .value("时间", point.at), y: .value("延迟", point.latencyMS))
                        .foregroundStyle(point.ok ? Theme.lime : Theme.coral).symbolSize(point.ok ? 18 : 65)
                }
                .chartYAxis {
                    AxisMarks(position: .leading) { value in
                        AxisGridLine().foregroundStyle(Theme.border)
                        AxisValueLabel { if let ms = value.as(Double.self) { Text("\(Int(ms)) ms") } }
                    }
                }
            }
        }
    }
}

private struct StabilitySummary: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 15) {
                Text("NETWORK QUALITY").eyebrow()
                HStack {
                    ZStack {
                        Circle().stroke(Theme.border, lineWidth: 8)
                        Circle().trim(from: 0, to: min((model.stats?.health.availability ?? 0) / 100, 1))
                            .stroke(Theme.lime, style: StrokeStyle(lineWidth: 8, lineCap: .round)).rotationEffect(.degrees(-90))
                        Text(String(format: "%.0f", model.stats?.health.availability ?? 0)).font(.title2.bold())
                    }.frame(width: 92, height: 92)
                    VStack(alignment: .leading, spacing: 9) {
                        QualityRow("健康状态", model.stats?.health.healthy == true ? "稳定" : "异常", model.stats?.health.healthy == true ? Theme.lime : Theme.coral)
                        QualityRow("平均延迟", formatMS(model.stats?.health.latencyMS), Theme.cyan)
                        QualityRow("平均抖动", formatMS(model.stats?.health.jitterMS), Theme.yellow)
                    }
                }
                if let error = model.stats?.health.lastError, !error.isEmpty {
                    Text(error).font(.caption).foregroundStyle(Theme.coral).lineLimit(2)
                }
            }
        }
    }
}

private struct ConnectionsView: View {
    @EnvironmentObject private var model: AppModel
    @State private var search = ""

    private var connections: [ConnectionInfo] {
        let all = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
        guard !search.isEmpty else { return all }
        return all.filter { $0.srcIP.localizedCaseInsensitiveContains(search) || $0.dstHost.localizedCaseInsensitiveContains(search) || $0.service.localizedCaseInsensitiveContains(search) }
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.muted)
                TextField("搜索设备、服务或域名", text: $search).textFieldStyle(.plain)
                Spacer()
                Text("\(connections.count) RECORDS").eyebrow()
            }
            .padding(.horizontal, 16).frame(height: 44).background(Theme.panel)

            Table(connections) {
                TableColumn("状态") { item in
                    HStack(spacing: 6) { Circle().fill(item.endedAt == nil ? Theme.lime : Theme.muted).frame(width: 6, height: 6); Text(item.endedAt == nil ? "活跃" : "完成") }
                }.width(70)
                TableColumn("设备") { Text($0.srcIP).font(.system(.body, design: .monospaced)) }.width(min: 110, ideal: 130)
                TableColumn("识别服务") { Text($0.service).fontWeight(.medium) }.width(min: 100, ideal: 130)
                TableColumn("目标域名 / 地址") { Text("\($0.dstHost):\($0.dstPort)").font(.system(.body, design: .monospaced)) }
                TableColumn("出口") { Text($0.viaProxy ? "PROXY" : "DIRECT").foregroundStyle($0.viaProxy ? Theme.cyan : Theme.yellow) }.width(70)
                TableColumn("流量") { Text(bytes($0.up + $0.down)) }.width(80)
                TableColumn("时间") { Text($0.startedAt, style: .time) }.width(70)
            }
            .scrollContentBackground(.hidden)
        }
        .background(Theme.canvas)
    }
}

private struct ProxyView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            HStack(alignment: .top, spacing: 16) {
                Panel {
                    VStack(alignment: .leading, spacing: 20) {
                        Text("UPSTREAM EGRESS").eyebrow()
                        Picker("", selection: $model.proxyType) {
                            Text("SOCKS5").tag("socks5")
                            Text("HTTP CONNECT").tag("http")
                        }.pickerStyle(.segmented)
                        VStack(alignment: .leading, spacing: 7) {
                            Text("HOST").eyebrow()
                            TextField("127.0.0.1", text: $model.proxyHost).textFieldStyle(DarkFieldStyle())
                        }
                        VStack(alignment: .leading, spacing: 7) {
                            Text("PORT").eyebrow()
                            TextField("7897", value: $model.proxyPort, format: .number).textFieldStyle(DarkFieldStyle())
                        }
                        HStack {
                            Button(model.isConfigured ? "应用代理" : "保存代理配置") { model.applyProxy() }.buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                            Button("切换直连") { model.useDirectConnection() }.buttonStyle(ActionButtonStyle(tint: Theme.yellow))
                        }.disabled(model.isBusy)
                        if model.status?.configured == false {
                            Text("保存后返回顶部启动核心服务。管理员授权仅用于系统网络与网关设置。")
                                .font(.caption).foregroundStyle(Theme.muted).fixedSize(horizontal: false, vertical: true)
                        }
                    }
                }.frame(maxWidth: 520)
                Panel {
                    VStack(alignment: .leading, spacing: 20) {
                        Text("CURRENT ROUTE").eyebrow()
                        RouteDiagram()
                        Divider().overlay(Theme.border)
                        ValuePair(label: "当前出口", value: model.status?.proxy ?? "DIRECT")
                    }
                }
            }.padding(20)
        }
    }
}

private struct RouteDiagram: View {
    var body: some View {
        HStack(spacing: 8) {
            RouteNode(icon: "desktopcomputer", label: "LAN")
            RouteLine(color: Theme.lime)
            RouteNode(icon: "server.rack", label: "CORE")
            RouteLine(color: Theme.cyan)
            RouteNode(icon: "cloud", label: "UPSTREAM")
        }.frame(maxWidth: .infinity).padding(.vertical, 28)
    }
}

private struct SettingsView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                Panel {
                    SettingsRow(title: "命令行工具", detail: "/usr/local/bin/gateway") {
                        Button("安装 CLI") { model.installCLI() }.buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                    }
                    Divider().overlay(Theme.border)
                    SettingsRow(title: "开机自启", detail: "\(model.serviceStatus) · 使用 /usr/local/bin/gateway") {
                        Button("启用") { model.installService() }.buttonStyle(ActionButtonStyle(tint: Theme.lime))
                        Button("移除") { model.uninstallService() }.buttonStyle(ActionButtonStyle(tint: Theme.coral))
                    }
                    Divider().overlay(Theme.border)
                    SettingsRow(title: "配置文件", detail: model.status?.configFile ?? "--") { EmptyView() }
                }
                Panel {
                    VStack(alignment: .leading, spacing: 12) {
                        HStack { Text("CORE LOG").eyebrow(); Spacer(); Button("刷新") { model.reloadLog() }.buttonStyle(IconButtonStyle()); Button("打开") { model.revealLog() }.buttonStyle(IconButtonStyle()) }
                        ScrollView([.horizontal, .vertical]) {
                            Text(model.logText).font(.system(size: 11, design: .monospaced)).foregroundStyle(Color.white.opacity(0.72)).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .topLeading)
                        }.frame(minHeight: 280)
                    }
                }
            }.padding(20)
        }.task { model.updateServiceStatus(); model.reloadLog() }
    }
}

// MARK: - Components

private struct Panel<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View { content.padding(16).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading).background(Theme.panel).overlay(RoundedRectangle(cornerRadius: 8).stroke(Theme.border)).clipShape(RoundedRectangle(cornerRadius: 8)) }
}

private struct MetricCard: View {
    let label: String, value: String, icon: String
    let color: Color
    init(_ label: String, _ value: String, _ icon: String, _ color: Color) { self.label = label; self.value = value; self.icon = icon; self.color = color }
    var body: some View {
        HStack(spacing: 12) {
            ZStack { RoundedRectangle(cornerRadius: 6).fill(color.opacity(0.14)); Image(systemName: icon).foregroundStyle(color).font(.system(size: 17, weight: .semibold)) }.frame(width: 40, height: 40)
            VStack(alignment: .leading, spacing: 3) { Text(label).font(.caption).foregroundStyle(Theme.muted); Text(value).font(.system(size: 18, weight: .semibold, design: .rounded)).lineLimit(1).minimumScaleFactor(0.7) }
            Spacer(minLength: 0)
        }.padding(14).frame(minHeight: 72).background(Theme.panelRaised).overlay(RoundedRectangle(cornerRadius: 8).stroke(Theme.border)).clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct LiveBadge: View {
    let active: Bool
    var body: some View { HStack(spacing: 6) { Circle().fill(active ? Theme.lime : Theme.coral).frame(width: 6, height: 6); Text(active ? "LIVE" : "OFFLINE") }.font(.system(size: 9, weight: .bold)).foregroundStyle(active ? Theme.lime : Theme.coral).padding(.horizontal, 8).frame(height: 24).background((active ? Theme.lime : Theme.coral).opacity(0.1)).clipShape(RoundedRectangle(cornerRadius: 5)) }
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
                HStack { Text("RECENT ACTIVITY").eyebrow(); Spacer(); Button("查看全部") { model.selectedSection = .connections }.buttonStyle(.plain).foregroundStyle(Theme.cyan).font(.caption) }
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

private struct RouteNode: View {
    let icon: String, label: String
    var body: some View { VStack(spacing: 8) { ZStack { RoundedRectangle(cornerRadius: 7).fill(Theme.panelRaised); Image(systemName: icon).font(.title2).foregroundStyle(Theme.cyan) }.frame(width: 62, height: 62); Text(label).eyebrow() } }
}

private struct RouteLine: View {
    let color: Color
    var body: some View { HStack(spacing: 3) { ForEach(0..<4, id: \.self) { _ in Capsule().fill(color.opacity(0.75)).frame(width: 8, height: 3) } } }
}

private struct SettingsRow<Actions: View>: View {
    let title: String, detail: String
    @ViewBuilder let actions: Actions
    var body: some View { HStack { VStack(alignment: .leading, spacing: 4) { Text(title).fontWeight(.semibold); Text(detail).font(.caption).foregroundStyle(Theme.muted).lineLimit(1) }; Spacer(); HStack { actions } }.padding(.vertical, 10) }
}

private struct NoticeBar: View {
    let text: String
    var body: some View { Label(text, systemImage: "checkmark.circle.fill").font(.subheadline.weight(.medium)).foregroundStyle(Color.black).padding(.horizontal, 14).frame(minHeight: 38).background(Theme.lime).clipShape(RoundedRectangle(cornerRadius: 6)).shadow(color: Color.black.opacity(0.4), radius: 10, y: 4) }
}

private struct IconButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View { configuration.label.frame(width: 30, height: 30).background(configuration.isPressed ? Theme.panelRaised : Theme.panel).overlay(RoundedRectangle(cornerRadius: 6).stroke(Theme.border)).clipShape(RoundedRectangle(cornerRadius: 6)) }
}

private struct ActionButtonStyle: ButtonStyle {
    let tint: Color
    func makeBody(configuration: Configuration) -> some View { configuration.label.font(.system(size: 12, weight: .semibold)).foregroundStyle(Color.black).padding(.horizontal, 13).frame(minHeight: 30).background(tint.opacity(configuration.isPressed ? 0.7 : 1)).clipShape(RoundedRectangle(cornerRadius: 6)) }
}

private struct DarkFieldStyle: TextFieldStyle {
    func _body(configuration: TextField<Self._Label>) -> some View { configuration.padding(.horizontal, 11).frame(height: 36).background(Color.black.opacity(0.22)).overlay(RoundedRectangle(cornerRadius: 6).stroke(Theme.border)).clipShape(RoundedRectangle(cornerRadius: 6)) }
}

private extension Text {
    func eyebrow() -> some View { self.font(.system(size: 10, weight: .bold)).foregroundStyle(Theme.muted) }
}

private extension String { var nonEmpty: String? { isEmpty ? nil : self } }

private func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
private func shortBytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
private func speed(_ fiveSecondBytes: Int64) -> String { bytes(fiveSecondBytes / 5) + "/s" }
private func uptime(_ seconds: Int64) -> String { seconds > 3600 ? "\(seconds / 3600)H" : "\(max(seconds / 60, 0))M" }
private func formatMS(_ value: Double?) -> String { guard let value, value > 0 else { return "--" }; return String(format: "%.1f ms", value) }
private func serviceColor(_ service: String) -> Color { [Theme.cyan, Theme.lime, Theme.coral, Theme.yellow][Int(service.hashValue.magnitude % 4)] }
