import SwiftUI

private struct TrafficUsageGroup: Identifiable {
    let id: String
    let rows: [DailyUsage]
    var up: Int64 { rows.reduce(0) { $0 + $1.up } }
    var down: Int64 { rows.reduce(0) { $0 + $1.down } }
    var total: Int64 { up + down }
    var proxy: Int64 { rows.filter { $0.egress == "proxy" }.reduce(0) { $0 + $1.total } }
    var direct: Int64 { rows.filter { $0.egress == "direct" }.reduce(0) { $0 + $1.total } }
}

struct PlayBridgeTrafficView: View {
    @EnvironmentObject private var model: AppModel
    @State private var grouping = "device"
    @State private var route = "all"
    @State private var search = ""
    @State private var limit = 50

    private var rows: [DailyUsage] {
        guard let network = model.stats?.hotspot ?? model.hotspot else { return [] }
        return hotspotUsage(model.stats?.usageHistory ?? [], network: network, date: usageDate())
    }
    private var available: Bool {
        model.stats?.usageHistory != nil && (model.stats?.hotspot ?? model.hotspot)?.available == true
    }
    private var total: Int64 { rows.reduce(0) { $0 + $1.total } }
    private var proxy: Int64 { rows.filter { $0.egress == "proxy" }.reduce(0) { $0 + $1.total } }
    private var direct: Int64 { rows.filter { $0.egress == "direct" }.reduce(0) { $0 + $1.total } }
    private var filtered: [DailyUsage] {
        let query = search.trimmingCharacters(in: .whitespacesAndNewlines)
        return rows.filter { row in
            let matchesRoute = route == "all" || row.egress == route || (route == "unknown" && row.egress != "proxy" && row.egress != "direct")
            let text = [row.device, model.effectiveDeviceLabel(for: row.device), row.destination ?? "", row.service ?? ""].joined(separator: " ")
            return matchesRoute && (query.isEmpty || text.localizedCaseInsensitiveContains(query))
        }
    }
    private var groups: [TrafficUsageGroup] {
        Dictionary(grouping: filtered) { row in
            grouping == "device" ? row.device : (row.destination?.nonEmpty ?? row.service?.nonEmpty ?? "未识别目标")
        }.map { TrafficUsageGroup(id: $0.key, rows: $0.value) }
            .sorted { $0.total == $1.total ? $0.id < $1.id : $0.total > $1.total }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            overview
            Panel {
                VStack(alignment: .leading, spacing: 18) {
                    HStack {
                        Text("流量去向").font(.headline)
                        Spacer()
                        Picker("统计维度", selection: $grouping) {
                            Text("按设备").tag("device")
                            Text("按访问目标").tag("destination")
                        }.labelsHidden().pickerStyle(.segmented).frame(width: 210)
                    }
                    HStack(spacing: 16) {
                        TextField("搜索设备、域名或 IP", text: $search)
                            .textFieldStyle(.roundedBorder).frame(maxWidth: 300)
                        Spacer(minLength: 0)
                        Picker("出口", selection: $route) {
                            Text("全部出口").tag("all")
                            Text("代理").tag("proxy")
                            Text("直连").tag("direct")
                            Text("未分类").tag("unknown")
                        }.frame(width: 165)
                    }
                    HStack {
                        Text("\(groups.count) 项 · \(bytes(filtered.reduce(0) { $0 + $1.total }))")
                        Spacer()
                        Text("按用量从高到低")
                    }.font(.caption).foregroundStyle(.secondary)
                    Divider()
                    if groups.isEmpty {
                        VStack(spacing: 10) {
                            Image(systemName: "tray").font(.title2).foregroundStyle(Theme.cyan)
                            Text(!available ? "等待热点统计" : rows.isEmpty ? "今天还没有记录到热点流量" : "没有符合筛选条件的记录")
                            Text("仅统计经过核心转发的热点流量。")
                                .font(.caption).foregroundStyle(.secondary)
                        }.frame(maxWidth: .infinity).padding(.vertical, 35)
                    } else {
                        HStack {
                            Text(grouping == "device" ? "设备 / IP" : "访问目标")
                            Spacer()
                            Text("上传").frame(width: 80, alignment: .trailing)
                            Text("下载").frame(width: 80, alignment: .trailing)
                            Text("总用量").frame(width: 95, alignment: .trailing)
                        }.font(.caption).foregroundStyle(.secondary)
                        LazyVStack(spacing: 0) {
                            ForEach(Array(groups.prefix(limit))) { group in
                                usageRow(group)
                                Divider()
                            }
                        }
                        if groups.count > limit {
                            Button("显示更多（剩余 \(groups.count - limit) 项）") { limit += 50 }
                                .frame(maxWidth: .infinity)
                        }
                    }
                }
            }
            Text("今日按本地日期汇总，仅包含核心记录的当前热点网段流量。历史记录按当前网段归类；未接管或绕过核心的流量不计入，旧记录可能未区分代理与直连。")
                .font(.caption2).foregroundStyle(.secondary)
        }
        .onChange(of: grouping) { _ in limit = 50 }
        .onChange(of: route) { _ in limit = 50 }
        .onChange(of: search) { _ in limit = 50 }
    }

    private var overview: some View {
        Panel {
            VStack(alignment: .leading, spacing: 22) {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("今日热点用量").font(.headline)
                        Text(available ? bytes(total) : "—")
                            .font(.system(size: 38, weight: .semibold, design: .rounded)).monospacedDigit()
                    }
                    Spacer()
                    VStack(alignment: .trailing, spacing: 8) {
                        Text(Date().formatted(date: .abbreviated, time: .omitted)).font(.caption).foregroundStyle(.secondary)
                        Text("\(Set(rows.map(\.device)).count) 台使用过的设备").font(.subheadline)
                    }
                }
                distribution(proxy: proxy, direct: direct, total: total).frame(height: 10)
                HStack(spacing: 32) {
                    breakdown("通过代理", proxy, Theme.cyan)
                    breakdown("直接连接", direct, Theme.lime)
                    if total > proxy + direct { breakdown("未分类", total - proxy - direct, .secondary) }
                    Spacer(minLength: 0)
                }
            }.padding(8)
        }
    }

    private func breakdown(_ title: String, _ value: Int64, _ color: Color) -> some View {
        VStack(alignment: .leading, spacing: 7) {
            Label { Text(title) } icon: { Circle().fill(color).frame(width: 7, height: 7) }
                .font(.caption).foregroundStyle(.secondary)
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text(available ? bytes(value) : "—").font(.system(size: 20, weight: .semibold, design: .rounded))
                Text(total > 0 ? String(format: "%.1f%%", Double(value) / Double(total) * 100) : "—")
                    .font(.caption).foregroundStyle(.secondary)
            }.monospacedDigit()
        }
    }

    private func usageRow(_ group: TrafficUsageGroup) -> some View {
        VStack(alignment: .leading, spacing: 9) {
            HStack(spacing: 12) {
                Image(systemName: grouping == "device" ? "gamecontroller" : "globe")
                    .foregroundStyle(Theme.cyan).frame(width: 24)
                VStack(alignment: .leading, spacing: 4) {
                    Text(grouping == "device" ? (model.effectiveDeviceLabel(for: group.id).nonEmpty ?? group.id) : group.id)
                        .font(.subheadline.weight(.medium)).lineLimit(1).help(group.id)
                    Text(grouping == "device" ? group.id : "\(Set(group.rows.map(\.device)).count) 台设备访问过")
                        .font(.caption2).foregroundStyle(.secondary)
                }
                Spacer(minLength: 8)
                Text(bytes(group.up)).frame(width: 80, alignment: .trailing)
                Text(bytes(group.down)).frame(width: 80, alignment: .trailing)
                Text(bytes(group.total)).fontWeight(.semibold).frame(width: 95, alignment: .trailing)
            }.font(.caption).monospacedDigit()
            HStack(spacing: 12) {
                distribution(proxy: group.proxy, direct: group.direct, total: group.total)
                    .frame(width: 90, height: 4)
                Text("代理 \(bytes(group.proxy)) · 直连 \(bytes(group.direct))" + (group.total > group.proxy + group.direct ? " · 未分类 \(bytes(group.total - group.proxy - group.direct))" : ""))
                    .font(.caption2).foregroundStyle(.secondary)
            }.padding(.leading, 36)
        }.padding(.vertical, 13)
    }

    private func distribution(proxy: Int64, direct: Int64, total: Int64) -> some View {
        GeometryReader { geometry in
            HStack(spacing: 0) {
                Rectangle().fill(Theme.cyan).frame(width: total > 0 ? geometry.size.width * Double(proxy) / Double(total) : 0)
                Rectangle().fill(Theme.lime).frame(width: total > 0 ? geometry.size.width * Double(direct) / Double(total) : 0)
                Rectangle().fill(Color.secondary.opacity(0.15))
            }.clipShape(Capsule())
        }.accessibilityHidden(true)
    }
    private func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
}
