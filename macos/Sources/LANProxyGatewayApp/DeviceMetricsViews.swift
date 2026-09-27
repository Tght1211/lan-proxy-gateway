import Charts
import SwiftUI

struct DeviceRanking: View {
    @EnvironmentObject private var model: AppModel
    private struct Selection: Identifiable {
        let id: String
    }
    @State private var selection: Selection?

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
                    Button("按设备查看代理用量") { selection = Selection(id: "") }
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
                                        Button { selection = Selection(id: item.name) } label: {
                                            Text(item.name).font(.system(size: 13, weight: .medium, design: .monospaced))
                                                .lineLimit(1).fixedSize(horizontal: true, vertical: false)
                                        }.buttonStyle(.plain).foregroundStyle(Theme.cyan)
                                            .help("查看此设备的代理用量、直连域名和访问明细")
                                    }
                                    .frame(maxWidth: .infinity, alignment: .leading)
									let ov = model.deviceOverride(for: item.name)
									let adaptive = model.adaptiveDeviceState(for: item.name)
									let protected = ov.isEmpty && adaptive?.mode == "direct"
									let observing = ov.isEmpty && adaptive?.mode == "observing"
                                    Menu {
                                        Button("默认（跟随规则）") { model.setDeviceOverride(item.name, action: "") }
                                        Button("强制直连") { model.setDeviceOverride(item.name, action: "direct") }
                                        Button("强制代理") { model.setDeviceOverride(item.name, action: "proxy") }
										Button("拒绝联网") { model.setDeviceOverride(item.name, action: "reject") }
                                    } label: {
                                        HStack(spacing: 3) {
											Circle().fill((protected || observing) ? Theme.yellow : (ov.isEmpty ? Theme.muted : (ov == "direct" ? Theme.lime : (ov == "reject" ? Theme.coral : Theme.cyan)))).frame(width: 6, height: 6)
											Text(protected ? "保护直连" : (observing ? "观察 \(adaptive?.failureCount ?? 0)/\(model.stats?.deviceAdaptive?.threshold ?? 5)" : (ov.isEmpty ? "默认" : (ov == "direct" ? "直连" : (ov == "reject" ? "拒绝" : "代理")))))
                                                .font(.caption2.weight(.medium))
                                        }
                                    }
                                    .menuStyle(.borderlessButton)
                                    .menuIndicator(.hidden)
                                    .frame(width: 64, alignment: .leading)
									.help(protected ? "检测到短时间多目标代理失败，已临时切换整台设备直连；手动策略优先级更高" : (observing ? "正在聚合同一设备的不同失败目标，达到阈值后仅将该设备临时切换直连" : "设备级前置开关：优先级高于域名规则"))
                                    TextField(
                                        model.autoDeviceLabels[item.name].map { "\($0)（自动识别）" } ?? "例如：客厅 Switch",
                                        text: Binding(
                                            get: { model.deviceLabel(for: item.name) },
                                            set: { model.setDeviceLabel($0, for: item.name) }
                                        )
                                    )
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
        .sheet(item: $selection) { selected in
            DeviceTrafficSheet(device: selected.id).environmentObject(model)
        }
    }
}

struct CompactSetupValue: View {
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

struct StabilityChart: View {
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

struct StabilitySummary: View {
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
            .overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(hovering ? Theme.cyan.opacity(0.5) : Color.clear, lineWidth: 1))
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .help("点击配置网络出口")
        .sheet(isPresented: $showProxyConfig) {
            ProxyConfigSheet().environmentObject(model)
        }
    }
}

struct ProbeHistoryStrip: View {
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
