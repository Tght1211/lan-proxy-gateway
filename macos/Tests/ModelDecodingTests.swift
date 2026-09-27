import Foundation

@main struct ModelDecodingTests {
    static func main() throws { try ModelDecodingTests().testUsageCompatibility(); try ModelDecodingTests().testIngressCompatibility(); try ModelDecodingTests().testHotspotCompatibility(); try ModelDecodingTests().testHotspotControlStates(); print("Model decoding tests passed") }
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

}
