import SwiftUI

struct NetworkExitsView: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.networkPageActive) private var isActive
    @State private var sheet: NetworkSheet?
    @State private var testing = false
    @State private var testResult: String?
    @State private var usage = NetworkExitUsageSummary(rows: nil, date: "")
    @State private var rejected: [ConnectionInfo] = []
    @State private var conditions: [String: NetworkExitCondition] = [:]
    @State private var statsAvailable = false
    let availableWidth: CGFloat
    private func refresh(_ stats: RuntimeStats?) {
        guard isActive else { return }
        let now = Date()
        usage = NetworkExitUsageSummary(rows: stats?.usageHistory, date: usageDate(now))
        rejected = ((stats?.relay.active ?? []) + (stats?.relay.recent ?? [])).filter(\.rejected)
        statsAvailable = stats != nil
        conditions = Dictionary(uniqueKeysWithValues: ["proxy", "direct"].map { exit in
            (exit, NetworkGlobalHealth.condition(for: exit, stats: stats, isRunning: model.isRunning, at: now))
        })
    }
    private var proxyConfigured: Bool { (model.status?.proxy ?? model.stats?.proxy)?.isEmpty == false }
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text("网络出口").font(.headline)
                Text("流量与响应状态").font(.caption).foregroundStyle(Theme.muted)
                Spacer()
                Button(proxyConfigured ? "编辑代理源" : "＋ 配置代理源") { sheet = .proxy }
            }
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 14),
                                     count: NetworkOverviewSizing.exitColumnCount(width: availableWidth)),
                      alignment: .leading, spacing: 14) {
                proxyCard
                directCard
                rejectionCard
            }
        }.sheet(item: $sheet) { NetworkSheetContent(destination: $0).environmentObject(model) }
            .onAppear { refresh(model.stats) }
            .onReceive(model.$stats.dropFirst()) { refresh($0) }
            .onChange(of: model.isRunning) { _ in refresh(model.stats) }
            .onChange(of: proxyConfigured) { _ in refresh(model.stats) }
            .onChange(of: isActive) { active in
                if active { refresh(model.stats) }
            }
    }
    private var proxyCard: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                heading("代理出口", icon: "cloud", tint: Theme.cyan,
                        status: proxyConfigured ? condition("proxy").title : "未配置",
                        statusTint: proxyConfigured ? statusColor(condition("proxy")) : Theme.muted)
                description(model.status?.proxy ?? model.stats?.proxy ?? "配置代理源后查看此出口的流量与探测")
                HStack {
                    metric("今日流量", value: total("proxy"))
                    Spacer()
                    metric("今日连接", value: connections("proxy"))
                }
                probeStrip(for: "proxy")
                Spacer(minLength: 0)
                HStack {
                    Button("配置代理源") { sheet = .proxy }
                    Spacer()
                    Button(testing ? "检测中…" : "测试代理") {
                        testing = true
                        Task {
                            let result = await model.testProxyAsync()
                            testResult = result.message
                            testing = false
                        }
                    }.disabled(testing || model.isBusy || !proxyConfigured)
                }
                Text(testResult ?? "").font(.caption2).foregroundStyle(Theme.muted)
                    .lineLimit(1).frame(height: 12).help(testResult ?? "")
            }.frame(height: 214, alignment: .topLeading)
        }
    }
    private var directCard: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                heading("本机直连", icon: "globe", tint: Theme.lime, status: condition("direct").title,
                        statusTint: statusColor(condition("direct")))
                description("使用 Mac 当前网络 · DIRECT")
                HStack {
                    metric("今日流量", value: total("direct"))
                    Spacer()
                    metric("今日连接", value: connections("direct"))
                }
                probeStrip(for: "direct")
                Spacer(minLength: 0)
                HStack {
                    Text("未配置域名默认直连").font(.caption2).foregroundStyle(Theme.muted)
                    Spacer()
                    Button("流量明细 ↗") { sheet = .direct }
                }
            }.frame(height: 214, alignment: .topLeading)
        }
    }
    private var rejectionCard: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                heading("拒绝规则", icon: "ban", tint: Theme.coral, status: "本机终止")
                description("命中拒绝规则的请求不会转发到目标")
                HStack {
                    metric("最近触发", value: statsAvailable ? "\(rejected.count) 次" : "—")
                    Spacer()
                    metric("拒绝规则", value: model.status == nil ? "—" : "\((model.status?.routing ?? []).filter { $0.action == "reject" }.count) 条")
                }
                Text(rejected.first.map { "最近拒绝 · \($0.dstHost)" } ?? "暂无触发记录")
                    .font(.caption2).foregroundStyle(Theme.muted).lineLimit(1)
                Spacer(minLength: 0)
                HStack {
                    Text("不计入出口健康统计").font(.caption2).foregroundStyle(Theme.muted)
                    Spacer()
                    Button("规则与记录 ↗") { sheet = .reject }
                }
            }.frame(height: 214, alignment: .topLeading)
        }
    }
    private func condition(_ exit: String) -> NetworkExitCondition {
        conditions[exit] ?? .unknown
    }
    private func total(_ exit: String) -> String {
        guard usage.hasData else { return "—" }
        return shortBytes(usage.usage(for: exit).total)
    }
    private func connections(_ exit: String) -> String {
        guard usage.hasData else { return "—" }
        return "\(usage.usage(for: exit).connections) 条"
    }
    private func statusColor(_ condition: NetworkExitCondition) -> Color {
        switch condition {
        case .healthy: return Theme.lime
        case .degraded: return Theme.yellow
        case .unavailable: return Theme.coral
        case .unknown, .stopped: return Theme.muted
        }
    }
    private func heading(_ title: String, icon: String, tint: Color, status: String, statusTint: Color? = nil) -> some View {
        HStack(spacing: 8) {
            StudioIcon(icon).frame(width: 20, height: 20).foregroundStyle(tint)
            Text(title).font(.system(size: 15, weight: .semibold))
            Spacer(minLength: 4)
            StudioTag(text: status, tint: statusTint ?? tint)
        }
    }
    private func description(_ text: String) -> some View {
        Text(text).font(.system(size: 11)).foregroundStyle(Theme.muted).lineLimit(2)
            .textSelection(.enabled).frame(height: 30, alignment: .topLeading).help(text)
    }
    private func metric(_ title: String, value: String) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.system(size: 10)).foregroundStyle(Theme.muted)
            Text(value).font(.system(size: 19, weight: .semibold)).monospacedDigit()
                .lineLimit(1).minimumScaleFactor(0.7)
        }
    }
    private func probeStrip(for exit: String) -> some View {
        let now = Date()
        let points = model.isRunning ? model.stats?.health(for: exit)?.history ?? [] : []
        let latest = points.last { (0..<30).contains(now.timeIntervalSince($0.at)) }
        return VStack(alignment: .leading, spacing: 5) {
            HStack {
                Text(exit == "direct" ? "百度直连探测 · 近 5 分钟" : "独立探测 · 近 5 分钟")
                Spacer()
                Text(latest.map { $0.ok ? formatMS($0.latencyMS) : "探测失败" } ?? "暂无探测")
            }.font(.system(size: 9)).foregroundStyle(Theme.muted)
            HStack(spacing: 2) {
                ForEach(0..<30, id: \.self) { slot in
                    let timestamp = now.addingTimeInterval(Double(slot - 29) * 10)
                    let point = points.last { abs($0.at.timeIntervalSince(timestamp)) <= 5 }
                    RoundedRectangle(cornerRadius: 2)
                        .fill(point.map { !$0.ok ? Theme.coral : $0.latencyMS > 500 ? Theme.yellow : Theme.lime } ?? Theme.border.opacity(0.4))
                        .frame(height: 8)
                        .help(point.map { $0.ok ? formatMS($0.latencyMS) : "探测失败" } ?? "无探测记录")
                }
            }
        }.help(exit == "direct" ? "每 10 秒通过本机直连请求百度，最多等待 8 秒；收到 HTTP 响应表示此探测路径可达，不代表所有网站或设备网络正常。旧核心没有独立直连数据时不会借用代理结果。" : "通过代理独立探测；不借用直连结果，也不代表所有网站可用。")
    }
}
