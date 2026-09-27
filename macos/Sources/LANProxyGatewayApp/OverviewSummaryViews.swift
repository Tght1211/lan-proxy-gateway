import Charts
import SwiftUI

struct GatewaySummary: View {
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
                if model.stats?.udpRelay != nil {
                    SummaryFact("UDP 中继", "已启用")
                }
                ExitIdentityFact(identity: model.stats?.health.egressIdentity)
                SummaryFact("运行时间", uptime(model.stats?.uptimeSec ?? 0))
            }
            if let comps = model.stats?.components, comps.contains(where: { $0.crashes > 0 }) {
                Divider().overlay(Theme.border)
                HStack(spacing: 8) {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .font(.caption).foregroundStyle(Theme.yellow)
                    Text("子服务自愈").font(.caption2.weight(.semibold)).foregroundStyle(Theme.yellow)
                    ForEach(comps.filter { $0.crashes > 0 }) { c in
                        HStack(spacing: 3) {
                            Circle().fill(c.running ? Theme.lime : Theme.coral).frame(width: 6, height: 6)
                            Text("\(c.name)")
                                .font(.caption2.weight(.medium).monospaced())
                            Text("重启\(c.crashes)次")
                                .font(.caption2).foregroundStyle(Theme.muted)
                        }
                        .padding(.horizontal, 6).padding(.vertical, 2)
                        .background(Theme.yellow.opacity(0.08))
                        .clipShape(RoundedRectangle(cornerRadius: 4))
                    }
                    Spacer()
                }
            }
        }
    }
}

struct ExitIdentityFact: View {
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

struct SummaryFact: View {
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

struct CoreHero: View {
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

struct GettingStartedPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var showProxyConfig = false
    @State private var showOnboarding = false

    var body: some View {
        Panel {
            HStack(spacing: 18) {
                ZStack {
                    RoundedRectangle(cornerRadius: Theme.radius).fill(Theme.cyan.opacity(0.14))
                    Image(systemName: "point.3.connected.trianglepath.dotted")
                        .font(.system(size: 25, weight: .semibold)).foregroundStyle(Theme.cyan)
                }
                .frame(width: 54, height: 54)
                VStack(alignment: .leading, spacing: 5) {
                    Text("首次使用").font(.system(size: 15, weight: .semibold))
                    Text("Mac 接网线，游戏机连代理 Wi-Fi。跟着引导设置一次，之后自动连接。")
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
                    showOnboarding = true
                } label: {
                    Label("连接游戏机", systemImage: "wifi")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.lime))
            }
        }
        .frame(minHeight: 88)
        .sheet(isPresented: $showOnboarding) {
            DeviceOnboardingSheet().environmentObject(model)
        }
        .sheet(isPresented: $showProxyConfig) {
            ProxyConfigSheet().environmentObject(model)
        }
    }
}

struct ThroughputChart: View {
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
                    LineMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5), series: .value("方向", "下载"))
                        .foregroundStyle(Theme.cyan).lineStyle(StrokeStyle(lineWidth: 2)).interpolationMethod(.linear)
                    LineMark(x: .value("时间", point.at), y: .value("上传", Double(point.up) / 5), series: .value("方向", "上传"))
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

struct ServiceUsagePanel: View {
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
                        Text(model.effectiveDeviceLabel(for: group.device).nonEmpty.map { "\($0) · \(group.device)" } ?? group.device)
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
