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
            let key = "\(c.id):\(c.startedAt.timeIntervalSince1970)"
            // Delay playback until response data or a terminal result is observable.
            // Merely opening a TCP connection does not prove the request worked.
            if !baseline && c.down == 0 && c.endedAt == nil && !c.rejected && c.status != "dial_failed" { continue }
            guard seen.insert(key).inserted else { continue }
            order.append(key)
            if !baseline { fresh.append(c) }
        }
        while order.count > 512 { seen.remove(order.removeFirst()) }
        return Array(fresh.suffix(12))
    }
}
