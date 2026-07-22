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
        switch model.selectedSection ?? .overview {
        case .overview: OverviewView()
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
        ScrollView(.vertical, showsIndicators: false) {
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

private struct ServicesView: View {
    @EnvironmentObject private var model: AppModel
    @State private var selectedDevice = "全部设备"

    var body: some View {
        let deviceGroups = model.stats?.relay.deviceServices ?? []
        let filtered = selectedDevice == "全部设备"
            ? (model.stats?.relay.services ?? [])
            : (deviceGroups.first { $0.device == selectedDevice }?.services ?? [])
        VStack(spacing: 14) {
            HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 6) {
                    Text("统计范围").fieldLabel()
                    Picker("统计范围", selection: $selectedDevice) {
                        Text("全部设备").tag("全部设备")
                        ForEach(deviceGroups) { group in
                            Text(model.deviceLabel(for: group.device).nonEmpty.map { "\($0) · \(group.device)" } ?? group.device)
                                .tag(group.device)
                        }
                    }
                    .labelsHidden().frame(width: 250)
                }
                MetricCard("已识别服务", "\(filtered.count)", "square.stack.3d.up.fill", Theme.cyan)
                MetricCard("访问最多", filtered.first?.displayName ?? "--", "crown.fill", Theme.yellow)
                MetricCard("服务流量", bytes(filtered.reduce(0) { $0 + $1.total }), "chart.bar.fill", Theme.coral)
            }
            DeviceTopServices(groups: deviceGroups)
            FilteredServiceList(data: filtered, scope: selectedDevice)
        }
        .padding(20)
        .onChange(of: deviceGroups.map(\.device)) { devices in
            if selectedDevice != "全部设备", !devices.contains(selectedDevice) { selectedDevice = "全部设备" }
        }
    }
}

private struct DeviceTopServices: View {
    @EnvironmentObject private var model: AppModel
    let groups: [DeviceServiceAggregate]

    var body: some View {
        HStack(spacing: 10) {
            ForEach(groups.prefix(5)) { group in
                VStack(alignment: .leading, spacing: 8) {
                    Text(model.deviceLabel(for: group.device).nonEmpty ?? group.device)
                        .font(.system(size: 12, weight: .semibold)).lineLimit(1)
                    Text(group.device).font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted)
                    ForEach(Array(group.services.prefix(3))) { service in
                        HStack(spacing: 6) {
                            Circle().fill(serviceColor(service.name)).frame(width: 5, height: 5)
                            Text(service.displayName).font(.caption).lineLimit(1)
                            Spacer(minLength: 4)
                            Text(shortBytes(service.total)).font(.caption2).foregroundStyle(Theme.muted)
                        }
                    }
                    if group.services.isEmpty { Text("暂无流量").font(.caption).foregroundStyle(Theme.muted) }
                }
                .padding(12)
                .frame(maxWidth: .infinity, minHeight: 108, alignment: .topLeading)
                .background(Theme.panel)
                .overlay(RoundedRectangle(cornerRadius: 7).stroke(Theme.border, lineWidth: 0.7))
                .clipShape(RoundedRectangle(cornerRadius: 7))
            }
            if groups.isEmpty {
                Text("等待设备服务数据").font(.caption).foregroundStyle(Theme.muted)
                    .frame(maxWidth: .infinity, minHeight: 108)
            }
        }
    }
}

private struct FilteredServiceList: View {
    let data: [UsageAggregate]
    let scope: String

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Text(scope == "全部设备" ? "全部设备服务流量" : "设备服务明细").sectionLabel()
                    Spacer()
                    Text(scope).font(.caption).foregroundStyle(Theme.muted)
                }
                if data.isEmpty {
                    EmptyTelemetry(icon: "square.stack.3d.up", text: "等待服务流量")
                } else {
                    let maximum = max(data.first?.total ?? 1, 1)
                    ScrollView(.vertical, showsIndicators: false) {
                        VStack(spacing: 10) {
                            ForEach(data) { item in
                                VStack(spacing: 5) {
                                    HStack {
                                        Text(item.displayName).font(.system(size: 12, weight: .medium))
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
        VStack(spacing: 14) {
            DeviceAccessSummary()
            LabeledDevicesStrip()
            DeviceRanking()
            SuggestedAddressPanel()
        }
        .padding(20)
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

    var body: some View {
        let suggestions = suggestedDeviceIPs(
            gateway: model.status?.gateway.localIP ?? "",
            occupied: Set(model.stats?.relay.devices.map(\.name) ?? [])
        )
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
                    Text("其中 \(model.activeDeviceCount) 台正在产生连接")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                CompactSetupValue(
                    label: "设备 IP 候选",
                    value: suggestionsRange(suggestions)
                )
                CompactSetupValue(label: "网关与 DNS", value: model.status?.gateway.localIP.nonEmpty ?? "--", copyable: true)
                CompactSetupValue(label: "子网掩码", value: "255.255.255.0", copyable: true)
            }
        }
    }
}

private struct SuggestedAddressPanel: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let suggestions = suggestedDeviceIPs(
            gateway: model.status?.gateway.localIP ?? "",
            occupied: Set(model.stats?.relay.devices.map(\.name) ?? [])
        )
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 3) {
                        Text("设备接入指南").sectionLabel()
                        Text("先选择候选 IP，再在设备中手动填写网络参数")
                            .font(.caption).foregroundStyle(Theme.muted)
                    }
                    Spacer()
                    Label("使用前请确认不在路由器 DHCP 地址池内", systemImage: "exclamationmark.triangle")
                        .font(.caption).foregroundStyle(Theme.yellow)
                }

                if suggestions.isEmpty {
                    Text("暂时无法根据当前网关地址生成候选，请先确认网关网络配置。")
                        .font(.caption).foregroundStyle(Theme.muted).padding(.vertical, 12)
                } else {
                    HStack(spacing: 0) {
                        GuideStep(number: "1", title: "选择手动 / 静态 IP", detail: "进入设备的网络或互联网设置")
                        Image(systemName: "chevron.right").foregroundStyle(Theme.border)
                        GuideStep(number: "2", title: "填写候选 IP", detail: "从下方选择一个地址")
                        Image(systemName: "chevron.right").foregroundStyle(Theme.border)
                        GuideStep(number: "3", title: "填写网关和 DNS", detail: model.status?.gateway.localIP.nonEmpty ?? "--")
                        Image(systemName: "chevron.right").foregroundStyle(Theme.border)
                        GuideStep(number: "4", title: "保存并测试", detail: "代理保持关闭或不填写")
                    }
                    HStack(spacing: 10) {
                        ForEach(Array(suggestions.enumerated()), id: \.element) { index, address in
                            Button {
                                NSPasteboard.general.clearContents()
                                NSPasteboard.general.setString(address, forType: .string)
                            } label: {
                                HStack(spacing: 10) {
                                    Text("\(index + 1)")
                                        .font(.caption2.weight(.bold)).foregroundStyle(Theme.cyan)
                                        .frame(width: 22, height: 22)
                                        .background(Theme.cyan.opacity(0.10))
                                        .clipShape(RoundedRectangle(cornerRadius: 5))
                                    Text(address)
                                        .font(.system(size: 12, weight: .semibold, design: .monospaced))
                                    Spacer(minLength: 4)
                                    Image(systemName: "doc.on.doc").foregroundStyle(Theme.cyan)
                                }
                                .padding(.horizontal, 10)
                                .frame(maxWidth: .infinity, minHeight: 42)
                                .background(Theme.panelRaised)
                                .overlay(RoundedRectangle(cornerRadius: 6).stroke(Theme.border, lineWidth: 0.7))
                                .clipShape(RoundedRectangle(cornerRadius: 6))
                            }
                            .buttonStyle(.plain)
                            .help("复制 \(address)")
                        }
                    }
                }
            }
        }
    }
}

private struct GuideStep: View {
    let number: String, title: String, detail: String
    var body: some View {
        HStack(spacing: 9) {
            Text(number).font(.caption2.weight(.bold)).foregroundStyle(Color.white)
                .frame(width: 22, height: 22).background(Theme.cyan).clipShape(Circle())
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.caption.weight(.semibold)).lineLimit(1)
                Text(detail).font(.caption2).foregroundStyle(Theme.muted).lineLimit(1)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
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
                        Text("标签备注").frame(width: 150, alignment: .leading)
                        Text("状态").frame(width: 80, alignment: .leading)
                        Text("连接").frame(width: 80, alignment: .trailing)
                        Text("流量").frame(width: 100, alignment: .trailing)
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
                                    }
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                    TextField("例如：客厅 Switch", text: Binding(
                                        get: { model.deviceLabel(for: item.name) },
                                        set: { model.setDeviceLabel($0, for: item.name) }
                                    ))
                                    .textFieldStyle(.plain)
                                    .font(.caption)
                                    .frame(width: 150)
                                    Text(activeIPs.contains(item.name) ? "正在使用" : "最近使用")
                                        .font(.caption).foregroundStyle(activeIPs.contains(item.name) ? Theme.lime : Theme.muted)
                                        .frame(width: 80, alignment: .leading)
                                    Text("\(item.connections)").font(.system(.caption, design: .monospaced))
                                        .frame(width: 80, alignment: .trailing)
                                    Text(shortBytes(item.total)).font(.system(.caption, design: .monospaced))
                                        .frame(width: 100, alignment: .trailing)
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
        .frame(minWidth: 150, alignment: .leading)
    }
}

private struct StabilityView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        VStack(spacing: 16) {
            HStack(spacing: 12) {
                MetricCard("平均延迟", formatMS(model.stats?.health.latencyMS), "timer", Theme.cyan)
                MetricCard("网络抖动", formatMS(model.stats?.health.jitterMS), "waveform.path", Theme.yellow)
                MetricCard("可用率", String(format: "%.1f%%", model.stats?.health.availability ?? 0), "checkmark.shield.fill", Theme.lime)
                MetricCard("连续失败", "\(model.stats?.health.failCount ?? 0)", "exclamationmark.triangle.fill", Theme.coral)
            }
            HStack(alignment: .top, spacing: 16) {
                StabilityChart().frame(maxWidth: .infinity, maxHeight: .infinity)
                StabilitySummary().frame(width: 330)
            }
        }
        .padding(20)
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
    }
}

private struct ProbeHistoryStrip: View {
    let points: [ProbePoint]
    let average: Double

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("最近 \(points.count) 次探测").font(.caption).foregroundStyle(Theme.muted)
                Spacer()
                Text("实时更新").font(.caption2).foregroundStyle(Theme.muted)
            }
            HStack(alignment: .bottom, spacing: 2) {
                ForEach(Array(points.enumerated()), id: \.offset) { _, point in
                    Capsule()
                        .fill(probeColor(point))
                        .frame(maxWidth: .infinity, minHeight: 22, maxHeight: 34)
                        .help(point.ok ? formatMS(point.latencyMS) : "探测失败")
                }
                if points.isEmpty {
                    Text("等待探测记录").font(.caption2).foregroundStyle(Theme.muted)
                }
            }
            HStack {
                Text("过去"); Spacer(); Text("现在")
            }.font(.system(size: 9, weight: .medium)).foregroundStyle(Theme.muted)
        }
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
    @State private var statusFilter = "全部"
    @State private var deviceFilter = "全部设备"
    @State private var routeFilter = "全部出口"
    @State private var unresolvedOnly = false

    private var connections: [ConnectionInfo] {
        let all = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
        return all.filter { item in
            let label = model.deviceLabel(for: item.srcIP)
            let matchesSearch = search.isEmpty || item.srcIP.localizedCaseInsensitiveContains(search) ||
                label.localizedCaseInsensitiveContains(search) || item.dstHost.localizedCaseInsensitiveContains(search) ||
                item.service.localizedCaseInsensitiveContains(search)
            let matchesStatus = statusFilter == "全部" || (statusFilter == "活跃" ? item.endedAt == nil : item.endedAt != nil)
            let matchesDevice = deviceFilter == "全部设备" || item.srcIP == deviceFilter
            let matchesRoute = routeFilter == "全部出口" || (routeFilter == "代理" ? item.viaProxy : !item.viaProxy)
            let matchesResolution = !unresolvedOnly || item.service == "未解析域名"
            return matchesSearch && matchesStatus && matchesDevice && matchesRoute && matchesResolution
        }
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
                Picker("状态", selection: $statusFilter) {
                    Text("全部").tag("全部"); Text("活跃").tag("活跃"); Text("完成").tag("完成")
                }.labelsHidden().pickerStyle(.segmented).frame(width: 180)
                Picker("设备", selection: $deviceFilter) {
                    Text("全部设备").tag("全部设备")
                    ForEach(devices, id: \.self) { ip in
                        Text(model.deviceLabel(for: ip).nonEmpty ?? ip).tag(ip)
                    }
                }.labelsHidden().frame(width: 150)
                Picker("出口", selection: $routeFilter) {
                    Text("全部出口").tag("全部出口"); Text("代理").tag("代理"); Text("直连").tag("直连")
                }.labelsHidden().frame(width: 120)
                Toggle("仅未解析域名", isOn: $unresolvedOnly).toggleStyle(.checkbox).font(.caption)
                Image(systemName: "info.circle").foregroundStyle(Theme.muted)
                    .help("设备直接连接 IP，或 DNS 映射不可用时无法还原域名。仍会记录目标 IP、端口、流量、时间和出口；HTTPS 加密下无法识别具体操作内容。")
                Spacer(minLength: 8)
                Text("\(connections.count) 条记录").font(.caption).foregroundStyle(Theme.muted)
            }
            .padding(.horizontal, 16).frame(height: 44).background(Theme.panel)

            Table(connections) {
                TableColumn("状态") { item in
                    HStack(spacing: 6) { Circle().fill(item.endedAt == nil ? Theme.lime : Theme.muted).frame(width: 6, height: 6); Text(item.endedAt == nil ? "活跃" : "完成") }
                }.width(70)
                TableColumn("设备") { item in
                    VStack(alignment: .leading, spacing: 1) {
                        if let label = model.deviceLabel(for: item.srcIP).nonEmpty {
                            Text(label).fontWeight(.medium)
                        }
                        Text(item.srcIP).font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                    }
                }.width(min: 125, ideal: 155)
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
    @State private var showRoutingEditor = false

    var body: some View {
        VStack(spacing: 16) {
                Panel {
                    VStack(alignment: .leading, spacing: 18) {
                        HStack(alignment: .top, spacing: 16) {
                            VStack(alignment: .leading, spacing: 5) {
                                Text("当前网络出口").sectionLabel()
                                Text(activeExitTitle)
                                    .font(.system(size: 22, weight: .semibold, design: .rounded))
                                    .lineLimit(1)
                                    .minimumScaleFactor(0.72)
                                Text(activeExitSubtitle)
                                    .font(.caption)
                                    .foregroundStyle(Theme.muted)
                            }
                            Spacer()
                            Button { showRoutingEditor = true } label: {
                                Label("管理分流规则", systemImage: "list.bullet.rectangle")
                            }.buttonStyle(.bordered)
                            LiveBadge(active: model.isRunning && (model.stats?.health.healthy ?? false))
                        }

                        RouteDiagram(
                            active: model.isRunning,
                            gateway: model.status?.gateway.localIP.nonEmpty ?? "--",
                            upstream: activeEndpoint,
                            devices: Array((model.stats?.relay.devices ?? []).prefix(3)),
                            labels: model.deviceLabels,
                            rules: model.status?.routing ?? []
                        )

                        Divider().overlay(Theme.border)
                        HStack(spacing: 0) {
                            ExitFact(label: "出口模式", value: model.status?.egress == "proxy" ? "代理转发" : "直接连接", icon: "arrow.triangle.branch")
                            ExitFact(label: "公网 IP", value: model.stats?.health.egressIdentity?.ip ?? "正在检测", icon: "globe.asia.australia")
                            ExitFact(label: "出口地区", value: egressLocation(model.stats?.health.egressIdentity), icon: "mappin.and.ellipse")
                            ExitFact(label: "网络服务商", value: model.stats?.health.egressIdentity?.isp?.nonEmpty ?? "--", icon: "building.2")
                        }
                        ProbeHistoryStrip(
                            points: Array((model.stats?.health.history ?? []).suffix(60)),
                            average: model.stats?.health.latencyMS ?? 0
                        )
                    }
                }

                Panel {
                    VStack(alignment: .leading, spacing: 16) {
                        HStack {
                            VStack(alignment: .leading, spacing: 3) {
                                Text("修改上游代理").sectionLabel()
                                Text("修改后会立即应用到局域网设备的新连接")
                                    .font(.caption).foregroundStyle(Theme.muted)
                            }
                            Spacer()
                            Button("切换直连") { model.useDirectConnection() }
                                .buttonStyle(.bordered)
                                .disabled(model.isBusy || model.status?.egress != "proxy")
                        }
                        HStack(alignment: .bottom, spacing: 12) {
                            VStack(alignment: .leading, spacing: 7) {
                                Text("代理协议").fieldLabel()
                                Picker("", selection: $model.proxyType) {
                                    Text("SOCKS5").tag("socks5")
                                    Text("HTTP CONNECT").tag("http")
                                }
                                .labelsHidden()
                                .pickerStyle(.segmented)
                                .tint(Theme.cyan)
                                .frame(width: 260)
                            }
                            VStack(alignment: .leading, spacing: 7) {
                                Text("代理地址").fieldLabel()
                                TextField("127.0.0.1", text: $model.proxyHost)
                                    .textFieldStyle(DarkFieldStyle())
                            }
                            VStack(alignment: .leading, spacing: 7) {
                                Text("端口").fieldLabel()
                                TextField("7897", value: $model.proxyPort, format: .number)
                                    .textFieldStyle(DarkFieldStyle())
                                    .frame(width: 130)
                            }
                            Button(model.isConfigured ? "应用代理" : "保存配置") { model.applyProxy() }
                                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                                .frame(height: 36)
                                .disabled(model.isBusy)
                        }
                    }
                }
        }.padding(20)
        .sheet(isPresented: $showRoutingEditor) {
            RoutingRulesEditor(rules: model.status?.routing ?? [])
                .environmentObject(model)
        }
    }

    private var activeEndpoint: String {
        model.status?.egress == "proxy" ? (model.status?.proxy?.nonEmpty ?? "代理未配置") : "DIRECT"
    }

    private var activeExitTitle: String {
        model.status?.egress == "proxy" ? activeEndpoint : "DIRECT 直连"
    }

    private var activeExitSubtitle: String {
        guard model.status?.egress == "proxy" else { return "局域网流量不经过上游代理" }
        let count = model.status?.routing?.count ?? 0
        return count == 0 ? "未配置分流规则，所有新建 TCP 连接通过此上游代理" : "按顺序匹配 \(count) 条规则，未命中时使用此上游代理"
    }
}

private struct RouteDiagram: View {
    let active: Bool
    let gateway: String
    let upstream: String
    let devices: [UsageAggregate]
    let labels: [String: String]
    let rules: [RoutingRule]

    var body: some View {
        HStack(spacing: 10) {
            DeviceRouteCluster(devices: devices, labels: labels)
            AnimatedRouteLine(color: Theme.lime, active: active)
            RouteNode(icon: "server.rack", label: "旁路由", detail: gateway)
            AnimatedRouteLine(color: Theme.cyan, active: active)
            RouteNode(icon: "arrow.triangle.branch", label: "规则判断", detail: "\(rules.count) 条 · 首条命中")
            AnimatedRouteBranch(active: active)
            VStack(spacing: 7) {
                RouteOutcome(icon: "cloud", title: "上游代理", detail: upstream, count: ruleCount("proxy"), color: Theme.cyan)
                RouteOutcome(icon: "network", title: "本机直连", detail: "DIRECT", count: ruleCount("direct"), color: Theme.lime)
                RouteOutcome(icon: "xmark.octagon", title: "拒绝", detail: "REJECT", count: ruleCount("reject"), color: Theme.coral)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 8)
    }

    private func ruleCount(_ action: String) -> Int { rules.filter { $0.action == action }.count }
}

private struct AnimatedRouteBranch: View {
    let active: Bool
    var body: some View {
        TimelineView(.animation(minimumInterval: 1.0 / 30.0, paused: !active)) { timeline in
            Canvas { context, size in
                let start = CGPoint(x: 0, y: size.height / 2)
                let ends = [CGPoint(x: size.width, y: 19), CGPoint(x: size.width, y: size.height / 2), CGPoint(x: size.width, y: size.height - 19)]
                let progress = timeline.date.timeIntervalSinceReferenceDate.truncatingRemainder(dividingBy: 1.4) / 1.4
                for (index, end) in ends.enumerated() {
                    var path = Path(); path.move(to: start); path.addLine(to: end)
                    let color = [Theme.cyan, Theme.lime, Theme.coral][index]
                    context.stroke(path, with: .color(color.opacity(0.28)), lineWidth: 1.5)
                    let point = CGPoint(x: start.x + (end.x - start.x) * progress, y: start.y + (end.y - start.y) * progress)
                    context.fill(Path(ellipseIn: CGRect(x: point.x - 3, y: point.y - 3, width: 6, height: 6)), with: .color(color))
                }
            }
        }
        .frame(width: 52, height: 128)
        .accessibilityLabel("规则流量分支")
    }
}

private struct DeviceRouteCluster: View {
    let devices: [UsageAggregate]
    let labels: [String: String]
    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text("局域网设备").font(.caption.weight(.semibold))
            if devices.isEmpty {
                Text("等待设备接入").font(.caption2).foregroundStyle(Theme.muted)
            } else {
                ForEach(devices) { device in
                    HStack(spacing: 6) {
                        Image(systemName: "desktopcomputer").foregroundStyle(Theme.lime)
                        VStack(alignment: .leading, spacing: 0) {
                            if let label = labels[device.name]?.nonEmpty { Text(label).font(.caption2.weight(.semibold)) }
                            Text(device.name).font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted)
                        }
                    }
                }
            }
        }
        .padding(10).frame(width: 155).frame(minHeight: 96, alignment: .leading)
        .background(Theme.panelRaised).clipShape(RoundedRectangle(cornerRadius: 7))
    }
}

private struct RouteOutcome: View {
    let icon: String, title: String, detail: String
    let count: Int
    let color: Color
    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: icon).foregroundStyle(color).frame(width: 18)
            VStack(alignment: .leading, spacing: 1) {
                Text(title).font(.caption.weight(.semibold))
                Text(detail).font(.system(size: 8, design: .monospaced)).foregroundStyle(Theme.muted).lineLimit(1)
            }
            Spacer(minLength: 5)
            Text("\(count)").font(.caption2.weight(.bold)).foregroundStyle(color)
                .frame(minWidth: 20, minHeight: 20).background(color.opacity(0.1)).clipShape(Circle())
        }
        .padding(.horizontal, 9).frame(width: 190, height: 38)
        .background(Theme.panelRaised).clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

private struct RoutingRulesEditor: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var draft: [RoutingRule]

    init(rules: [RoutingRule]) { _draft = State(initialValue: rules) }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                VStack(alignment: .leading, spacing: 3) {
                    Text("分流规则").font(.title2.weight(.semibold))
                    Text("按顺序匹配第一条规则；局域网网关无法识别远端设备的 PROCESS-NAME。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                Button { addRule() } label: { Label("添加规则", systemImage: "plus") }.buttonStyle(.bordered)
            }.padding(20)
            Divider()
            ScrollView(.vertical, showsIndicators: false) {
                VStack(spacing: 8) {
                    ForEach($draft) { $rule in
                        HStack(spacing: 10) {
                            Image(systemName: "line.3.horizontal").foregroundStyle(Theme.muted)
                            Picker("类型", selection: $rule.type) {
                                Text("完整域名").tag("domain")
                                Text("域名后缀").tag("domain-suffix")
                            }.labelsHidden().frame(width: 125)
                            TextField("例如 openai.com", text: $rule.value).textFieldStyle(DarkFieldStyle())
                            Picker("动作", selection: $rule.action) {
                                Text("上游代理").tag("proxy")
                                Text("本机直连").tag("direct")
                                Text("拒绝").tag("reject")
                            }.labelsHidden().frame(width: 120)
                            Button(role: .destructive) { draft.removeAll { $0.id == rule.id } } label: {
                                Image(systemName: "trash")
                            }.buttonStyle(.plain)
                        }
                    }
                    if draft.isEmpty {
                        EmptyTelemetry(icon: "arrow.triangle.branch", text: "暂无规则，全部流量使用默认出口")
                            .frame(minHeight: 220)
                    }
                }.padding(20)
            }
            Divider()
            HStack {
                Text("PROXY \(count("proxy")) · DIRECT \(count("direct")) · REJECT \(count("reject"))")
                    .font(.caption).foregroundStyle(Theme.muted)
                Spacer()
                Button("取消") { dismiss() }.buttonStyle(.bordered)
                Button("应用规则") {
                    model.applyRoutingRules(draft)
                    dismiss()
                }.buttonStyle(ActionButtonStyle(tint: Theme.cyan)).disabled(hasInvalidRule || model.isBusy)
            }.padding(20)
        }
        .frame(width: 760, height: 520).background(Theme.canvas)
    }

    private var hasInvalidRule: Bool { draft.contains { $0.value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty } }
    private func count(_ action: String) -> Int { draft.filter { $0.action == action }.count }
    private func addRule() { draft.append(RoutingRule(type: "domain-suffix", value: "", action: "proxy")) }
}

private struct SettingsView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        VStack(spacing: 16) {
            Panel {
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
                }
            }
            .frame(maxHeight: .infinity)
        }
        .padding(20)
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

private struct RouteNode: View {
    let icon: String, label: String, detail: String
    var body: some View {
        VStack(spacing: 8) {
            ZStack {
                RoundedRectangle(cornerRadius: 7).fill(Theme.panelRaised)
                Image(systemName: icon).font(.title2).foregroundStyle(Theme.cyan)
            }
            .frame(width: 58, height: 52)
            Text(label).font(.caption.weight(.medium))
            Text(detail).font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                .lineLimit(1).minimumScaleFactor(0.7)
        }
        .frame(width: 150)
    }
}

private struct AnimatedRouteLine: View {
    let color: Color
    let active: Bool

    var body: some View {
        TimelineView(.animation(minimumInterval: 1.0 / 30.0, paused: !active)) { timeline in
            GeometryReader { geometry in
                let width = max(geometry.size.width, 1)
                let phase = timeline.date.timeIntervalSinceReferenceDate
                    .truncatingRemainder(dividingBy: 1.35) / 1.35
                ZStack(alignment: .leading) {
                    Capsule().fill(color.opacity(active ? 0.18 : 0.10)).frame(height: 2)
                    ForEach(0..<3, id: \.self) { index in
                        let progress = (phase + Double(index) / 3).truncatingRemainder(dividingBy: 1)
                        Circle().fill(color.opacity(active ? 0.9 : 0.25))
                            .frame(width: 6, height: 6)
                            .offset(x: max(0, width - 6) * progress)
                    }
                    Image(systemName: "chevron.right")
                        .font(.system(size: 9, weight: .bold)).foregroundStyle(color.opacity(0.7))
                        .frame(maxWidth: .infinity, alignment: .trailing)
                }
                .frame(maxHeight: .infinity)
            }
        }
        .frame(minWidth: 90, maxWidth: .infinity, minHeight: 12, maxHeight: 12)
        .accessibilityLabel(active ? "流量正在转发" : "链路当前未运行")
    }
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

private extension Array where Element == String {
    func uniqued() -> [String] {
        reduce(into: []) { result, value in
            if !result.contains(value) { result.append(value) }
        }
    }
}
