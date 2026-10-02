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
                Text("自动学习")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(isActive ? Color.primary : Theme.muted)
                if !candidates.isEmpty {
                    Text(LearningCountLabel.compact(candidates.count))
                        .font(.system(size: 10, weight: .bold, design: .monospaced))
                        .foregroundStyle(Theme.yellow)
                        .padding(.horizontal, 5).padding(.vertical, 1)
                        .background(Theme.yellow.opacity(0.14))
                        .clipShape(Capsule())
                }
                if !learnedRules.isEmpty {
                    HStack(spacing: 3) {
                        Image(systemName: "checkmark").font(.system(size: 7, weight: .bold))
                        Text(LearningCountLabel.compact(learnedRules.count))
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
            .lineLimit(1).fixedSize(horizontal: true, vertical: false)
            .padding(.horizontal, 10).padding(.vertical, 5)
            .background(isActive ? Theme.yellow.opacity(0.08) : Theme.panel)
            .overlay(Capsule().stroke(isActive ? Theme.yellow.opacity(0.45) : Theme.border, lineWidth: 0.8))
            .clipShape(Capsule())
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .help("已保存 \(learnedRules.count) 条规则，待确认 \(candidates.count) 条；点击搜索、分页查看或暂停学习")
        .accessibilityLabel("自动学习")
        .accessibilityValue("已保存 \(learnedRules.count) 条，待确认 \(candidates.count) 条")
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
    @Environment(\.networkPageActive) private var isActive
    @Environment(\.dismiss) private var dismiss
    let onEditRules: () -> Void
    var embedded = false
    @State private var showSettings = false
    @State private var tab = LearningRecordState.saved.rawValue
    @State private var search = ""
    @State private var service = ""
    @State private var route = ""
    @State private var scope = ""
    @State private var index = LearningRecordsIndex()
    @State private var page = LearningRecordsIndex().page(state: .saved)
    private var threshold: Int { model.stats?.fallback?.threshold ?? 1 }
    private var state: LearningRecordState { LearningRecordState(rawValue: tab) ?? .saved }
    private func showPage(_ number: Int) {
        page = index.page(state: state, search: search, service: service,
                          action: state == .saved ? route : "", ruleType: state == .saved ? scope : "", number: number)
    }
    private func refresh(_ stats: RuntimeStats?) {
        guard isActive else { return }
        let changed = index.update(fallback: stats?.fallback,
                                   connections: (stats?.relay.active ?? []) + (stats?.relay.recent ?? []))
        if changed { showPage(page.number) }
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                if !embedded { Text("自动学习").font(.title2.bold()) }
                Spacer()
                Button("管理规则", action: onEditRules)
                if !embedded { Button("完成") { dismiss() } }
            }
            if !embedded { Button("学习方案与设置") { showSettings = true } }
            Text("未匹配规则的网站先直连；直连失败且代理收到响应后，按你设定的次数和保存方式学习。已有明确规则优先。").font(.caption).foregroundStyle(Theme.muted)
            if model.stats?.fallback?.strategy != "direct-first" || model.stats?.fallback?.settings == nil {
                Text("当前核心尚未支持新版自动学习，请更新并重启核心后管理学习记录。")
                    .font(.caption).foregroundStyle(Theme.yellow)
            }
            StudioTabs(title: "学习记录状态", selection: $tab, items: LearningRecordState.allCases.map {
                ($0.rawValue, "\($0.title) \(LearningCountLabel.compact(index.count(for: $0)))", "")
            }, compact: true)
            HStack(spacing: 12) {
                Picker("服务", selection: $service) {
                    Text("全部服务").tag("")
                    ForEach(index.services(for: state)) { category in
                        Text("\(category.name)（\(category.count)）").tag(category.name)
                    }
                    if !service.isEmpty && !index.services(for: state).contains(where: { $0.name == service }) {
                        Text("\(service)（0）").tag(service)
                    }
                }
                if state == .saved {
                    Picker("路由", selection: $route) {
                        Text("全部路由").tag("")
                        Text("代理").tag("proxy")
                        Text("直连（旧版）").tag("direct")
                        Text("拒绝").tag("reject")
                    }
                    Picker("范围", selection: $scope) {
                        Text("全部范围").tag("")
                        Text("仅此域名").tag("domain")
                        Text("域名及子域名").tag("domain-suffix")
                        Text("IP 网段").tag("ip-cidr")
                        Text("来源设备").tag("src-ip")
                    }
                }
                if !service.isEmpty || !route.isEmpty || !scope.isEmpty {
                    Button("重置筛选") { service = ""; route = ""; scope = "" }
                }
            }.pickerStyle(.menu).font(.caption)
            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.muted)
                TextField("搜索域名、服务或规则分组", text: $search).textFieldStyle(.plain)
                if !search.isEmpty { Button("清除") { search = "" } }
                Text("\(page.matched) / \(page.total) 条").font(.caption).foregroundStyle(Theme.muted).fixedSize()
            }.padding(10).background(Theme.panelRaised).cornerRadius(8)
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 12) {
                    if page.entries.isEmpty {
                        Text(page.total == 0 ? "暂无\(state.title)记录" : "没有匹配的记录，请调整搜索或筛选").foregroundStyle(Theme.muted).padding(.vertical, 16)
                    }
                    ForEach(page.sections) { section in
                        if !section.service.isEmpty {
                            Text(section.service).font(.caption.bold()).foregroundStyle(Theme.cyan).padding(.top, 4)
                        }
                        ForEach(section.entries) { entry in
                            if state == .pending {
                                VStack(alignment: .leading, spacing: 8) {
                                    Text(entry.host).lineLimit(1).help(entry.host).textSelection(.enabled)
                                    Text("\(entry.service) · 代理候选（尚未保存）").font(.caption).foregroundStyle(Theme.muted)
                                    Text("最近 24 小时有 \(entry.confirmations) 次直连失败后代理收到响应 · \(entry.lastAt.map(relativeTime) ?? "时间未记录")")
                                        .font(.caption).foregroundStyle(Theme.muted)
                                    HStack {
                                        Text(entry.confirmations >= threshold ? "已达到保存条件" : "等待有效代理响应").font(.caption)
                                        Spacer()
                                        Button("暂停学习") { Task { await model.learningAction("ignore", host: entry.host) } }
                                        Button("保存代理规则") { Task { await model.learningAction("accept", host: entry.host) } }
                                            .disabled(entry.confirmations < threshold)
                                    }
                                }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                            } else if state == .saved {
                                HStack {
                                    VStack(alignment: .leading) {
                                        Text(entry.host).lineLimit(1).help(entry.host).textSelection(.enabled)
                                        Text("\(entry.scopeTitle) · \(entry.routeTitle)").font(.caption).foregroundStyle(Theme.muted)
                                        if !entry.group.isEmpty { Text(entry.group).font(.caption).foregroundStyle(Theme.muted) }
                                    }
                                    Spacer()
                                    Button("撤销并暂停学习") { Task { await model.learningAction("undo", host: entry.host) } }
                                        .help("移除该目标值对应的所有学习规则并暂停学习；域名后缀规则涉及子域名，显式规则保持不变。")
                                }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                            } else {
                                HStack {
                                    VStack(alignment: .leading) {
                                        Text(entry.host).lineLimit(1).help(entry.host).textSelection(.enabled)
                                        Text("\(entry.service) · 暂停自动学习（不是拒绝访问）").font(.caption).foregroundStyle(Theme.muted)
                                    }
                                    Spacer()
                                    Button("恢复学习") { Task { await model.learningAction("restore", host: entry.host) } }
                                }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                            }
                        }
                    }
                }
            }.id("\(tab)|\(search)|\(service)|\(route)|\(scope)|\(page.number)")
                .disabled(model.stats?.fallback?.strategy != "direct-first")
            HStack {
                Text("\(page.firstRecord)–\(page.lastRecord) / \(page.matched) 条").font(.caption).foregroundStyle(Theme.muted)
                Spacer()
                Button("上一页") { showPage(page.number - 1) }.disabled(page.number == 0)
                Text("\(page.number + 1) / \(page.pageCount)").font(.caption.monospaced()).fixedSize()
                Button("下一页") { showPage(page.number + 1) }.disabled(page.number + 1 >= page.pageCount)
            }
            Text("收到响应只证明连接有数据，不保证业务成功。撤销会同时暂停学习；恢复后需重新观察，不会立即添加规则。").font(.caption).foregroundStyle(Theme.muted)
        }.padding(embedded ? 0:24).frame(maxWidth:embedded ? .infinity:800).frame(height:embedded ? 520:680)
        .disabled(model.isBusy)
        .onAppear { refresh(model.stats) }
        .onReceive(model.$stats.dropFirst()) { refresh($0) }
        .onChange(of: search) { _ in showPage(0) }
        .onChange(of: tab) { _ in service = ""; route = ""; scope = ""; showPage(0) }
        .onChange(of: service) { _ in showPage(0) }
        .onChange(of: route) { _ in showPage(0) }
        .onChange(of: scope) { _ in showPage(0) }
        .onChange(of: isActive) { active in if active { refresh(model.stats) } }
        .sheet(isPresented:$showSettings) { SheetFrame(title:"自学习设置") { LearningSettingsView() } }
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
