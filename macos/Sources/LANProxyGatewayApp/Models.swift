import Foundation

struct GatewayStatus: Decodable {
    let httpProxy: LANHTTPProxyStatus?
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
        case httpProxy = "http_proxy"
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
    let udpRedir: Int?
    let api: Int
    let dns: Int

    enum CodingKeys: String, CodingKey {
        case redir, api, dns
        case udpRedir = "udp_redir"
    }
}

struct UDPRelayStats: Decodable {
    let sessions: Int
    let listen: String
}

struct ComponentHealth: Decodable, Identifiable {
    let name: String
    let running: Bool
    let crashes: Int

    var id: String { name }
}

struct RuntimeStats: Decodable {
    let usageHistory: [DailyUsage]?
    let httpProxy: LANHTTPProxyStatus?
    let schemaVersion: Int?
    let egress: String
    let proxy: String?
    let uptimeSec: Int64
    let relay: RelayStats
    let udpRelay: UDPRelayStats?
    let dns: DNSStats?
    let health: HealthStats
    let fallback: FallbackStats?
    let egressHealth: EgressHealthStats?
	let deviceAdaptive: DeviceAdaptiveStats?
    let components: [ComponentHealth]?

    enum CodingKeys: String, CodingKey {
        case usageHistory = "usage_history"
        case httpProxy = "http_proxy"
        case egress, proxy, relay, dns, health, fallback, components
        case schemaVersion = "schema_version"
        case uptimeSec = "uptime_sec"
        case udpRelay = "udp_relay"
		case egressHealth = "egress_health"
		case deviceAdaptive = "device_adaptive"
    }
}

struct DeviceAdaptiveStats: Decodable {
	let threshold: Int
	let windowSeconds: Int
	let directSeconds: Int
	let devices: [DeviceAdaptiveState]

	enum CodingKeys: String, CodingKey {
		case threshold, devices
		case windowSeconds = "window_seconds"
		case directSeconds = "direct_seconds"
	}

	init(from decoder: Decoder) throws {
		let v = try decoder.container(keyedBy: CodingKeys.self)
		threshold = try v.decodeIfPresent(Int.self, forKey: .threshold) ?? 5
		windowSeconds = try v.decodeIfPresent(Int.self, forKey: .windowSeconds) ?? 120
		directSeconds = try v.decodeIfPresent(Int.self, forKey: .directSeconds) ?? 900
		devices = try v.decodeIfPresent([DeviceAdaptiveState].self, forKey: .devices) ?? []
	}
}

struct DeviceAdaptiveState: Decodable, Identifiable {
	let device: String
	let mode: String
	let failureCount: Int
	let hosts: [String]
	let since: Date?
	let until: Date?
	var id: String { device }

	enum CodingKeys: String, CodingKey {
		case device, mode, hosts, since, until
		case failureCount = "failure_count"
	}

	init(from decoder: Decoder) throws {
		let v = try decoder.container(keyedBy: CodingKeys.self)
		device = try v.decode(String.self, forKey: .device)
		mode = try v.decodeIfPresent(String.self, forKey: .mode) ?? "observing"
		failureCount = try v.decodeIfPresent(Int.self, forKey: .failureCount) ?? 0
		hosts = try v.decodeIfPresent([String].self, forKey: .hosts) ?? []
		since = try v.decodeIfPresent(Date.self, forKey: .since)
		until = try v.decodeIfPresent(Date.self, forKey: .until)
	}
}

struct EgressHealthStats: Decodable {
    let proxyDown: Bool
    let since: Date?
    let actions: [EgressAction]
    let alerts: [String]
    let directFailures: [DirectFailure]
    let failureStats: [EgressFailureStat]
    let alertThreshold: Int
    let alertWindowSec: Int
    let statsWindowSec: Int

    enum CodingKeys: String, CodingKey {
        case actions, alerts
        case proxyDown = "proxy_down"
        case since
        case directFailures = "direct_failures"
        case failureStats = "failure_stats"
        case alertThreshold = "alert_threshold"
        case alertWindowSec = "alert_window_sec"
        case statsWindowSec = "stats_window_sec"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        proxyDown = try v.decodeIfPresent(Bool.self, forKey: .proxyDown) ?? false
        since = try v.decodeIfPresent(Date.self, forKey: .since)
        actions = try v.decodeIfPresent([EgressAction].self, forKey: .actions) ?? []
        alerts = try v.decodeIfPresent([String].self, forKey: .alerts) ?? []
        directFailures = try v.decodeIfPresent([DirectFailure].self, forKey: .directFailures) ?? []
        failureStats = try v.decodeIfPresent([EgressFailureStat].self, forKey: .failureStats) ?? []
        alertThreshold = try v.decodeIfPresent(Int.self, forKey: .alertThreshold) ?? 3
        alertWindowSec = try v.decodeIfPresent(Int.self, forKey: .alertWindowSec) ?? 600
        statsWindowSec = try v.decodeIfPresent(Int.self, forKey: .statsWindowSec) ?? 3600
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
    let count: Int
    let lastAt: Date?
    var id: String { "\(device)-\(host)" }

    enum CodingKeys: String, CodingKey {
        case device, host, reason, count
        case lastAt = "last_at"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        device = try v.decodeIfPresent(String.self, forKey: .device) ?? ""
        host = try v.decodeIfPresent(String.self, forKey: .host) ?? ""
        reason = try v.decodeIfPresent(String.self, forKey: .reason) ?? ""
        count = try v.decodeIfPresent(Int.self, forKey: .count) ?? 1
        lastAt = try v.decodeIfPresent(Date.self, forKey: .lastAt)
    }
}

struct EgressFailureStat: Decodable, Identifiable {
    let device: String
    let host: String
    let reason: String
    let count: Int
    let lastAt: Date?
    let suppressed: Bool
    let alerting: Bool
    var id: String { "\(device)-\(host)" }

    enum CodingKeys: String, CodingKey {
        case device, host, reason, count, suppressed, alerting
        case lastAt = "last_at"
    }

    init(from decoder: Decoder) throws {
        let v = try decoder.container(keyedBy: CodingKeys.self)
        device = try v.decodeIfPresent(String.self, forKey: .device) ?? ""
        host = try v.decodeIfPresent(String.self, forKey: .host) ?? ""
        reason = try v.decodeIfPresent(String.self, forKey: .reason) ?? ""
        count = try v.decodeIfPresent(Int.self, forKey: .count) ?? 0
        lastAt = try v.decodeIfPresent(Date.self, forKey: .lastAt)
        suppressed = try v.decodeIfPresent(Bool.self, forKey: .suppressed) ?? false
        alerting = try v.decodeIfPresent(Bool.self, forKey: .alerting) ?? false
    }
}

struct FallbackStats: Decodable {
    let ignored: [String]
    let threshold: Int
    let windowHours: Int
    let candidates: [FallbackCandidate]
    let learned: [RoutingRule]

    enum CodingKeys: String, CodingKey {
        case threshold, candidates, learned, ignored
        case windowHours = "window_hours"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        ignored = try values.decodeIfPresent([String].self, forKey: .ignored) ?? []
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

struct LANHTTPProxyStatus: Decodable, Equatable {
    let enabled: Bool
    let port: Int
    let auth: String
    let username: String
    let passwordSet: Bool
    enum CodingKeys: String, CodingKey {
        case enabled, port, auth, username
        case passwordSet = "password_set"
    }
}

struct DailyUsage: Decodable {
    let date: String
    let device: String
    let ingress: String
    let up: Int64
    let down: Int64
    let connections: Int64
    let lastSeen: Date
    enum CodingKeys: String, CodingKey {
        case date, device, ingress, up, down, connections
        case lastSeen = "last_seen"
    }
}
