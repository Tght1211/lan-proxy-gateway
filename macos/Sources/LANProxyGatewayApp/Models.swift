import Foundation

struct GatewayStatus: Decodable {
    let configured: Bool
    let running: Bool
    let egress: String
    let proxy: String?
    let routing: [RoutingRule]?
    let dns: DNSStatus
    let quicBlock: Bool
    let gateway: NetworkStatus
    let ports: PortsStatus
    let configFile: String
    let logFile: String

    enum CodingKeys: String, CodingKey {
        case configured, running, egress, proxy, routing, dns, gateway, ports
        case quicBlock = "quic_block"
        case configFile = "config_file"
        case logFile = "log_file"
    }
}

struct RoutingRule: Codable, Identifiable, Equatable {
    var id = UUID()
    var type: String
    var value: String
    var action: String
    var group: String
    var learned: Bool

    init(id: UUID = UUID(), type: String, value: String, action: String, group: String = "", learned: Bool = false) {
        self.id = id
        self.type = type
        self.value = value
        self.action = action
        self.group = group
        self.learned = learned
    }

    enum CodingKeys: String, CodingKey { case type, value, action, group, learned }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = UUID()
        type = try values.decode(String.self, forKey: .type)
        value = try values.decode(String.self, forKey: .value)
        action = try values.decode(String.self, forKey: .action)
        group = try values.decodeIfPresent(String.self, forKey: .group) ?? ""
        learned = try values.decodeIfPresent(Bool.self, forKey: .learned) ?? false
    }
}

struct DNSStatus: Decodable {
    let enabled: Bool
    let port: Int
    let hijack: Bool
    let fakeIP: Bool

    enum CodingKeys: String, CodingKey {
        case enabled, port, hijack
        case fakeIP = "fake_ip"
    }
}

struct NetworkStatus: Decodable {
    let ipForward: Bool
    let interface: String
    let localIP: String
    let router: String

    enum CodingKeys: String, CodingKey {
        case ipForward = "IPForward"
        case interface = "Interface"
        case localIP = "LocalIP"
        case router = "Router"
    }
}

struct PortsStatus: Decodable {
    let redir: Int
    let api: Int
    let dns: Int
}

struct RuntimeStats: Decodable {
    let schemaVersion: Int?
    let egress: String
    let proxy: String?
    let uptimeSec: Int64
    let relay: RelayStats
    let dns: DNSStats?
    let health: HealthStats
    let fallback: FallbackStats?
    let egressHealth: EgressHealthStats?

    enum CodingKeys: String, CodingKey {
        case egress, proxy, relay, dns, health, fallback
        case schemaVersion = "schema_version"
        case uptimeSec = "uptime_sec"
        case egressHealth = "egress_health"
    }
}

struct EgressHealthStats: Decodable {
    let proxyDown: Bool
    let since: Date?
    let actions: [EgressAction]
    let alerts: [String]
    let directFailures: [DirectFailure]

    enum CodingKeys: String, CodingKey {
        case actions, alerts
        case proxyDown = "proxy_down"
        case since
        case directFailures = "direct_failures"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        proxyDown = try v.decodeIfPresent(Bool.self, forKey: .proxyDown) ?? false
        since = try v.decodeIfPresent(Date.self, forKey: .since)
        actions = try v.decodeIfPresent([EgressAction].self, forKey: .actions) ?? []
        alerts = try v.decodeIfPresent([String].self, forKey: .alerts) ?? []
        directFailures = try v.decodeIfPresent([DirectFailure].self, forKey: .directFailures) ?? []
    }
}

struct EgressAction: Decodable, Identifiable {
    let at: Date
    let text: String
    var id: String { "\(at.timeIntervalSince1970)-\(text)" }
}

struct DirectFailure: Decodable, Identifiable {
    let device: String
    let host: String
    let reason: String
    var id: String { "\(device)-\(host)" }
}

struct FallbackStats: Decodable {
    let threshold: Int
    let windowHours: Int
    let candidates: [FallbackCandidate]
    let learned: [RoutingRule]

    enum CodingKeys: String, CodingKey {
        case threshold, candidates, learned
        case windowHours = "window_hours"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        threshold = try values.decodeIfPresent(Int.self, forKey: .threshold) ?? 3
        windowHours = try values.decodeIfPresent(Int.self, forKey: .windowHours) ?? 24
        candidates = try values.decodeIfPresent([FallbackCandidate].self, forKey: .candidates) ?? []
        learned = try values.decodeIfPresent([RoutingRule].self, forKey: .learned) ?? []
    }
}

struct FallbackCandidate: Decodable, Identifiable {
    let host: String
    let count: Int
    let lastAt: Date
    var id: String { host }

    enum CodingKeys: String, CodingKey {
        case host, count
        case lastAt = "last_at"
    }
}

struct RelayStats: Decodable {
    let upTotal: Int64
    let downTotal: Int64
    let active: [ConnectionInfo]
    let recent: [ConnectionInfo]
    let traffic: [TrafficPoint]
    let devices: [UsageAggregate]
    let services: [UsageAggregate]
    let deviceServices: [DeviceServiceAggregate]

    enum CodingKeys: String, CodingKey {
        case active, recent, traffic, devices, services
        case deviceServices = "device_services"
        case upTotal = "up_total"
        case downTotal = "down_total"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        upTotal = try values.decodeIfPresent(Int64.self, forKey: .upTotal) ?? 0
        downTotal = try values.decodeIfPresent(Int64.self, forKey: .downTotal) ?? 0
        active = try values.decodeIfPresent([ConnectionInfo].self, forKey: .active) ?? []
        recent = try values.decodeIfPresent([ConnectionInfo].self, forKey: .recent) ?? []
        traffic = try values.decodeIfPresent([TrafficPoint].self, forKey: .traffic) ?? []
        devices = try values.decodeIfPresent([UsageAggregate].self, forKey: .devices) ?? []
        services = try values.decodeIfPresent([UsageAggregate].self, forKey: .services) ?? []
        deviceServices = try values.decodeIfPresent([DeviceServiceAggregate].self, forKey: .deviceServices) ?? []
    }
}

struct DeviceServiceAggregate: Decodable, Identifiable {
    let device: String
    let services: [UsageAggregate]
    var id: String { device }
}

struct ConnectionInfo: Decodable, Identifiable {
    let id: UInt64
    let srcIP: String
    let dstHost: String
    let dstPort: Int
    let service: String
    let up: Int64
    let down: Int64
    let startedAt: Date
    let endedAt: Date?
    let viaProxy: Bool
    let rejected: Bool
    let status: String
    let failure: String
    let fallback: Bool

    enum CodingKeys: String, CodingKey {
        case id, up, down
        case service
        case rejected
        case status
        case failure
        case fallback
        case srcIP = "src_ip"
        case dstHost = "dst_host"
        case dstPort = "dst_port"
        case startedAt = "started_at"
        case endedAt = "ended_at"
        case viaProxy = "via_proxy"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(UInt64.self, forKey: .id)
        srcIP = try values.decodeIfPresent(String.self, forKey: .srcIP) ?? "--"
        dstHost = try values.decodeIfPresent(String.self, forKey: .dstHost) ?? "--"
        dstPort = try values.decodeIfPresent(Int.self, forKey: .dstPort) ?? 0
        let decodedService = try values.decodeIfPresent(String.self, forKey: .service) ?? "未解析域名"
        service = ["未识别流量", "IP 地址流量"].contains(decodedService) ? "未解析域名" : decodedService
        up = try values.decodeIfPresent(Int64.self, forKey: .up) ?? 0
        down = try values.decodeIfPresent(Int64.self, forKey: .down) ?? 0
        startedAt = try values.decode(Date.self, forKey: .startedAt)
        endedAt = try values.decodeIfPresent(Date.self, forKey: .endedAt)
        viaProxy = try values.decodeIfPresent(Bool.self, forKey: .viaProxy) ?? false
        rejected = try values.decodeIfPresent(Bool.self, forKey: .rejected) ?? false
        status = try values.decodeIfPresent(String.self, forKey: .status) ?? (rejected ? "rejected" : "")
        failure = try values.decodeIfPresent(String.self, forKey: .failure) ?? ""
        fallback = try values.decodeIfPresent(Bool.self, forKey: .fallback) ?? false
    }

    var outcome: ConnectionOutcome {
        if status == "rejected" || rejected { return .rejected }
        if status == "dial_failed" { return .failed(failure.nonEmptyValue ?? "连接失败") }
        if endedAt == nil { return .active }
        if up + down == 0 { return .noData }
        return .success
    }
}

enum ConnectionOutcome: Equatable {
    case active
    case success
    case noData
    case failed(String)
    case rejected

    var label: String {
        switch self {
        case .active: return "活跃"
        case .success: return "成功"
        case .noData: return "无数据"
        case .failed(let reason): return "失败·\(reason)"
        case .rejected: return "拒绝"
        }
    }
}

private extension String {
    var nonEmptyValue: String? { isEmpty ? nil : self }
}

struct TrafficPoint: Decodable, Identifiable {
    let at: Date
    let up: Int64
    let down: Int64
    var id: Date { at }
}

struct UsageAggregate: Decodable, Identifiable {
    let name: String
    let up: Int64
    let down: Int64
    let connections: Int64
    let lastSeen: Date
    var id: String { name }
    var total: Int64 { up + down }
    var displayName: String { ["未识别流量", "IP 地址流量"].contains(name) ? "未解析域名" : name }

    enum CodingKeys: String, CodingKey {
        case name, up, down, connections
        case lastSeen = "last_seen"
    }
}

struct DNSStats: Decodable {
    let queries: Int64
    let fakeAnswered: Int64
    let forwarded: Int64
    let failures: Int64
    let poolSize: Int

    enum CodingKeys: String, CodingKey {
        case queries, forwarded, failures
        case fakeAnswered = "fake_answered"
        case poolSize = "pool_size"
    }
}

struct HealthStats: Decodable {
    let healthy: Bool
    let lastError: String?
    let checkedAt: Date?
    let failCount: Int
    let latencyMS: Double
    let jitterMS: Double
    let availability: Double
    let history: [ProbePoint]
    let egressIdentity: EgressIdentity?

    enum CodingKeys: String, CodingKey {
        case healthy
        case lastError = "last_error"
        case checkedAt = "checked_at"
        case failCount = "fail_count"
        case latencyMS = "latency_ms"
        case jitterMS = "jitter_ms"
        case availability, history
        case egressIdentity = "egress_identity"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        healthy = try values.decodeIfPresent(Bool.self, forKey: .healthy) ?? false
        lastError = try values.decodeIfPresent(String.self, forKey: .lastError)
        checkedAt = try values.decodeIfPresent(Date.self, forKey: .checkedAt)
        failCount = try values.decodeIfPresent(Int.self, forKey: .failCount) ?? 0
        latencyMS = try values.decodeIfPresent(Double.self, forKey: .latencyMS) ?? 0
        jitterMS = try values.decodeIfPresent(Double.self, forKey: .jitterMS) ?? 0
        availability = try values.decodeIfPresent(Double.self, forKey: .availability) ?? 0
        history = try values.decodeIfPresent([ProbePoint].self, forKey: .history) ?? []
        egressIdentity = try values.decodeIfPresent(EgressIdentity.self, forKey: .egressIdentity)
    }
}

struct EgressIdentity: Decodable {
    let ip: String
    let countryCode: String?
    let region: String?
    let city: String?
    let isp: String?
    let checkedAt: Date

    enum CodingKeys: String, CodingKey {
        case ip, region, city, isp
        case countryCode = "country_code"
        case checkedAt = "checked_at"
    }
}

struct ProbePoint: Decodable, Identifiable {
    let at: Date
    let latencyMS: Double
    let ok: Bool
    var id: Date { at }

    enum CodingKeys: String, CodingKey {
        case at, ok
        case latencyMS = "latency_ms"
    }
}

enum AppSection: String, CaseIterable, Identifiable {
    case overview = "网络总览"
    case devices = "设备与服务"
    case connections = "访问记录"
    case settings = "设置"

    var id: String { rawValue }

    var systemImage: String {
        switch self {
        case .overview: return "command"
        case .devices: return "desktopcomputer.and.macbook"
        case .connections: return "list.bullet.rectangle.portrait"
        case .settings: return "gearshape"
        }
    }
}
