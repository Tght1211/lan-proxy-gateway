import SwiftUI

func aggregateHistory(_ rows: [DailyUsage], byDevice: Bool) -> [UsageAggregate] {
    Dictionary(grouping: rows, by: { byDevice ? $0.device : $0.ingress }).map { name, entries in
        UsageAggregate(name: name, up: entries.reduce(0) { $0 + $1.up }, down: entries.reduce(0) { $0 + $1.down }, connections: entries.reduce(0) { $0 + $1.connections }, lastSeen: entries.map(\.lastSeen).max() ?? .distantPast)
    }.sorted { $0.total > $1.total }
}

struct UsageHistorySheet: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    let devices: [UsageAggregate]
    let period: String
    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack {
                Text("接入设备 · \(period)").font(.title2.bold())
                Spacer()
                Button("完成") { dismiss() }
            }
            Text("按设备 IP 统计；穿透连接可能显示隧道客户端的 IP。仅统计经过本网关的流量。").font(.caption).foregroundStyle(Theme.muted)
            if devices.isEmpty { Text("所选日期暂无记录").foregroundStyle(Theme.muted) }
            ScrollView {
                LazyVStack(spacing: 12) {
                    ForEach(devices) { device in
                        HStack {
                            VStack(alignment: .leading, spacing: 4) {
                                Text(model.effectiveDeviceLabel(for: device.name).nonEmpty ?? device.name).font(.headline)
                                Text(device.name + " · 最近接入 " + device.lastSeen.formatted(date: .abbreviated, time: .shortened)).font(.caption).foregroundStyle(Theme.muted)
                            }
                            Spacer()
                            Text("↓ \(shortBytes(device.down))  ↑ \(shortBytes(device.up))").monospacedDigit()
                        }.padding(12).background(Theme.panelRaised).cornerRadius(8)
                    }
                }
            }
            Text("从启用历史统计后开始记录，重启保留。非正常断电可能丢失最后 5 秒数据。").font(.caption).foregroundStyle(Theme.muted)
        }.padding(24).frame(width: 700, height: 480)
    }
}
