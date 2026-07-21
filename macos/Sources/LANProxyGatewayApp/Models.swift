import Foundation

struct GatewayStatus: Decodable {
    let configured: Bool
    let running: Bool
    let egress: String
    let proxy: String?
    let dns: DNSStatus
    let quicBlock: Bool
    let gateway: NetworkStatus
    let ports: PortsStatus
    let configFile: String
    let logFile: String

    enum CodingKeys: String, CodingKey {
        case configured, running, egress, proxy, dns, gateway, ports
        case quicBlock = "quic_block"
        case configFile = "config_file"
        case logFile = "log_file"
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
    let egress: String
    let proxy: String?
    let uptimeSec: Int64
    let relay: RelayStats
    let dns: DNSStats?
    let health: HealthStats

    enum CodingKeys: String, CodingKey {
        case egress, proxy, relay, dns, health
        case uptimeSec = "uptime_sec"
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

    enum CodingKeys: String, CodingKey {
        case active, recent, traffic, devices, services
        case upTotal = "up_total"
        case downTotal = "down_total"
    }
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

    enum CodingKeys: String, CodingKey {
        case id, up, down
        case service
        case srcIP = "src_ip"
        case dstHost = "dst_host"
        case dstPort = "dst_port"
        case startedAt = "started_at"
        case endedAt = "ended_at"
        case viaProxy = "via_proxy"
    }
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

    enum CodingKeys: String, CodingKey {
        case healthy
        case lastError = "last_error"
        case checkedAt = "checked_at"
        case failCount = "fail_count"
        case latencyMS = "latency_ms"
        case jitterMS = "jitter_ms"
        case availability, history
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
    case traffic = "实时流量"
    case services = "服务分析"
    case devices = "设备洞察"
    case stability = "稳定性"
    case connections = "访问记录"
    case proxy = "出口设置"
    case settings = "设置"

    var id: String { rawValue }

    var systemImage: String {
        switch self {
        case .overview: return "command"
        case .traffic: return "waveform.path.ecg"
        case .services: return "square.stack.3d.up.fill"
        case .devices: return "desktopcomputer.and.macbook"
        case .stability: return "dot.radiowaves.left.and.right"
        case .connections: return "list.bullet.rectangle.portrait"
        case .proxy: return "arrow.triangle.branch"
        case .settings: return "gearshape"
        }
    }
}
