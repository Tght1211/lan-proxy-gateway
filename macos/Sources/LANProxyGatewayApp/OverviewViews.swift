import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct OverviewView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        ScrollPage {
            if model.status?.configured == false {
                GettingStartedPanel()
            }
            GatewaySummary()
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 12), count: model.stats?.udpRelay != nil ? 5 : 4), spacing: 12) {
                MetricCard("实时下载", speed(model.stats?.relay.traffic.last?.down ?? 0), "arrow.down", Theme.cyan)
                MetricCard("实时上传", speed(model.stats?.relay.traffic.last?.up ?? 0), "arrow.up", Theme.yellow)
                MetricCard("活动连接", "\(model.stats?.relay.active.count ?? 0)", "point.3.connected.trianglepath.dotted", Theme.lime)
                if let udp = model.stats?.udpRelay {
                    MetricCard("UDP 会话", "\(udp.sessions)", "waveform.path", Theme.yellow)
                }
                MetricCard("活跃设备", "\(model.activeDeviceCount)", "desktopcomputer", Theme.coral)
            }
            TopologyPanel()
            RecentStrip().frame(minHeight: 210)
        }
    }
}

struct TopologyPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var showRoutingEditor = false
    @State private var showProxyConfig = false
    @State private var period = "今日"
    @State private var startDate = Date()
    @State private var endDate = Date()
    @State private var showDevices = false

    private var history: [DailyUsage] {
        let calendar = Calendar.current
        let today = calendar.startOfDay(for: Date())
        let from: Date
        let to: Date
        switch period {
        case "近 7 日": from = calendar.date(byAdding: .day, value: -6, to: today)!; to = today
        case "近一个月": from = calendar.date(byAdding: .month, value: -1, to: today)!; to = today
        case "指定日期": from = calendar.startOfDay(for: startDate); to = from
        case "日期范围": from = calendar.startOfDay(for: startDate); to = calendar.startOfDay(for: endDate)
        default: from = today; to = today
        }
        let formatter = DateFormatter(); formatter.dateFormat = "yyyy-MM-dd"; formatter.locale = Locale(identifier: "en_US_POSIX")
        let lower = formatter.string(from: from), upper = formatter.string(from: to)
        return (model.stats?.usageHistory ?? []).filter { $0.date >= lower && $0.date <= upper }
    }
    private var devices: [UsageAggregate] { aggregateHistory(history, byDevice: true) }


    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    Text("流量拓扑").sectionLabel()
                    Spacer()
                    Text("设备 → 接入方式 → 网关 → ①设备策略 → ②域名规则 → 出口").font(.caption).foregroundStyle(Theme.muted)
                    Button { showRoutingEditor = true } label: {
                        Label("管理规则", systemImage: "slider.horizontal.3").font(.caption)
                    }
                    .buttonStyle(.bordered).controlSize(.small)
                    LiveBadge(active: model.isRunning && (model.stats?.health.healthy ?? false))
                }
                HStack(spacing: 12) {
                    Picker("统计范围", selection: $period) {
                        ForEach(["今日", "近 7 日", "近一个月", "指定日期", "日期范围"], id: \.self) { Text($0).tag($0) }
                    }.frame(width: 190)
                    if period == "指定日期" || period == "日期范围" {
                        DatePicker("开始", selection: $startDate, in: ...Date(), displayedComponents: .date).labelsHidden()
                    }
                    if period == "日期范围" {
                        Text("至").foregroundStyle(Theme.muted)
                        DatePicker("结束", selection: $endDate, in: startDate...max(startDate, Date()), displayedComponents: .date).labelsHidden()
                    }
                    Spacer()
                    Button { showDevices = true } label: { Label("\(devices.count) 台接入过的设备", systemImage: "desktopcomputer") }
                    Text("↓ \(shortBytes(history.reduce(0) { $0 + $1.down }))  ↑ \(shortBytes(history.reduce(0) { $0 + $1.up }))")
                        .font(.caption.monospacedDigit()).foregroundStyle(Theme.cyan)
                }
                if model.status?.accessMode == "hotspot" {
                    HStack {
                        Label("代理 Wi-Fi 使用本 App 共用规则", systemImage: "wifi")
                        Spacer()
                        Text("设备策略优先 · 域名规则按顺序匹配 · 未匹配使用默认出口")
                    }.font(.caption).foregroundStyle(Theme.cyan)
                    let connections = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
                    let hotspotConnections = connections.filter { !$0.isHTTPProxy && model.stats?.hotspot?.containsClient($0.srcIP) == true }
                    Text("热点连接结果（当前及最近记录）：代理 \(hotspotConnections.filter { $0.viaProxy && !$0.rejected }.count) · 直连 \(hotspotConnections.filter { !$0.viaProxy && !$0.rejected }.count) · 拒绝 \(hotspotConnections.filter { $0.rejected }.count)。出口结果不代表请求一定成功。")
                        .font(.caption2).foregroundStyle(Theme.muted)
                }
                if model.stats?.usageHistory == nil {
                    Text("历史统计需要更新并重启核心；从启用后开始记录。").font(.caption).foregroundStyle(Theme.muted)
                }
                HStack(alignment: .top, spacing: 18) {
                    RouteDiagram(
                        onEditRules: { showRoutingEditor = true },
                        onEditProxy: { showProxyConfig = true },
                        historicalDevices: devices, historicalIngress: aggregateHistory(history, byDevice: false), historyRows: history,
                        onShowDevices: { showDevices = true }
                    )
                    .frame(maxWidth: .infinity)
                    VStack(spacing: 12) {
                        StabilitySummary()
                        ThroughputChart(compact: true).frame(height: 230)
                    }
                    .frame(width: 300)
                }
            }
        }
        .onChange(of: startDate) { value in if endDate < value { endDate = value } }
        .sheet(isPresented: $showDevices) {
            UsageHistorySheet(devices: devices, period: period).environmentObject(model)
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
struct FallbackLearningBadge: View {
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
                Text("规则建议")
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
        .help("查看回退直连的证据，确认后才保存规则")
        .sheet(isPresented: $showDetail) {
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

struct FallbackLearningPopover: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    let onEditRules: () -> Void
    @State private var tab = "待处理"
    private var candidates: [FallbackCandidate] { model.stats?.fallback?.candidates ?? [] }
    private var learned: [RoutingRule] { model.stats?.fallback?.learned ?? [] }
    private var ignored: [String] { model.stats?.fallback?.ignored ?? [] }
    private var threshold: Int { model.stats?.fallback?.threshold ?? 3 }
    private func service(_ host: String) -> String {
        let records = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
        return records.first { $0.dstHost == host }?.service ?? host
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                Text("规则建议").font(.title2.bold())
                Spacer()
                Button("管理规则", action: onEditRules)
                Button("完成") { dismiss() }
            }
            Text("依据代理失败后的直连响应生成建议；不会自动保存永久规则，也不会覆盖已有规则。").font(.caption).foregroundStyle(Theme.muted)
            Picker("状态", selection: $tab) {
                Text("待处理 \(candidates.count)").tag("待处理")
                Text("已保存 \(learned.count)").tag("已保存")
                Text("已忽略 \(ignored.count)").tag("已忽略")
            }.pickerStyle(.segmented)
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 12) {
                    if tab == "待处理" {
                        if candidates.isEmpty { Text("暂无建议，正常使用网络即可。").foregroundStyle(Theme.muted) }
                        ForEach(Array(Dictionary(grouping: candidates, by: { service($0.host) }).keys.sorted()), id: \.self) { name in
                            Text(name).font(.headline).foregroundStyle(Theme.cyan)
                            ForEach(candidates.filter { service($0.host) == name }) { candidate in
                                VStack(alignment: .leading, spacing: 8) {
                                    Text(candidate.host).textSelection(.enabled)
                                    Text("最近 24 小时有 \(candidate.count) 次回退直连收到响应 · \(relativeTime(candidate.lastAt))")
                                        .font(.caption).foregroundStyle(Theme.muted)
                                    HStack {
                                        Text(candidate.count >= threshold ? "建议此域名直连" : "证据不足，继续观察").font(.caption)
                                        Spacer()
                                        Button("忽略") { Task { await model.learningAction("ignore", host: candidate.host) } }
                                        Button("保存直连规则") { Task { await model.learningAction("accept", host: candidate.host) } }
                                            .disabled(candidate.count < threshold)
                                    }
                                }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                            }
                        }
                    } else if tab == "已保存" {
                        if learned.isEmpty { Text("暂无已保存的学习规则").foregroundStyle(Theme.muted) }
                        ForEach(learned) { rule in
                            HStack {
                                VStack(alignment: .leading) {
                                    Text(rule.value).textSelection(.enabled)
                                    Text(rule.type == "domain" ? "仅此域名 · 直连" : "域名及子域名 · 历史学习规则").font(.caption).foregroundStyle(Theme.muted)
                                }
                                Spacer()
                                Button("撤销") { Task { await model.learningAction("undo", host: rule.value) } }
                            }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                        }
                    } else {
                        if ignored.isEmpty { Text("暂无已忽略的域名").foregroundStyle(Theme.muted) }
                        ForEach(ignored, id: \.self) { host in
                            HStack {
                                Text(host).textSelection(.enabled)
                                Spacer()
                                Button("恢复观察") { Task { await model.learningAction("restore", host: host) } }
                            }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                        }
                    }
                }
            }
            Text("收到响应只证明连接有数据，不保证视频等业务可用；保存后仍可撤销。").font(.caption).foregroundStyle(Theme.muted)
        }.padding(24).frame(width: 680, height: 540).disabled(model.isBusy)
    }
}

struct FlowStep: View {
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
        .overlay(Capsule().stroke(color.opacity(0.28), lineWidth: Theme.borderWidth))
        .clipShape(Capsule())
        .fixedSize()
    }
}

struct FlowArrow: View {
    var body: some View {
        Image(systemName: "chevron.compact.right")
            .font(.system(size: 9, weight: .semibold))
            .foregroundStyle(Theme.muted)
            .frame(maxWidth: .infinity)
    }
}

func relativeTime(_ date: Date) -> String {
    let formatter = RelativeDateTimeFormatter()
    formatter.locale = Locale(identifier: "zh_CN")
    formatter.unitsStyle = .short
    return formatter.localizedString(for: date, relativeTo: Date())
}
