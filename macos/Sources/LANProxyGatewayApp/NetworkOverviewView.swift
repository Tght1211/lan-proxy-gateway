import SwiftUI
import Charts

struct NetworkOverviewView: View {
    @EnvironmentObject private var model: AppModel
    @State private var sheet: NetworkSheet?
    var body: some View {
        ScrollPage {
            HStack(spacing: 22) {
                overviewMetric("在线设备", "\(model.activeDeviceCount)", "台")
                overviewMetric("实时下载 / 上传", "\(shortBytes((model.stats?.relay.traffic.last?.down ?? 0) / 5)) / \(shortBytes((model.stats?.relay.traffic.last?.up ?? 0) / 5))", "/s")
                overviewMetric("已记录流量", shortBytes((model.stats?.relay.upTotal ?? 0) + (model.stats?.relay.downTotal ?? 0)), "本次运行")
                overviewMetric("正在连接", "\(model.stats?.relay.active.count ?? 0)", "条")
            }.padding(.vertical, 16).overlay(alignment: .top) { Rectangle().fill(Theme.border).frame(height: 0.7) }
                .overlay(alignment: .bottom) { Rectangle().fill(Theme.border).frame(height: 0.7) }
            NetworkTopologyView { sheet = $0 }
            HStack(alignment: .top, spacing: 18) {
                NetworkStabilityCard().frame(maxWidth: .infinity)
                NetworkThroughputCard().frame(maxWidth: .infinity)
            }
        }
        .sheet(item: $sheet) { NetworkSheetContent(destination: $0).environmentObject(model) }
        .task { await model.refreshHotspot() }
    }
    private func overviewMetric(_ title: String, _ value: String, _ unit: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.caption).foregroundStyle(Theme.muted)
            HStack(alignment: .firstTextBaseline, spacing: 5) { Text(model.stats == nil ? "—" : value).font(.system(size: 21, weight: .semibold)).monospacedDigit().lineLimit(1).minimumScaleFactor(0.65); Text(unit).font(.caption).foregroundStyle(Theme.muted) }
        }.frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct NetworkStabilityCard: View {
    @EnvironmentObject private var model: AppModel
    @State private var probe: ProbePoint?
    var compact = false
    private var points: [ProbePoint] { (model.stats?.health.history ?? []).filter { (0...300).contains(Date().timeIntervalSince($0.at)) } }
    private var latest: ProbePoint? { points.last.flatMap { Date().timeIntervalSince($0.at) < 30 ? $0 : nil } }
    var body: some View {
        Group {
            if compact { content }
            else { Panel { content.frame(height: 270) } }
        }.sheet(item: $probe) { p in SheetFrame(title: "探测记录") { Text(p.at.formatted()); Text(p.ok ? formatMS(p.latencyMS) : "未收到探测响应") } }
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
            HStack(spacing: 3) {
                ForEach(0..<30, id: \.self) { slot in
                    let time = Date().addingTimeInterval(Double(slot - 29) * 10)
                    let point = points.last { abs($0.at.timeIntervalSince(time)) <= 5 }
                    Button { probe = point } label: {
                        RoundedRectangle(cornerRadius: 3).fill(point.map { !$0.ok ? Theme.coral : $0.latencyMS > 500 ? Theme.yellow : Theme.lime } ?? Theme.border.opacity(0.4)).frame(height: compact ? 32 : 35)
                    }.buttonStyle(.plain).disabled(point == nil)
                        .help(point.map { $0.ok ? formatMS($0.latencyMS) : "探测失败" } ?? "无数据")
                }
            }
            HStack { Text("5 分钟前"); Spacer(); Text("现在") }.font(.system(size: 11)).foregroundStyle(Theme.muted)
            Text("当前出口独立探测，不代表设备 Wi-Fi 或所有网站。")
                .font(.caption2).foregroundStyle(Theme.muted)
        }
    }
}

struct NetworkThroughputCard: View {
    @EnvironmentObject private var model: AppModel
    private var end: Date { Date() }
    private var points: [TrafficPoint] { (model.stats?.relay.traffic ?? []).filter { $0.at >= end.addingTimeInterval(-60) } }
    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack { Text("实时吞吐").font(.system(size: 15, weight: .semibold)); Spacer(); Text("最近 1 分钟").font(.caption).foregroundStyle(Theme.muted) }
                HStack(spacing: 22) {
                    Label(shortBytes((points.last?.down ?? 0) / 5) + "/s", systemImage: "arrow.down").foregroundStyle(Theme.cyan)
                    Label(shortBytes((points.last?.up ?? 0) / 5) + "/s", systemImage: "arrow.up").foregroundStyle(Theme.yellow)
                }.font(.system(size: 21, weight: .medium)).monospacedDigit()
                Chart(points) { point in
                    LineMark(x: .value("时间", point.at), y: .value("下载", Double(point.down) / 5), series: .value("方向", "下载")).foregroundStyle(Theme.cyan)
                    LineMark(x: .value("时间", point.at), y: .value("上传", Double(point.up) / 5), series: .value("方向", "上传")).foregroundStyle(Theme.yellow)
                }
                .chartXScale(domain: end.addingTimeInterval(-60)...end)
                .chartYAxis {
                    AxisMarks(position: .leading) { value in
                        AxisGridLine().foregroundStyle(Theme.border.opacity(0.5))
                        AxisValueLabel {
                            if let b = value.as(Double.self) { Text(shortBytes(Int64(max(0, b))) + "/s") }
                        }
                    }
                }
                .overlay { if points.isEmpty { Text("等待真实流量采样").font(.caption).foregroundStyle(Theme.muted) } }
                HStack { ChartLegend(color: Theme.cyan, text: "下载"); ChartLegend(color: Theme.yellow, text: "上传"); Spacer(); Text("核心采样 · 5 秒").font(.caption2).foregroundStyle(Theme.muted) }
            }.frame(height: 270)
        }
    }
}
