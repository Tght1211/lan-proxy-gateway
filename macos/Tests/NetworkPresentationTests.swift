import Foundation
@main struct NetworkPresentationTests {
    static func main() throws {
        let now = Date(timeIntervalSinceReferenceDate: 100_000)
        func device(_ total: Int, seen: Int, traffic: Int? = nil) throws -> UsageAggregate {
            let stamp = traffic.map { ",\"last_traffic_at\":\($0)" } ?? ""
            return try JSONDecoder().decode(UsageAggregate.self, from: Data("{\"name\":\"192.168.2.5\",\"up\":\(total),\"down\":0,\"connections\":1,\"last_seen\":\(seen)\(stamp)}".utf8))
        }
        var activity = NetworkDeviceActivity()
        let freshDevice = try device(50, seen: 10, traffic: 100_000)
        activity.ingest([freshDevice], uptime: 1, at: now)
        precondition(activity.state(for: freshDevice, at: now.addingTimeInterval(600)) == .active)
        precondition(activity.state(for: freshDevice, at: now.addingTimeInterval(601)) == .inactive, "idle long-lived connections must fade")
        precondition(activity.state(for: freshDevice, at: now.addingTimeInterval(86_400)) == .inactive)
        precondition(activity.state(for: freshDevice, at: now.addingTimeInterval(86_401)) == .hidden, "day-old devices leave the topology")
        let resumed = try device(100, seen: 10, traffic: 186_402)
        activity.ingest([resumed], uptime: 86_402, at: now.addingTimeInterval(86_402))
        precondition(activity.state(for: resumed, at: now.addingTimeInterval(86_402)) == .active)
        // Older cores lack byte timestamps. Polling/closing must not keep them alive.
        var legacyActivity = NetworkDeviceActivity()
        let legacy = try device(50, seen: 100_000)
        legacyActivity.ingest([legacy], uptime: 1, at: now)
        let closedLegacy = try device(50, seen: 100_700)
        legacyActivity.ingest([closedLegacy], uptime: 701, at: now.addingTimeInterval(700))
        precondition(legacyActivity.state(for: closedLegacy, at: now.addingTimeInterval(700)) == .inactive)
        var expiredLegacy = legacyActivity
        expiredLegacy.ingest([closedLegacy], uptime: 86_402, at: now.addingTimeInterval(86_401))
        precondition(expiredLegacy.state(for: closedLegacy, at: now.addingTimeInterval(86_401)) == .hidden, "an idle close timestamp cannot resurrect expired devices")
        let legacyResumed = try device(51, seen: 100_700)
        legacyActivity.ingest([legacyResumed], uptime: 702, at: now.addingTimeInterval(701))
        precondition(legacyActivity.state(for: legacyResumed, at: now.addingTimeInterval(701)) == .active)
        let silent = try device(0, seen: 100_000)
        activity.ingest([silent], uptime: 1, at: now) // restart clears old counters
        precondition(activity.state(for: silent, at: now) == .hidden, "a socket without bytes is not device activity")

        func evidence(_ domain: String, service: String = "", age: Int = 0, down: Int = 10) throws -> ConnectionInfo {
            try JSONDecoder().decode(ConnectionInfo.self, from: Data("{\"id\":1,\"src_ip\":\"192.168.2.5\",\"started_at\":\(100_000-age),\"dst_host\":\"\(domain)\",\"service\":\"\(service)\",\"down\":\(down)}".utf8))
        }
        let store = try evidence("store.playstation.com", service: "PlayStation")
        let wechat = try evidence("wetype.weixin.qq.com", service: "微信")
        let play = try evidence("play-fe.googleapis.com", service: "Google")
        let mobile = try evidence("mdp-appconf-cn.heytapdownload.com", service: "heytapdownload.com")
        let pc = try evidence("client-update.steamstatic.com", service: "Steam")
        precondition(DeviceIdentification.label(connections: [store], services: [], at: now) == nil, "a gaming website alone cannot identify a console")
        precondition(DeviceIdentification.label(connections: [store,wechat,play,mobile], services: [], at: now) == "手机", "mobile platform evidence must correct console misclassification")
        precondition(DeviceIdentification.label(connections: [wechat], services: [], at: now) == nil, "WeChat also runs on computers")
        precondition(DeviceIdentification.label(connections: [pc,wechat,store], services: [], at: now) == "电脑")
        let stale = try evidence("play-fe.googleapis.com", age: 2000)
        let misleading = try [evidence("play-fe.googleapis.com.evil.test"), evidence("play-fe.googleapis.com", down: 0)]
        precondition(DeviceIdentification.label(connections: [stale], services: [], at: now) == nil, "stale observations cannot identify today's device")
        precondition(DeviceIdentification.label(connections: misleading, services: [], at: now) == nil, "unknown or failed requests are not platform evidence")
        let console = try [evidence("a.playstation.net",service:"PlayStation"),evidence("b.playstation.net",service:"PlayStation"),evidence("c.playstation.net",service:"PlayStation")]
        precondition(DeviceIdentification.label(connections: console, services: [], at: now) == "PlayStation")
        precondition(DeviceIdentification.label(connections: console + [mobile], services: [], at: now) == "手机")
        func connection(_ id: Int, start: Int = 0, ingress: String = "gateway") throws -> ConnectionInfo {
            try JSONDecoder().decode(ConnectionInfo.self, from: Data("{\"id\":\(id),\"started_at\":\(start),\"src_ip\":\"192.168.2.5\",\"dst_host\":\"example.com\",\"ingress\":\"\(ingress)\",\"down\":20}".utf8))
        }
        var pendingFeed = NetworkEventFeed()
        _ = pendingFeed.ingest([], uptime: 1)
        let waiting = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":99,"started_at":0,"up":20,"down":0}"#.utf8))
        precondition(pendingFeed.ingest([waiting], uptime: 2).isEmpty, "wait for actual result before replay")
        let answered = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":99,"started_at":0,"up":20,"down":80,"response_ms":450}"#.utf8))
        precondition(pendingFeed.ingest([answered], uptime: 3).count == 1)
        precondition(pendingFeed.ingest([answered], uptime: 4).isEmpty)
        let initialRule = RoutingRule(type: "domain", value: "a.test", action: "proxy")
        let addedRule = RoutingRule(type: "domain", value: "b.test", action: "proxy")
        let changedRule = RoutingRule(type: "domain", value: "a.test", action: "reject")
        precondition(NetworkRuleRevision.assess(baseline: [initialRule], latest: [initialRule,addedRule], draftChanged: false) == .reload)
        precondition(NetworkRuleRevision.assess(baseline: [initialRule], latest: [initialRule,addedRule], draftChanged: true) == .conflict)
        precondition(NetworkRuleRevision.assess(baseline: [initialRule], latest: [initialRule], draftChanged: true) == .unchanged)
        precondition(NetworkRuleRevision.signature([initialRule]) != NetworkRuleRevision.signature([changedRule]))
        let sameRuleNewID = RoutingRule(type: "domain", value: "a.test", action: "proxy")
        precondition(NetworkRuleRevision.signature([initialRule]) == NetworkRuleRevision.signature([sameRuleNewID]), "decode UUIDs are not configuration revisions")
        var feed = NetworkEventFeed()
        let a = try connection(1)
        precondition(feed.ingest([a], uptime: 100).isEmpty, "initial history is not live traffic")
        let b = try connection(2)
        let restartedA = try connection(1,start:4), restartedB = try connection(2,start:5)
        let http = try connection(3,ingress:"http-proxy"), unknown = try connection(4,ingress:"other")
        precondition(feed.ingest([a,b,b], uptime: 103).count == 1)
        precondition(feed.ingest([a,b], uptime: 106).isEmpty)
        precondition(feed.ingest([restartedA], uptime: 1).isEmpty, "restart clears baseline")
        precondition(feed.ingest([restartedB], uptime: 4).count == 1)
        let network = try JSONDecoder().decode(HotspotStatus.self, from: Data(#"{"supported":true,"available":true,"enabled":true,"applied":true,"message":"","interface":"bridge100","ip":"192.168.2.1","cidr":"192.168.2.0/24","uplink":"en0"}"#.utf8))
        precondition(NetworkIngress.resolve(a, hotspot: network) == .wifi)
        precondition(NetworkIngress.resolve(http, hotspot: network) == .http)
        precondition(NetworkIngress.resolve(unknown, hotspot: network) == .unknown)
        precondition(NetworkIngress.resolve(a, hotspot: nil) == .gateway)
        precondition(NetworkDomainColor.index("EXAMPLE.COM.") == NetworkDomainColor.index("example.com"))
        for n in 0..<1000 { _ = feed.ingest([try connection(n,start:n)], uptime: Int64(n+10)) }
        precondition(feed.retainedCount <= 512)
        print("Network presentation tests passed")
    }
}
