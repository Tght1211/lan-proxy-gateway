import Foundation

struct RelayStats: Decodable {
    let ingress: [UsageAggregate]
    let upTotal: Int64
    let downTotal: Int64
    let active: [ConnectionInfo]
    let recent: [ConnectionInfo]
    let traffic: [TrafficPoint]
    let devices: [UsageAggregate]
    let services: [UsageAggregate]
    let deviceServices: [DeviceServiceAggregate]

    enum CodingKeys: String, CodingKey {
        case ingress, active, recent, traffic, devices, services
        case deviceServices = "device_services"
        case upTotal = "up_total"
        case downTotal = "down_total"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        ingress = try values.decodeIfPresent([UsageAggregate].self, forKey: .ingress) ?? []
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
    let proxyEndpoint: String?
    let ingress: String
    var isHTTPProxy: Bool { ingress == "http-proxy" }
    var ingressTitle: String { isHTTPProxy ? "HTTP 代理" : "网关接入" }
    let id: UInt64
    let srcIP: String
    let dstHost: String
    let dstPort: Int
    let proto: String
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

    var isUDP: Bool { proto == "udp" }

    enum CodingKeys: String, CodingKey {
        case proxyEndpoint = "proxy_endpoint"
        case ingress, id, up, down, proto
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
        proxyEndpoint = try values.decodeIfPresent(String.self, forKey: .proxyEndpoint)
        ingress = try values.decodeIfPresent(String.self, forKey: .ingress) ?? "gateway"
        id = try values.decode(UInt64.self, forKey: .id)
        srcIP = try values.decodeIfPresent(String.self, forKey: .srcIP) ?? "--"
        dstHost = try values.decodeIfPresent(String.self, forKey: .dstHost) ?? "--"
        dstPort = try values.decodeIfPresent(Int.self, forKey: .dstPort) ?? 0
        proto = try values.decodeIfPresent(String.self, forKey: .proto) ?? "tcp"
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
