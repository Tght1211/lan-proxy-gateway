import Foundation

/// Presentation retention only: never deletes usage history, labels, or device rules.
struct NetworkDeviceActivity {
    enum State { case active, inactive, hidden }
    private struct Observation { let up: Int64, down: Int64; let trafficAt: Date? }
    private var observations: [String: Observation] = [:]
    private var uptime: Int64?

    mutating func ingest(_ devices: [UsageAggregate], uptime: Int64, at now: Date) {
        if let previous = self.uptime, uptime < previous { observations = [:] }
        self.uptime = uptime
        let current = Set(devices.map(\.name))
        observations = observations.filter { current.contains($0.key) || now.timeIntervalSince($0.value.trafficAt ?? .distantPast) <= 86_400 }
        for device in devices {
            let previous = observations[device.name]
            let trafficAt: Date?
            if let exact = device.lastTrafficAt { trafficAt = min(now, exact) }
            else if let previous {
                trafficAt = device.up > previous.up || device.down > previous.down ? now : previous.trafficAt
            } else {
                // Backward-compatible baseline for older cores. Subsequent activity
                // comes from byte deltas, never an idle connection's open/close time.
                trafficAt = device.total > 0 ? min(now, device.lastSeen) : nil
            }
            observations[device.name] = Observation(up: device.up, down: device.down, trafficAt: trafficAt)
        }
    }
    func lastTraffic(for device: UsageAggregate) -> Date? {
        device.lastTrafficAt ?? observations[device.name]?.trafficAt
    }
    func state(for device: UsageAggregate, at now: Date) -> State {
        guard let at = lastTraffic(for: device), device.total > 0 else { return .hidden }
        let age = now.timeIntervalSince(at)
        return age > 86_400 ? .hidden : age > 600 ? .inactive : .active
    }
}

enum NetworkRuleRevision {
    enum Change { case unchanged, reload, conflict }
    static func signature(_ rules: [RoutingRule]) -> String {
        let encoder = JSONEncoder(); encoder.outputFormatting = .sortedKeys
        return String(data: (try? encoder.encode(rules)) ?? Data(), encoding: .utf8) ?? ""
    }
    static func assess(baseline: [RoutingRule], latest: [RoutingRule], draftChanged: Bool) -> Change {
        guard signature(baseline) != signature(latest) else { return .unchanged }
        return draftChanged ? .conflict : .reload
    }
}

enum NetworkTopologyLayout {
    static let topInset: Double = 64
    static let bottomInset: Double = 82
    static let deviceSpacing: Double = 96

    static func orderedDevices(_ devices: [UsageAggregate], inactive: Set<String>) -> [UsageAggregate] {
        devices.sorted { first, second in
            let firstInactive = inactive.contains(first.name)
            let secondInactive = inactive.contains(second.name)
            if firstInactive != secondInactive { return !firstInactive }
            return first.name.localizedStandardCompare(second.name) == .orderedAscending
        }
    }
    static func minimumHeight(deviceCount: Int) -> Double {
        topInset + bottomInset + max(192, Double(max(0, deviceCount - 1)) * deviceSpacing)
    }
    static func verticalPosition(row: Double, height: Double) -> Double {
        topInset + max(0, height - topInset - bottomInset) * row / 3
    }
}

enum NetworkOverviewSizing {
    static let spacing: Double = 22
    static func exitColumnCount(width: Double) -> Int {
        width >= 900 ? 3 : width >= 560 ? 2 : 1
    }
}

struct NetworkTrafficSummary {
    let hasData: Bool
    let proxy: Int64
    let direct: Int64
    let total: Int64
    var unclassified: Int64 { max(0, total - proxy - direct) }

    init(rows: [DailyUsage]?, at: Date = Date(), calendar: Calendar = .current) {
        hasData = rows != nil
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = calendar.timeZone
        formatter.dateFormat = "yyyy-MM-dd"
        let start = calendar.dateInterval(of: .quarter, for: at)?.start ?? calendar.startOfDay(for: at)
        let lower = formatter.string(from: start)
        let upper = formatter.string(from: at)
        var proxyBytes: Int64 = 0
        var directBytes: Int64 = 0
        var totalBytes: Int64 = 0
        for row in rows ?? [] where row.date >= lower && row.date <= upper {
            totalBytes += row.total
            if row.egress == "proxy" { proxyBytes += row.total }
            if row.egress == "direct" { directBytes += row.total }
        }
        proxy = proxyBytes
        direct = directBytes
        total = totalBytes
    }
}

struct NetworkExitUsage {
    var up: Int64 = 0
    var down: Int64 = 0
    var connections: Int64 = 0
    var total: Int64 { up + down }
}

struct NetworkExitUsageSummary {
    let hasData: Bool
    private var exits: [String: NetworkExitUsage] = [:]

    init(rows: [DailyUsage]?, date: String) {
        hasData = rows != nil
        for row in rows ?? [] where row.date == date {
            guard let exit = row.egress, exit == "proxy" || exit == "direct" else { continue }
            exits[exit, default: .init()].up += row.up
            exits[exit, default: .init()].down += row.down
            exits[exit, default: .init()].connections += row.connections
        }
    }

    func usage(for exit: String) -> NetworkExitUsage { exits[exit] ?? .init() }
}

enum NetworkExitCondition: Equatable {
    case healthy, degraded, unavailable, unknown, stopped

    var title: String {
        switch self {
        case .healthy: return "响应正常"
        case .degraded: return "响应异常"
        case .unavailable: return "检测失败"
        case .unknown: return "待检测"
        case .stopped: return "已停止"
        }
    }
}

struct NetworkGlobalHealth {
    let conditions: [NetworkExitCondition]
    var healthyCount: Int { conditions.filter { $0 == .healthy }.count }
    var hasFailure: Bool { conditions.contains(.degraded) || conditions.contains(.unavailable) }
    var title: String {
        if conditions.allSatisfy({ $0 == .stopped }) { return "网关已停止" }
        if healthyCount == conditions.count { return "全部正常" }
        if hasFailure { return healthyCount > 0 ? "部分异常" : "检测异常" }
        return healthyCount > 0 ? "检测未齐" : "等待检测"
    }

    init(stats: RuntimeStats?, proxyConfigured: Bool, isRunning: Bool, at: Date = Date(), includeProbe: Bool = true) {
        let exits = proxyConfigured ? ["proxy", "direct"] : ["direct"]
        conditions = exits.map {
            Self.condition(for: $0, stats: stats, isRunning: isRunning, at: at, includeProbe: includeProbe)
        }
    }

    static func condition(for exit: String, stats: RuntimeStats?, isRunning: Bool,
                          at: Date = Date(), includeProbe: Bool = true) -> NetworkExitCondition {
        guard isRunning else { return .stopped }
        guard let stats else { return .unknown }
        let connections = stats.relay.active + stats.relay.recent
        var latest: (at: Date, condition: NetworkExitCondition)?
        for connection in connections {
            guard !connection.rejected, (connection.viaProxy ? "proxy" : "direct") == exit else { continue }
            let timestamp: Date
            if case .failed = connection.outcome {
                timestamp = connection.endedAt ?? connection.startedAt
            } else {
                timestamp = connection.lastTrafficAt ?? connection.endedAt ?? connection.startedAt
            }
            guard (0...300).contains(at.timeIntervalSince(timestamp)) else { continue }
            if let latest, timestamp <= latest.at { continue }
            switch connection.outcome {
            case .active, .success: latest = (timestamp, .healthy)
            case .failed: latest = (timestamp, .degraded)
            case .waiting, .noData, .rejected: continue
            }
        }
        if includeProbe,
           let probe = stats.health(for: exit)?.history.last(where: { (0..<30).contains(at.timeIntervalSince($0.at)) }),
           latest.map({ probe.at >= $0.at }) ?? true {
            return probe.ok ? .healthy : .unavailable
        }
        return latest?.condition ?? .unknown
    }
}

struct ConnectionListFilter: Equatable {
    var search = ""
    var outcome = "全部"
    var device = "全部设备"
    var route = "全部出口"
    var unresolvedOnly = false
}

struct ConnectionListProjection {
    let connections: [ConnectionInfo]
    let devices: [String]
    let counts: [String: Int]
    let responseRate: String
    var total: Int { counts["全部", default: 0] }

    init(records: [ConnectionInfo], filter: ConnectionListFilter = .init(), labels: [String: String] = [:]) {
        var visible: [ConnectionInfo] = []
        var addresses = Set<String>()
        var totals: [String: Int] = [:]
        var responses = 0
        for record in records {
            addresses.insert(record.srcIP)
            let matchesSearch = filter.search.isEmpty || record.srcIP.localizedCaseInsensitiveContains(filter.search) ||
                (labels[record.srcIP] ?? "").localizedCaseInsensitiveContains(filter.search) ||
                record.dstHost.localizedCaseInsensitiveContains(filter.search) ||
                record.service.localizedCaseInsensitiveContains(filter.search)
            let matchesRoute: Bool
            switch filter.route {
            case "代理": matchesRoute = record.viaProxy && !record.rejected
            case "直连": matchesRoute = !record.viaProxy && !record.rejected
            case "拒绝": matchesRoute = record.rejected
            default: matchesRoute = true
            }
            guard matchesSearch, matchesRoute,
                  filter.device == "全部设备" || record.srcIP == filter.device,
                  !filter.unresolvedOnly || record.service == "未解析域名" else { continue }
            let outcome = Self.outcomeName(record)
            totals["全部", default: 0] += 1
            totals[outcome, default: 0] += 1
            if record.fallback { totals["出口回退", default: 0] += 1 }
            if !record.rejected && record.down > 0 { responses += 1 }
            if filter.outcome == "全部" || filter.outcome == outcome ||
                (filter.outcome == "出口回退" && record.fallback) {
                visible.append(record)
            }
        }
        connections = visible
        devices = addresses.sorted()
        counts = totals
        let total = totals["全部", default: 0]
        responseRate = total > 0 ? "\(responses * 100 / total)%" : "--"
    }

    private static func outcomeName(_ connection: ConnectionInfo) -> String {
        switch connection.outcome {
        case .active: return "活跃"
        case .waiting: return "等待响应"
        case .success: return "成功"
        case .noData: return "无数据"
        case .failed: return "失败"
        case .rejected: return "拒绝"
        }
    }
}

struct NetworkFloatingTextMotion {
    let lift: Double
    let opacity: Double

    static func frame(at progress: Double, reducedMotion: Bool = false) -> Self {
        guard !reducedMotion else { return Self(lift: 0, opacity: 1) }
        let fraction = min(1, max(0, progress))
        return Self(lift: 28 * (1 - pow(1 - fraction, 2)),
                    opacity: min(1, fraction / 0.12) * min(1, (1 - fraction) / 0.25))
    }
}

/// UI projections only. Never changes routing or guesses whether a PAC script was used.
enum NetworkIngress: String {
    case wifi, gateway, http, unknown
    static func resolve(_ connection: ConnectionInfo, hotspot: HotspotStatus?) -> Self {
        if connection.isHTTPProxy { return .http }
        guard connection.ingress == "gateway" else { return .unknown }
        return hotspot?.containsClient(connection.srcIP) == true ? .wifi : .gateway
    }
    var title: String {
        switch self {
        case .wifi: return "Wi-Fi 热点"
        case .gateway: return "静态网关"
        case .http: return "HTTP / PAC"
        case .unknown: return "未知入口"
        }
    }
}

enum NetworkDomainColor {
    static func index(_ domain: String) -> Int {
        let normalized = domain.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "."))
        return Int(normalized.utf8.reduce(UInt64(14695981039346656037)) { ($0 ^ UInt64($1)) &* 1099511628211 } % 6)
    }
}

struct NetworkRequestPresentation {
    enum Tone { case success, delayed, failure, neutral }
    static let delayThresholdMS: Int64 = 500
    let connection: ConnectionInfo

    var tone: Tone {
        switch connection.outcome {
        case .active, .success:
            return (connection.responseMS ?? 0) > Self.delayThresholdMS ? .delayed : .success
        case .failed, .rejected: return .failure
        case .waiting, .noData: return .neutral
        }
    }
    var label: String {
        switch connection.outcome {
        case .active, .success:
            guard let milliseconds = connection.responseMS, milliseconds >= 0 else { return "已响应 · 耗时未记录" }
            return "首响应 \(milliseconds) ms"
        default: return connection.outcome.label
        }
    }
    var eventPhase: String {
        switch connection.outcome {
        case .active, .success: return "response"
        case .waiting: return "waiting"
        case .noData: return "no-data"
        case .failed: return "failed"
        case .rejected: return "rejected"
        }
    }
}

enum NetworkFlowHue: String { case green, yellow, red, muted, purple, teal, orange, pink, blue, indigo, cyan, mint }

enum NetworkFlowPalette: String, CaseIterable, Identifiable {
    case status, spectrum, ocean, aurora, sunset
    var id: String { rawValue }
    var title: String {
        switch self {
        case .status: return "纯色状态"
        case .spectrum: return "缤纷"
        case .ocean: return "海洋"
        case .aurora: return "极光"
        case .sunset: return "落日"
        }
    }
    var hues: [NetworkFlowHue] {
        switch self {
        case .status: return [.green]
        case .spectrum: return [.purple, .teal, .orange, .pink, .blue, .indigo]
        case .ocean: return [.cyan, .teal, .blue, .indigo, .mint, .purple]
        case .aurora: return [.mint, .purple, .teal, .pink, .indigo, .cyan]
        case .sunset: return [.orange, .pink, .purple, .orange, .pink, .indigo]
        }
    }
    var detail: String {
        self == .status ? "正常绿色 · 首响应超过 500 ms 黄色 · 错误红色" : "同域名同色 · 首响应超过 500 ms 黄色 · 错误红色"
    }
    func hue(for presentation: NetworkRequestPresentation) -> NetworkFlowHue {
        switch presentation.tone {
        case .failure: return .red
        case .delayed: return .yellow
        case .neutral: return .muted
        case .success: return hues[NetworkDomainColor.index(presentation.connection.dstHost) % hues.count]
        }
    }
}

/// Polling can repeat a connection many times. Replay only new identities, with a bounded cache.
struct NetworkEventFeed {
    private var seen = Set<String>()
    private var order: [String] = []
    private var previousUptime: Int64?
    var retainedCount: Int { seen.count }
    mutating func reset() { seen = []; order = []; previousUptime = nil }
    mutating func ingest(_ connections: [ConnectionInfo], uptime: Int64) -> [ConnectionInfo] {
        let baseline = previousUptime == nil || uptime < (previousUptime ?? 0)
        if baseline { seen = []; order = [] }
        previousUptime = uptime
        var fresh: [ConnectionInfo] = []
        for c in connections.sorted(by: { $0.startedAt < $1.startedAt }).suffix(512) {
            let key = "\(c.id):\(c.startedAt.timeIntervalSince1970):\(NetworkRequestPresentation(connection: c).eventPhase)"
            if c.up == 0 && c.down == 0 && c.outcome == .waiting { continue }
            guard seen.insert(key).inserted else { continue }
            order.append(key)
            if !baseline { fresh.append(c) }
        }
        while order.count > 512 { seen.remove(order.removeFirst()) }
        return Array(fresh.suffix(12))
    }
}
