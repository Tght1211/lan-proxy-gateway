import Foundation

@main struct ModelDecodingTests {
    static func main() throws { try ModelDecodingTests().testUsageCompatibility(); try ModelDecodingTests().testIngressCompatibility(); try ModelDecodingTests().testHotspotCompatibility(); try ModelDecodingTests().testHotspotControlStates(); try ModelDecodingTests().testPlayBridgeMetrics(); ModelDecodingTests().testThroughputWindows(); ModelDecodingTests().testUsageDates(); try ModelDecodingTests().testResponseOutcomes(); print("Model decoding tests passed") }
    func testUsageDates() {
        let date = ISO8601DateFormatter().date(from: "2026-01-01T00:30:00Z")!
        precondition(usageDate(date, timeZone: TimeZone(identifier: "Asia/Shanghai")!) == "2026-01-01")
        precondition(usageDate(date, timeZone: TimeZone(identifier: "America/Los_Angeles")!) == "2025-12-31")
        let leapDay = ISO8601DateFormatter().date(from: "2024-02-29T12:30:00Z")!
        precondition(usageDate(leapDay, timeZone: TimeZone(secondsFromGMT: 0)!) == "2024-02-29")
        for zone in ["Asia/Shanghai", "America/Los_Angeles", "UTC"] {
            let timeZone = TimeZone(identifier: zone)!
            let original = DateFormatter()
            original.locale = Locale(identifier: "en_US_POSIX")
            original.timeZone = timeZone
            original.dateFormat = "yyyy-MM-dd"
            let offsets: [TimeInterval] = [-86400, 0, 86400, 86400 * 31, 86400 * 365]
            for offset in offsets {
                let sample = date.addingTimeInterval(offset)
                precondition(usageDate(sample, timeZone: timeZone) == original.string(from: sample))
            }
        }
    }
    func testResponseOutcomes() throws {
        func row(_ fields: String) throws -> ConnectionInfo {
            try JSONDecoder().decode(ConnectionInfo.self, from: Data(("{\"id\":1,\"started_at\":0," + fields + "}").utf8))
        }
        let waiting = try row("\"up\":1366,\"down\":0")
        precondition(waiting.outcome.label == "等待响应")
        let failed = try row("\"up\":1366,\"down\":0,\"ended_at\":10")
        precondition(failed.outcome == .failed("未收到响应"))
        let response = try row("\"up\":1366,\"down\":100,\"ended_at\":10")
        precondition(response.outcome == .success)
    }
    func testUsageCompatibility() throws {
        let old = #"{"date":"2026-09-28","device":"device","ingress":"gateway","up":10,"down":20,"connections":1,"last_seen":0}"#
        let row = try JSONDecoder().decode(DailyUsage.self, from: Data(old.utf8))
        precondition(row.egress == nil && row.proxyEndpoint == nil && row.total == 30)
        let current = #"{"date":"2026-09-28","device":"device","ingress":"gateway","up":10,"down":90,"connections":1,"last_seen":0,"egress":"proxy","proxy_endpoint":"127.0.0.1:7897","destination":"youtube.com","service":"YouTube"}"#
        let usage = try JSONDecoder().decode(DailyUsage.self, from: Data(current.utf8))
        precondition(usage.egress == "proxy" && usage.proxyEndpoint == "127.0.0.1:7897" && usage.destination == "youtube.com" && usage.total == 100)
    }

    func testIngressCompatibility() throws {
        let old = try JSONDecoder().decode(RelayStats.self, from: Data("{\"recent\":[{\"id\":1,\"started_at\":0}]}".utf8))
        precondition(old.ingress.isEmpty && old.recent[0].ingressTitle == "网关接入")
        let current = try JSONDecoder().decode(RelayStats.self, from: Data("{\"active\":[{\"id\":2,\"started_at\":0,\"ingress\":\"http-proxy\"}]}".utf8))
        precondition(current.active[0].isHTTPProxy && current.active[0].ingressTitle == "HTTP 代理")
    }

    func testHotspotCompatibility() throws {
        let legacy = #"{"configured":true,"running":false,"egress":"direct","dns":{"enabled":true,"port":1053,"hijack":false,"fake_ip":false},"quic_block":false,"gateway":{"IPForward":false,"Interface":"en0","LocalIP":"192.168.1.10","Router":"192.168.1.1"},"ports":{"redir":17892,"api":19090,"dns":1053},"config_file":"test.yaml","log_file":"test.log"}"#
        let old = try JSONDecoder().decode(GatewayStatus.self, from: Data(legacy.utf8))
        precondition(old.accessMode == nil)
        let json = #"{"supported":true,"available":true,"enabled":true,"applied":false,"message":"Waiting for core","interface":"bridge100","ip":"192.168.2.1","cidr":"192.168.2.0/24","uplink":"en0"}"#
        let hotspot = try JSONDecoder().decode(HotspotStatus.self, from: Data(json.utf8))
        precondition(hotspot.available && hotspot.enabled && !hotspot.applied)
    }

    func testHotspotControlStates() throws {
        func runtime(enabled: Bool, applied: Bool) throws -> HotspotStatus {
            let json = "{\"supported\":true,\"available\":true,\"enabled\":\(enabled),\"applied\":\(applied),\"message\":\"waiting\",\"interface\":\"bridge100\",\"ip\":\"192.168.2.1\",\"cidr\":\"192.168.2.0/24\",\"uplink\":\"en0\"}"
            return try JSONDecoder().decode(HotspotStatus.self, from: Data(json.utf8))
        }
        let ready = try runtime(enabled: true, applied: true)
        precondition(ready.containsClient("192.168.2.3"))
        precondition(!ready.containsClient("192.168.12.3"))
        precondition(!ready.containsClient("192.168.2.1"))
        precondition(!ready.containsClient("192.168.2.999"))
        let pending = try runtime(enabled: true, applied: false)
        precondition(HotspotControlState.resolve(operation: nil, error: nil, desired: true, running: true, hasStats: true, runtime: nil) == .needsCoreUpdate)
        precondition(HotspotControlState.resolve(operation: nil, error: nil, desired: true, running: true, hasStats: false, runtime: nil) == .unavailable)
        precondition(HotspotControlState.resolve(operation: "enable", error: nil, desired: true, running: true, hasStats: true, runtime: ready) == .enabling)
        precondition(HotspotControlState.resolve(operation: "verify", error: nil, desired: true, running: true, hasStats: true, runtime: ready) == .verifying)
        precondition(HotspotControlState.resolve(operation: nil, error: "授权被取消", desired: true, running: true, hasStats: true, runtime: nil) == .failed)
        precondition(HotspotControlState.resolve(operation: nil, error: nil, desired: true, running: true, hasStats: true, runtime: pending) == .waiting)
        precondition(HotspotControlState.resolve(operation: nil, error: nil, desired: true, running: true, hasStats: true, runtime: ready) == .enabled)
        precondition(HotspotControlState.resolve(operation: nil, error: nil, desired: false, running: false, hasStats: false, runtime: nil) == .off)
    }

    func testPlayBridgeMetrics() throws {
        precondition(InterfaceMode.restored(nil) == .classic)
        precondition(InterfaceMode.restored("unknown") == .classic)
        precondition(InterfaceMode.restored("playBridge") == .playBridge)
        let networkJSON = #"{"supported":true,"available":true,"enabled":true,"applied":true,"message":"ready","interface":"bridge100","ip":"192.168.2.1","cidr":"192.168.2.0/24","uplink":"en0"}"#
        let network = try JSONDecoder().decode(HotspotStatus.self, from: Data(networkJSON.utf8))
        func usage(_ device: String, _ ingress: String, _ date: String) throws -> DailyUsage {
            let json = "{\"date\":\"\(date)\",\"device\":\"\(device)\",\"ingress\":\"\(ingress)\",\"up\":10,\"down\":20,\"connections\":1,\"last_seen\":0}"
            return try JSONDecoder().decode(DailyUsage.self, from: Data(json.utf8))
        }
        let rows = try [usage("192.168.2.4", "gateway", "2026-09-28"),
                        usage("192.168.2.4", "http-proxy", "2026-09-28"),
                        usage("192.168.1.4", "gateway", "2026-09-28"),
                        usage("192.168.2.1", "gateway", "2026-09-28"),
                        usage("192.168.2.4", "gateway", "2026-09-27")]
        precondition(hotspotUsage(rows, network: network, date: "2026-09-28").count == 1)
        precondition(hotspotUsage(rows, network: network).count == 2)
        var sampler = HotspotTrafficSampler()
        let start = Date(timeIntervalSince1970: 1000)
        sampler.record(key: "a", at: start, up: 100, down: 500, uptime: 1)
        precondition(sampler.points.isEmpty)
        sampler.record(key: "a", at: start.addingTimeInterval(4), up: 300, down: 900, uptime: 5)
        precondition(sampler.points.last?.up == 50 && sampler.points.last?.down == 100)
        sampler.record(key: "a", at: start.addingTimeInterval(7), up: 0, down: 0, uptime: 8)
        precondition(sampler.points.isEmpty) // Counters reset.
        sampler.record(key: "a", at: start.addingTimeInterval(10), up: 30, down: 60, uptime: 11)
        precondition(sampler.points.count == 1)
        sampler.record(key: "b", at: start.addingTimeInterval(13), up: 90, down: 120, uptime: 14)
        precondition(sampler.points.isEmpty) // Day or subnet changed.
        sampler.record(key: "b", at: start.addingTimeInterval(16), up: 120, down: 180, uptime: 1)
        precondition(sampler.points.isEmpty) // Core restarted.
        sampler.record(key: "b", at: start.addingTimeInterval(36), up: 150, down: 240, uptime: 21)
        precondition(sampler.points.isEmpty) // Stale sample.
        sampler.reset()
        precondition(sampler.points.isEmpty)
    }

    func testThroughputWindows() {
        let end = Date(timeIntervalSince1970: 1000)
        let recent = [HotspotRatePoint(at: end.addingTimeInterval(-10), up: 1, down: 2),
                      HotspotRatePoint(at: end, up: 3, down: 4)]
        let minute = HotspotChartWindow(points: recent, seconds: 60, now: end)
        let five = HotspotChartWindow(points: recent, seconds: 300, now: end)
        precondition(minute.domain.upperBound.timeIntervalSince(minute.domain.lowerBound) == 60)
        precondition(five.domain.upperBound.timeIntervalSince(five.domain.lowerBound) == 300)
        precondition(minute.points.count == 2 && five.points.count == 2)
        let history = [HotspotRatePoint(at: end.addingTimeInterval(-200), up: 5, down: 6)] + recent
        precondition(HotspotChartWindow(points: history, seconds: 60, now: end).points.count == 2)
        precondition(HotspotChartWindow(points: history, seconds: 300, now: end).points.count == 3)
        let empty = HotspotChartWindow(points: [], seconds: 300, now: end)
        precondition(empty.points.isEmpty && empty.domain.upperBound == end)
    }

}
