import SwiftUI

private struct DestinationUsage: Identifiable {
    let destination: String
    let service: String
    let route: String
    let endpoint: String
    var up: Int64 = 0
    var down: Int64 = 0
    var connections: Int64 = 0
    var lastSeen: Date = .distantPast
    var total: Int64 { up + down }
    var id: String { "\(route)|\(endpoint)|\(destination)" }
}

struct DeviceTrafficSheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var device: String
    @State private var period = "今天"
    @State private var route = "proxy"
    @State private var endpoint = ""
    @State private var search = ""
    @State private var tab = "域名用量"

    init(device: String) { _device = State(initialValue: device) }

    private var devices: [String] {
        Array(Set((model.stats?.usageHistory ?? []).map(\.device) +
                  (model.stats?.relay.devices ?? []).map(\.name) + (device.isEmpty ? [] : [device]))).sorted()
    }
    private var rows: [DailyUsage] {
        let formatter = DateFormatter(); formatter.dateFormat = "yyyy-MM-dd"
        let today = formatter.string(from: Date())
        let days = period == "近 7 天" ? 6 : (period == "近 30 天" ? 29 : 0)
        let start = formatter.string(from: Calendar.current.date(byAdding: .day, value: -days, to: Date()) ?? Date())
        return (model.stats?.usageHistory ?? []).filter {
            (device.isEmpty || $0.device == device) && (period == "全部记录" || ($0.date >= start && $0.date <= today))
        }
    }
    private var endpoints: [String] {
        let current = model.status?.proxy?.split(separator: " ").last.map(String.init) ?? ""
        return Array(Set(rows.compactMap(\.proxyEndpoint).filter { !$0.isEmpty } + (current.isEmpty ? [] : [current]))).sorted()
    }
    private var proxyRows: [DailyUsage] {
        rows.filter { $0.egress == "proxy" && (endpoint.isEmpty || $0.proxyEndpoint == endpoint) }
    }
    private var destinations: [DestinationUsage] {
        var groups: [String: DestinationUsage] = [:]
        for row in rows {
            let target = row.destination?.nonEmpty ?? "旧记录 · 未记录域名"
            let mode = row.egress?.nonEmpty ?? "unknown"
            guard route == "all" || mode == route else { continue }
            if mode == "proxy", !endpoint.isEmpty, row.proxyEndpoint != endpoint { continue }
            let service = row.service?.nonEmpty ?? "未识别服务"
            guard search.isEmpty || target.localizedCaseInsensitiveContains(search) || service.localizedCaseInsensitiveContains(search) else { continue }
            let key = "\(mode)|\(row.proxyEndpoint ?? "")|\(target)"
            var item = groups[key] ?? DestinationUsage(destination: target, service: service, route: mode, endpoint: row.proxyEndpoint ?? "")
            item.up += row.up; item.down += row.down; item.connections += row.connections
            item.lastSeen = max(item.lastSeen, row.lastSeen)
            groups[key] = item
        }
        return groups.values.sorted { $0.total == $1.total ? $0.id < $1.id : $0.total > $1.total }
    }
    private var recent: [ConnectionInfo] {
        ((model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])).filter {
            (device.isEmpty || $0.srcIP == device) &&
            (search.isEmpty || $0.dstHost.localizedCaseInsensitiveContains(search) || $0.service.localizedCaseInsensitiveContains(search)) &&
            (route == "all" || (route == "proxy" && $0.viaProxy && !$0.rejected) || (route == "direct" && !$0.viaProxy && !$0.rejected)) &&
            (!$0.viaProxy || endpoint.isEmpty || $0.proxyEndpoint == endpoint)
        }.sorted { $0.startedAt > $1.startedAt }
    }
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text("设备流量明细").font(.title2.bold())
                Spacer()
                Button("完成") { dismiss() }
            }
            HStack {
                Picker("设备", selection: $device) {
                    Text("全部设备").tag("")
                    ForEach(devices, id: \.self) { ip in
                        Text("\(model.effectiveDeviceLabel(for: ip).nonEmpty ?? "设备") · \(ip)").tag(ip)
                    }
                }.frame(width: 310)
                Picker("日期", selection: $period) {
                    ForEach(["今天", "近 7 天", "近 30 天", "全部记录"], id: \.self) { Text($0) }
                }.frame(width: 160).disabled(tab != "域名用量")
                Picker("代理入口", selection: $endpoint) {
                    Text("全部代理入口").tag("")
                    ForEach(endpoints, id: \.self) { Text($0).tag($0) }
                }.frame(maxWidth: .infinity)
            }
            HStack(spacing: 12) {
                usageCard(endpoint.isEmpty ? "上游代理用量" : "代理 · \(endpoint)", proxyRows, Theme.cyan)
                usageCard("普通宽带 · 本机直连", rows.filter { $0.egress == "direct" }, Theme.lime)
                usageCard("设备总用量", rows, Theme.muted)
            }
            let unknown = rows.filter { $0.egress == nil || $0.egress == "" }.reduce(Int64(0)) { $0 + $1.total }
            if unknown > 0 {
                Text("总量含 \(shortBytes(unknown)) 旧记录，未保存出口，无法追溯拆分。新的代理／直连明细从本次更新后开始累计。")
                    .font(.caption).foregroundStyle(Theme.yellow)
            }
            HStack {
                Picker("记录", selection: $tab) {
                    Text("域名用量").tag("域名用量")
                    Text("当前及最近连接").tag("当前及最近连接")
                }.pickerStyle(.segmented).frame(width: 260)
                Picker("出口", selection: $route) {
                    Text("代理").tag("proxy"); Text("直连").tag("direct"); Text("全部").tag("all")
                }.pickerStyle(.segmented).frame(width: 210)
                TextField("搜索服务或域名", text: $search).textFieldStyle(.roundedBorder)
            }
            if tab == "域名用量" {
                Table(destinations) {
                    TableColumn("服务") { Text($0.service) }.width(min: 85, ideal: 110)
                    TableColumn("域名 / 目标地址") { Text($0.destination).textSelection(.enabled).help($0.destination) }.width(min: 200, ideal: 310)
                    TableColumn("实际出口") { item in
                        Text(item.route == "proxy" ? (item.endpoint.isEmpty ? "上游代理" : item.endpoint) : (item.route == "direct" ? "本机直连" : "未分类"))
                            .foregroundStyle(item.route == "proxy" ? Theme.cyan : Theme.muted)
                    }.width(min: 110, ideal: 140)
                    TableColumn("上传") { Text(shortBytes($0.up)) }.width(75)
                    TableColumn("下载") { Text(shortBytes($0.down)) }.width(75)
                    TableColumn("合计") { Text(shortBytes($0.total)).fontWeight(.semibold) }.width(75)
                    TableColumn("连接") { Text("\($0.connections)") }.width(50)
                }
                if destinations.isEmpty { Text("此筛选范围暂无记录；设备产生新流量后会自动显示。").font(.caption).foregroundStyle(Theme.muted) }
            } else {
                Text("以下为当前及最近连接，不受日期筛选影响；重启后清空。上方累计用量不会随连接记录清空。")
                    .font(.caption).foregroundStyle(Theme.muted)
                Table(recent) {
                    TableColumn("时间") { Text($0.startedAt.formatted(date: .omitted, time: .standard)) }.width(80)
                    TableColumn("设备") { Text($0.srcIP) }.width(105)
                    TableColumn("服务 / 目标") { Text("\($0.service) · \($0.dstHost):\($0.dstPort)").help($0.dstHost) }.width(min: 230, ideal: 350)
                    TableColumn("实际出口") { item in
                        Text(item.rejected ? "拒绝" : (item.viaProxy ? (item.proxyEndpoint ?? "上游代理") : (item.fallback ? "回退直连" : "本机直连")))
                    }.width(min: 110, ideal: 140)
                    TableColumn("状态") { Text($0.outcome.label).help($0.failure) }.width(65)
                    TableColumn("上传") { Text(shortBytes($0.up)) }.width(70)
                    TableColumn("下载") { Text(shortBytes($0.down)) }.width(70)
                }
            }
            Text("按设备 IP 统计经过本 App 的转发字节（上传＋下载）；直接绕过本 App 的流量无法统计。域名用量按日保存在本机，重启保留；HTTPS 不记录页面内容。")
                .font(.caption2).foregroundStyle(Theme.muted)
            Text("代理入口表示实际连接的上游端口，不包含 Mac 自身其他应用使用该端口的流量，也不等同于代理服务商账单。设备 IP 被重新分配时，请留意设备归属。")
                .font(.caption2).foregroundStyle(Theme.muted)
        }
        .padding(22).frame(width: 1000, height: 700)
        .task {
            if let current = model.status?.proxy?.split(separator: " ").last.map(String.init), endpoints.contains(current) { endpoint = current }
        }
    }
    private func usageCard(_ title: String, _ entries: [DailyUsage], _ tint: Color) -> some View {
        let up = entries.reduce(Int64(0)) { $0 + $1.up }
        let down = entries.reduce(Int64(0)) { $0 + $1.down }
        return VStack(alignment: .leading, spacing: 7) {
            Text(title).font(.caption).foregroundStyle(tint)
            Text(shortBytes(up + down)).font(.title2.bold().monospacedDigit())
            Text("↑ \(shortBytes(up))   ↓ \(shortBytes(down))").font(.caption.monospacedDigit()).foregroundStyle(Theme.muted)
        }.frame(maxWidth: .infinity, alignment: .leading)
            .padding(14).background(tint.opacity(0.08)).cornerRadius(10)
    }
}

struct ProxyUsageOverview: View {
    @EnvironmentObject private var model: AppModel
    @State private var showDetails = false
    var body: some View {
        let formatter = DateFormatter()
        let _ = formatter.dateFormat = "yyyy-MM-dd"
        let today = formatter.string(from: Date())
        let rows = (model.stats?.usageHistory ?? []).filter { $0.date == today }
        let endpoint = model.status?.proxy?.split(separator: " ").last.map(String.init) ?? ""
        let proxy = rows.filter { $0.egress == "proxy" && (endpoint.isEmpty || $0.proxyEndpoint == endpoint) }
        let up = proxy.reduce(Int64(0)) { $0 + $1.up }
        let down = proxy.reduce(Int64(0)) { $0 + $1.down }
        let direct = rows.filter { $0.egress == "direct" }.reduce(Int64(0)) { $0 + $1.total }
        return Panel {
            HStack(spacing: 24) {
                VStack(alignment: .leading, spacing: 6) {
                    Text("今日代理用量 · \(endpoint.isEmpty ? "全部入口" : endpoint)").font(.headline).foregroundStyle(Theme.cyan)
                    Text(shortBytes(up + down)).font(.system(size: 28, weight: .bold, design: .rounded))
                    Text("上传 \(shortBytes(up)) · 下载 \(shortBytes(down))").font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                VStack(alignment: .leading, spacing: 6) {
                    Text("今日直连用量").font(.caption).foregroundStyle(Theme.muted)
                    Text(shortBytes(direct)).font(.title3.monospacedDigit())
                    Text("旧版未分类用量不计入以上两项").font(.caption2).foregroundStyle(Theme.muted)
                }
                Button("查看设备与域名明细") { showDetails = true }.buttonStyle(.bordered)
            }
        }.sheet(isPresented: $showDetails) { DeviceTrafficSheet(device: "").environmentObject(model) }
    }
}
