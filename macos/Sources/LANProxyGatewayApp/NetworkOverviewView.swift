import SwiftUI
import Charts

struct NetworkOverviewView: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.networkPageActive) private var isActive
    @State private var sheet: NetworkSheet?
    @State private var trafficSummary = NetworkTrafficSummary(rows: nil)
    private func refreshTraffic(_ stats: RuntimeStats?) {
        guard isActive else { return }
        trafficSummary = NetworkTrafficSummary(rows: stats?.usageHistory)
    }
    var body: some View {
        GeometryReader { viewport in
            ScrollView(.vertical) {
                VStack(alignment: .leading, spacing: NetworkOverviewSizing.spacing) {
                    summary
                    NetworkTopologyView { sheet = $0 }
                    NetworkExitsView(availableWidth: max(0, viewport.size.width - 44))
                }.padding(22).frame(maxWidth: .infinity)
            }
        }
        .sheet(item: $sheet) { NetworkSheetContent(destination: $0).environmentObject(model) }
        .task { await model.refreshHotspot() }
        .onAppear { refreshTraffic(model.stats) }
        .onReceive(model.$stats.dropFirst()) { refreshTraffic($0) }
        .onChange(of: isActive) { active in
            if active { refreshTraffic(model.stats) }
        }
    }
    private var summary: some View {
        NetworkSummaryLayout {
            deviceMetric
            recordedMetric
            throughputSummary
            stabilitySummary
        }
        .background { NetworkSummarySeparators().stroke(Theme.border.opacity(0.6), lineWidth: 0.7) }
        .padding(.vertical, 14).overlay(alignment: .top) { Rectangle().fill(Theme.border).frame(height: 0.7) }
            .overlay(alignment: .bottom) { Rectangle().fill(Theme.border).frame(height: 0.7) }
    }
    private var deviceMetric: some View {
        Button { model.selectedSection = .devices } label: {
            overviewMetric("在线设备", "\(model.activeDeviceCount)", "台", footnote: "近 10 分钟有流量")
        }.buttonStyle(.plain).help("查看接入设备")
    }
    private var recordedMetric: some View {
        let traffic = trafficSummary
        return VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("流量构成").font(.caption)
                Spacer(minLength: 4)
                Text("本季度已记录").font(.system(size: 9))
            }.foregroundStyle(Theme.muted)
            HStack(spacing: 18) {
                trafficMetric("代理", bytes: traffic.proxy, available: traffic.hasData, color: Theme.cyan)
                trafficMetric("直连", bytes: traffic.direct, available: traffic.hasData, color: Theme.lime)
            }.frame(height: 28, alignment: .leading)
            VStack(alignment: .leading, spacing: 4) {
                GeometryReader { geometry in
                    HStack(spacing: 0) {
                        Rectangle().fill(Theme.cyan).frame(width: traffic.total > 0 ? geometry.size.width * Double(traffic.proxy) / Double(traffic.total) : 0)
                        Rectangle().fill(Theme.lime).frame(width: traffic.total > 0 ? geometry.size.width * Double(traffic.direct) / Double(traffic.total) : 0)
                    }
                }.frame(height: 4).background(Theme.border.opacity(0.4)).clipShape(Capsule())
                Text(traffic.hasData ? "合计 \(shortBytes(traffic.total))" : "等待流量记录")
                    .font(.system(size: 9)).foregroundStyle(Theme.muted)
            }.frame(height: 20, alignment: .leading)
        }.help("按本季度已保留的网关流量记录统计，不包含未经过网关的流量。" + (traffic.unclassified > 0 ? "另有 \(shortBytes(traffic.unclassified)) 历史流量未标注出口，不计入代理或直连。" : ""))
    }
    private func trafficMetric(_ title: String, bytes: Int64, available: Bool, color: Color) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 4) {
            Text(title).font(.system(size: 10)).foregroundStyle(Theme.muted)
            Text(available ? shortBytes(bytes) : "—").font(.system(size: 20, weight: .semibold))
                .foregroundStyle(color).monospacedDigit().lineLimit(1).minimumScaleFactor(0.65)
        }
    }
    private var throughputSummary: some View { NetworkThroughputSummary() }
    private var stabilitySummary: some View { NetworkGlobalHealthSummary() }
    private func overviewMetric(_ title: String, _ value: String, _ unit: String, footnote: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.caption).foregroundStyle(Theme.muted)
            HStack(alignment: .firstTextBaseline, spacing: 5) {
                Text(model.stats == nil ? "—" : value).font(.system(size: 22, weight: .semibold)).monospacedDigit().lineLimit(1).minimumScaleFactor(0.75)
                if !unit.isEmpty { Text(unit).font(.system(size: 10)).foregroundStyle(Theme.muted) }
            }.frame(height: 28, alignment: .leading)
            Text(footnote).font(.system(size: 10)).foregroundStyle(Theme.muted).frame(height: 20, alignment: .leading)
        }.frame(maxWidth: .infinity, alignment: .leading)
    }
}

private struct NetworkSummaryLayout: Layout {
    static let spacing: CGFloat = 24
    static let breakpoint: CGFloat = 880

    static func wideWidths(for width: CGFloat) -> [CGFloat] {
        let available = max(0, width - spacing * 3)
        return [available * 0.18, available * 0.36, available * 0.28, available * 0.18]
    }
    private func rows(width: CGFloat) -> [(indices: [Int], widths: [CGFloat])] {
        if width >= Self.breakpoint { return [([0, 1, 2, 3], Self.wideWidths(for: width))] }
        if width < 540 { return (0..<4).map { ([$0], [width]) } }
        let available = width - Self.spacing
        return [([0, 1], [available / 3, available * 2 / 3]),
                ([2, 3], [available * 0.6, available * 0.4])]
    }
    private func height(for row: (indices: [Int], widths: [CGFloat]), subviews: Subviews) -> CGFloat {
        zip(row.indices, row.widths).map { index, width in
            subviews[index].sizeThatFits(ProposedViewSize(width: width, height: nil)).height
        }.max() ?? 0
    }
    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? Self.breakpoint
        let rows = rows(width: width)
        return CGSize(width: width, height: rows.reduce(CGFloat(0)) { $0 + height(for: $1, subviews: subviews) } + CGFloat(rows.count - 1) * 16)
    }
    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var vertical = bounds.minY
        for row in rows(width: bounds.width) {
            let rowHeight = height(for: row, subviews: subviews)
            var horizontal = bounds.minX
            for (index, width) in zip(row.indices, row.widths) {
                subviews[index].place(at: CGPoint(x: horizontal, y: vertical), anchor: .topLeading,
                                      proposal: ProposedViewSize(width: width, height: rowHeight))
                horizontal += width + Self.spacing
            }
            vertical += rowHeight + 16
        }
    }
}

private struct NetworkSummarySeparators: Shape {
    func path(in rect: CGRect) -> Path {
        var path = Path()
        guard rect.width >= NetworkSummaryLayout.breakpoint else { return path }
        var horizontal = rect.minX
        for width in NetworkSummaryLayout.wideWidths(for: rect.width).dropLast() {
            horizontal += width + NetworkSummaryLayout.spacing
            let position = horizontal - NetworkSummaryLayout.spacing / 2
            path.move(to: CGPoint(x: position, y: rect.minY + 1))
            path.addLine(to: CGPoint(x: position, y: rect.maxY - 1))
        }
        return path
    }
}

private struct NetworkGlobalHealthSummary: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.networkPageActive) private var isActive
    @State private var current = NetworkGlobalHealth(stats: nil, proxyConfigured: false, isRunning: false)
    @State private var history: [NetworkGlobalHealth] = []
    private var proxyConfigured: Bool { (model.status?.proxy ?? model.stats?.proxy)?.isEmpty == false }
    private func refresh(_ stats: RuntimeStats?) {
        guard isActive else { return }
        let now = Date()
        current = NetworkGlobalHealth(stats: stats, proxyConfigured: proxyConfigured,
                                      isRunning: model.isRunning, at: now)
        history = (0..<30).map { slot in
            NetworkGlobalHealth(stats: stats, proxyConfigured: proxyConfigured, isRunning: model.isRunning,
                                at: now.addingTimeInterval(Double(slot - 29) * 10))
        }
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("网络出口健康").font(.caption).foregroundStyle(Theme.muted)
            HStack(alignment: .firstTextBaseline, spacing: 5) {
                Text(current.title).font(.system(size: 19, weight: .semibold)).lineLimit(1).minimumScaleFactor(0.7)
                Text("\(current.healthyCount)/\(current.conditions.count)").font(.system(size: 10)).foregroundStyle(Theme.muted)
            }.frame(height: 28, alignment: .leading)
            HStack(spacing: 2) {
                ForEach(Array(history.enumerated()), id: \.offset) { _, sample in
                    RoundedRectangle(cornerRadius: 2).fill(color(sample)).frame(height: 10)
                        .help("近期探测与连接响应 · \(sample.title) · 正常 \(sample.healthyCount)/\(sample.conditions.count)")
                }
            }.frame(height: 20, alignment: .center)
        }.help("汇总已配置的代理与直连出口，拒绝规则不计入。正常数来自各出口的独立探测或真实连接响应；直连探测百度，无记录显示待检测。时间条回看近 5 分钟的探测与连接响应，不把单个出口延迟当作全局延迟。")
            .onAppear { refresh(model.stats) }
            .onReceive(model.$stats.dropFirst()) { refresh($0) }
            .onChange(of: model.isRunning) { _ in refresh(model.stats) }
            .onChange(of: proxyConfigured) { _ in refresh(model.stats) }
            .onChange(of: isActive) { active in
                if active { refresh(model.stats) }
            }
    }
    private func color(_ health: NetworkGlobalHealth) -> Color {
        if health.hasFailure { return health.healthyCount > 0 ? Theme.yellow : Theme.coral }
        if health.healthyCount == health.conditions.count { return Theme.lime }
        return Theme.border.opacity(0.5)
    }
}

struct NetworkStabilityCard: View {
    @EnvironmentObject private var model: AppModel
    @State private var probe: ProbePoint?
    var compact = false
    var summary = false
    private var points: [ProbePoint] { (model.stats?.health.history ?? []).filter { (0...300).contains(Date().timeIntervalSince($0.at)) } }
    private var latest: ProbePoint? { points.last.flatMap { Date().timeIntervalSince($0.at) < 30 ? $0 : nil } }
    var body: some View {
        Group {
            if summary { summaryContent }
            else if compact { content }
            else { Panel { content.frame(height: 270) } }
        }.sheet(item: $probe) { p in SheetFrame(title: "探测记录") { Text(p.at.formatted()); Text(p.ok ? formatMS(p.latencyMS) : "未收到探测响应") } }
    }
    private var summaryContent: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("网络稳定性").font(.caption)
                Spacer(minLength: 4)
                Text("近 5 分钟").font(.system(size: 9))
            }.foregroundStyle(Theme.muted)
            HStack(alignment: .firstTextBaseline, spacing: 6) {
                Text(points.isEmpty ? "—" : String(format: "%.1f%%", Double(points.filter(\.ok).count) / Double(points.count) * 100))
                    .font(.system(size: 22, weight: .semibold)).monospacedDigit()
                Text(latest.map { $0.ok ? formatMS($0.latencyMS) : "探测失败" } ?? "等待探测")
                    .font(.system(size: 10)).foregroundStyle(latest.map { !$0.ok ? Theme.coral : $0.latencyMS > 500 ? Theme.yellow : Theme.lime } ?? Theme.muted)
                    .lineLimit(1).minimumScaleFactor(0.8)
            }.frame(height: 28, alignment: .leading)
            historyBar(height: 12).frame(height: 20, alignment: .center)
        }.help("近 5 分钟当前出口探测可用率；不代表设备 Wi-Fi 或所有网站。点击时间条查看探测记录。")
    }
    private func historyBar(height: CGFloat) -> some View {
        HStack(spacing: 3) {
            ForEach(0..<30, id: \.self) { slot in
                let time = Date().addingTimeInterval(Double(slot - 29) * 10)
                let point = points.last { abs($0.at.timeIntervalSince(time)) <= 5 }
                Button { probe = point } label: {
                    RoundedRectangle(cornerRadius: 2).fill(point.map { !$0.ok ? Theme.coral : $0.latencyMS > 500 ? Theme.yellow : Theme.lime } ?? Theme.border.opacity(0.4)).frame(height: height)
                }.buttonStyle(.plain).disabled(point == nil)
                    .help(point.map { $0.ok ? formatMS($0.latencyMS) : "探测失败" } ?? "无数据")
                    .accessibilityLabel(point.map { "\($0.at.formatted())，\($0.ok ? formatMS($0.latencyMS) : "探测失败")" } ?? "无探测数据")
            }
        }
    }
    private var content: some View {
        VStack(alignment: .leading, spacing: compact ? 12 : 18) {
            HStack {
                Text(compact ? "最近 5 分钟稳定性" : "网络稳定性").font(.system(size: 15, weight: .semibold))
                Spacer()
                if !compact { StudioTag(text: latest?.ok == true ? "最近探测成功" : latest == nil ? "等待探测" : "探测失败", tint: latest?.ok == true ? Theme.cyan : Theme.muted) }
            }
            HStack(alignment: .firstTextBaseline) {
                Text(points.isEmpty ? "—" : String(format: "%.1f%%", Double(points.filter(\.ok).count) / Double(points.count) * 100))
                    .font(.system(size: compact ? 24 : 36, weight: .semibold)).monospacedDigit()
                Text("探测可用率").font(.caption).foregroundStyle(Theme.muted)
                Spacer()
                if !compact {
                    VStack(alignment: .trailing, spacing: 8) {
                        Text(model.stats?.egress == "proxy" ? "代理出口" : "本机直连").foregroundStyle(Theme.cyan)
                        Text(latest.map { $0.ok ? "最近响应 · " + formatMS($0.latencyMS) : "探测失败" } ?? "等待新探测").foregroundStyle(Theme.muted)
                    }.font(.system(size: 11))
                }
            }
            if !compact { Spacer(minLength: 0) }
            HStack { Text("近 5 分钟稳定情况"); Spacer(); Text("每 10 秒") }.font(.system(size: 11)).foregroundStyle(Theme.muted)
            historyBar(height: compact ? 32 : 35)
            HStack { Text("5 分钟前"); Spacer(); Text("现在") }.font(.system(size: 11)).foregroundStyle(Theme.muted)
            Text("当前出口独立探测，不代表设备 Wi-Fi 或所有网站。")
                .font(.caption2).foregroundStyle(Theme.muted)
        }
    }
}

struct NetworkThroughputSummary: View {
    @EnvironmentObject private var model: AppModel
    private var end: Date { Date() }
    private var points: [TrafficPoint] { (model.stats?.relay.traffic ?? []).filter { (0...60).contains(end.timeIntervalSince($0.at)) } }
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("实时下载 / 上传").font(.caption)
                Spacer(minLength: 4)
                Text("近 1 分钟").font(.system(size: 9))
            }.foregroundStyle(Theme.muted)
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                rate(points.last?.down, icon: "arrow.down", color: Theme.cyan)
                rate(points.last?.up, icon: "arrow.up", color: Theme.yellow)
            }.frame(height: 28, alignment: .leading)
            Chart(points) { point in
                LineMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5), series: .value("方向", "下载")).foregroundStyle(Theme.cyan)
                LineMark(x: .value("时间", point.at), y: .value("上传", Double(point.up) / 5), series: .value("方向", "上传")).foregroundStyle(Theme.yellow)
            }
            .chartXScale(domain: end.addingTimeInterval(-60)...end)
            .chartXAxis(.hidden).chartYAxis(.hidden).chartLegend(.hidden)
            .frame(height: 20)
            .overlay { if points.isEmpty { Text("等待流量采样").font(.system(size: 9)).foregroundStyle(Theme.muted) } }
        }.help("最近 1 分钟吞吐趋势，青色下载、黄色上传；核心每 5 秒采样。")
    }
    private func rate(_ bytes: Int64?, icon: String, color: Color) -> some View {
        HStack(spacing: 3) {
            Image(systemName: icon).font(.system(size: 11))
            Text(bytes.map { shortBytes($0 / 5) } ?? "—")
                .font(.system(size: 22, weight: .semibold)).monospacedDigit().lineLimit(1).minimumScaleFactor(0.75)
            Text("/s").font(.system(size: 9))
        }.foregroundStyle(color)
    }
}
