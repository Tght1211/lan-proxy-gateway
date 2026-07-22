import AppKit
import Charts
import SwiftUI

private enum Theme {
    static let canvas = Color(red: 0.955, green: 0.961, blue: 0.969)
    static let sidebar = Color(red: 0.925, green: 0.937, blue: 0.945)
    static let panel = Color.white
    static let panelRaised = Color(red: 0.969, green: 0.975, blue: 0.979)
    static let border = Color(red: 0.835, green: 0.855, blue: 0.875)
    static let cyan = Color(red: 0.08, green: 0.42, blue: 0.36)
    static let coral = Color(red: 0.72, green: 0.20, blue: 0.18)
    static let lime = Color(red: 0.20, green: 0.52, blue: 0.28)
    static let yellow = Color(red: 0.78, green: 0.47, blue: 0.08)
    static let muted = Color(nsColor: .secondaryLabelColor)
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
        .tint(Theme.cyan)
        .preferredColorScheme(.light)
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

private struct OverviewView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
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
                ThroughputChart(compact: true).frame(minHeight: 270)
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
                        .interpolationMethod(.catmullRom)
                    LineMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5))
                        .foregroundStyle(Theme.cyan).lineStyle(StrokeStyle(lineWidth: 2)).interpolationMethod(.catmullRom)
                    LineMark(x: .value("时间", point.at), y: .value("上传", Double(point.up) / 5))
                        .foregroundStyle(Theme.yellow).lineStyle(StrokeStyle(lineWidth: 1.5)).interpolationMethod(.catmullRom)
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
                    MetricCard("访问最多", model.stats?.relay.services.first?.displayName ?? "--", "crown.fill", Theme.yellow)
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
                    Text("服务流量").sectionLabel()
                    Spacer()
                    Text("域名级识别").font(.caption).foregroundStyle(Theme.muted)
                }
                if data.isEmpty {
                    EmptyTelemetry(icon: "square.stack.3d.up", text: "等待服务流量")
                } else {
                    let maximum = max(data.first?.total ?? 1, 1)
                    VStack(spacing: 11) {
                        ForEach(data) { item in
                            VStack(spacing: 5) {
                                HStack {
                                    Text(item.displayName).font(.system(size: 12, weight: .medium)).lineLimit(1)
                                    Spacer()
                                    Text(shortBytes(item.total)).font(.system(size: 11, design: .monospaced)).foregroundStyle(Theme.muted)
                                }
                                GeometryReader { geometry in
                                    ZStack(alignment: .leading) {
                                        Capsule().fill(Theme.panelRaised)
                                        Capsule().fill(Theme.cyan.opacity(0.72))
                                            .frame(width: geometry.size.width * CGFloat(Double(item.total) / Double(maximum)))
                                    }
                                }
                                .frame(height: 5)
                            }
                        }
                    }
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
                    Text("设备流量").sectionLabel()
                    Spacer()
                    Text("本次运行会话").font(.caption).foregroundStyle(Theme.muted)
                }
                let devices = Array((model.stats?.relay.devices ?? []).prefix(12))
                if devices.isEmpty {
                    EmptyTelemetry(icon: "desktopcomputer", text: "等待局域网设备接入")
                } else {
                    let maximum = max(devices.first?.total ?? 1, 1)
                    VStack(spacing: 13) {
                        ForEach(devices) { item in
                            VStack(spacing: 6) {
                                HStack {
                                    HStack(spacing: 7) {
                                        Circle().fill(Theme.lime).frame(width: 7, height: 7)
                                        Text(item.name).font(.system(size: 13, weight: .medium, design: .monospaced))
                                    }
                                    Spacer()
                                    Text("\(item.connections) 个连接").font(.caption).foregroundStyle(Theme.muted)
                                    Text(shortBytes(item.total)).font(.system(size: 11, design: .monospaced)).frame(width: 72, alignment: .trailing)
                                }
                                GeometryReader { geometry in
                                    ZStack(alignment: .leading) {
                                        Capsule().fill(Theme.panelRaised)
                                        Capsule().fill(Theme.lime.opacity(0.72))
                                            .frame(width: geometry.size.width * CGFloat(Double(item.total) / Double(maximum)))
                                    }
                                }
                                .frame(height: 6)
                            }
                        }
                    }
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
                Text("设备接入参数").sectionLabel()
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
                HStack { Text("出口延迟").sectionLabel(); Spacer(); LiveBadge(active: model.stats?.health.healthy == true) }
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
                Text("网络质量").sectionLabel()
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
                Text("\(connections.count) 条记录").font(.caption).foregroundStyle(Theme.muted)
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
                        Text("代理出口").sectionLabel()
                        Picker("", selection: $model.proxyType) {
                            Text("SOCKS5").tag("socks5")
                            Text("HTTP CONNECT").tag("http")
                        }.pickerStyle(.segmented).tint(Theme.cyan)
                        VStack(alignment: .leading, spacing: 7) {
                            Text("代理地址").fieldLabel()
                            TextField("127.0.0.1", text: $model.proxyHost).textFieldStyle(DarkFieldStyle())
                        }
                        VStack(alignment: .leading, spacing: 7) {
                            Text("代理端口").fieldLabel()
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
                        Text("当前链路").sectionLabel()
                        RouteDiagram()
                        Divider().overlay(Theme.border)
                        ValuePair(label: "当前出口", value: model.status?.proxy ?? "DIRECT")
                        if let identity = model.stats?.health.egressIdentity {
                            Divider().overlay(Theme.border)
                            HStack(spacing: 28) {
                                ValuePair(label: "公网 IP", value: identity.ip)
                                ValuePair(label: "地区", value: egressLocation(identity))
                                ValuePair(label: "网络", value: identity.isp?.nonEmpty ?? "--")
                            }
                        } else {
                            Text("正在通过当前出口检测公网 IP 和地区...")
                                .font(.caption).foregroundStyle(Theme.muted)
                        }
                    }
                }
            }.padding(20)
        }
    }
}

private struct RouteDiagram: View {
    var body: some View {
        HStack(spacing: 8) {
            RouteNode(icon: "desktopcomputer", label: "局域网设备")
            RouteLine(color: Theme.lime)
            RouteNode(icon: "server.rack", label: "旁路由")
            RouteLine(color: Theme.cyan)
            RouteNode(icon: "cloud", label: "上游代理")
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
                        HStack { Text("运行日志").sectionLabel(); Spacer(); Button("刷新") { model.reloadLog() }.buttonStyle(.bordered); Button("在访达中显示") { model.revealLog() }.buttonStyle(.bordered) }
                        ScrollView([.horizontal, .vertical]) {
                            Text(model.logText).font(.system(size: 11, design: .monospaced)).foregroundStyle(Color.primary.opacity(0.78)).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .topLeading)
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

private struct RouteNode: View {
    let icon: String, label: String
    var body: some View { VStack(spacing: 8) { ZStack { RoundedRectangle(cornerRadius: 7).fill(Theme.panelRaised); Image(systemName: icon).font(.title2).foregroundStyle(Theme.cyan) }.frame(width: 62, height: 62); Text(label).font(.caption2).foregroundStyle(Theme.muted) } }
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

private extension Array where Element == String {
    func uniqued() -> [String] {
        reduce(into: []) { result, value in
            if !result.contains(value) { result.append(value) }
        }
    }
}
