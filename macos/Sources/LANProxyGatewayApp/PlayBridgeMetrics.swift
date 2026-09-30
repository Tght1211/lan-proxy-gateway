import Foundation

/// UI modes have no gateway side effects. Unknown persisted values use classic.
enum InterfaceMode: String, CaseIterable, Identifiable {
    case classic, playBridge
    var id: String { rawValue }
    var title: String { self == .classic ? "经典 · LAN Proxy Gateway" : "PlayBridge" }
    static func restored(_ value: String?) -> Self { Self(rawValue: value ?? "") ?? .classic }
}

struct HotspotRatePoint: Identifiable {
    let at: Date
    let up: Double
    let down: Double
    var id: Date { at }
}

struct HotspotTrafficSampler {
    private var baseline: (key: String, at: Date, up: Int64, down: Int64, uptime: Int64)?
    private(set) var points: [HotspotRatePoint] = []

    mutating func reset() { baseline = nil; points = [] }

    mutating func record(key: String, at: Date, up: Int64, down: Int64, uptime: Int64) {
        defer { baseline = (key, at, up, down, uptime) }
        guard let previous = baseline else { return }
        let elapsed = at.timeIntervalSince(previous.at)
        guard previous.key == key, uptime >= previous.uptime,
              up >= previous.up, down >= previous.down,
              elapsed > 0, elapsed <= 15 else { points = []; return }
        points.append(HotspotRatePoint(at: at, up: Double(up - previous.up) / elapsed,
                                      down: Double(down - previous.down) / elapsed))
        points.removeAll { at.timeIntervalSince($0.at) > 300 }
    }
}

func hotspotUsage(_ rows: [DailyUsage], network: HotspotStatus, date: String? = nil) -> [DailyUsage] {
    rows.filter { $0.ingress == "gateway" && network.containsClient($0.device) && (date == nil || $0.date == date) }
}

func usageDate(_ now: Date = Date()) -> String {
    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.dateFormat = "yyyy-MM-dd"
    return formatter.string(from: now)
}

/// The selected duration controls the axis even when only a few samples exist.
struct HotspotChartWindow {
    let domain: ClosedRange<Date>
    let points: [HotspotRatePoint]

    init(points: [HotspotRatePoint], seconds: Int, now: Date) {
        let end = points.last?.at ?? now
        let range = end.addingTimeInterval(-Double(seconds))...end
        domain = range
        self.points = points.filter { range.contains($0.at) }
    }
}
