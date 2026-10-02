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

        func topologyDevice(_ address: String) throws -> UsageAggregate {
            try JSONDecoder().decode(UsageAggregate.self, from: Data("{\"name\":\"\(address)\",\"up\":10,\"down\":20,\"connections\":1,\"last_seen\":0}".utf8))
        }
        let topologyDevices = try [topologyDevice("192.168.2.10"), topologyDevice("192.168.2.5"), topologyDevice("192.168.2.4")]
        let topologyOrder = NetworkTopologyLayout.orderedDevices(topologyDevices, inactive: ["192.168.2.4"])
        precondition(topologyOrder.map(\.name) == ["192.168.2.5", "192.168.2.10", "192.168.2.4"], "active devices use stable numeric IP ordering; inactive devices remain last")
        precondition(NetworkTopologyLayout.orderedDevices(Array(topologyDevices.reversed()), inactive: ["192.168.2.4"]).map(\.name) == topologyOrder.map(\.name), "traffic polling cannot reorder topology nodes")
        precondition(NetworkTopologyLayout.orderedDevices([], inactive: []).isEmpty)
        let fiveDevices = try (1...5).map { try topologyDevice("192.168.2.\($0)") }
        precondition(NetworkTopologyLayout.orderedDevices(fiveDevices, inactive: []).count == 5, "the topology must display every visible device")
        let manyDevices = try (1...20).map { try topologyDevice("192.168.2.\($0)") }
        precondition(NetworkTopologyLayout.orderedDevices(manyDevices, inactive: []).count == 20)
        for count in [0, 1, 3, 4, 5, 20] {
            let graphHeight = NetworkTopologyLayout.minimumHeight(deviceCount: count)
            if count > 1 {
                let first = NetworkTopologyLayout.verticalPosition(row: 0, height: graphHeight)
                let next = NetworkTopologyLayout.verticalPosition(row: 3 / Double(count - 1), height: graphHeight)
                precondition(next - first >= NetworkTopologyLayout.deviceSpacing - 0.001, "device labels cannot overlap")
            }
            let last = NetworkTopologyLayout.verticalPosition(row: 3, height: graphHeight)
            precondition(graphHeight - last == NetworkTopologyLayout.bottomInset)
        }
        precondition(NetworkTopologyLayout.minimumHeight(deviceCount: 20) > NetworkTopologyLayout.minimumHeight(deviceCount: 5))
        precondition(NetworkOverviewSizing.exitColumnCount(width: 1200) == 3)
        precondition(NetworkOverviewSizing.exitColumnCount(width: 900) == 3)
        precondition(NetworkOverviewSizing.exitColumnCount(width: 899) == 2)
        precondition(NetworkOverviewSizing.exitColumnCount(width: 560) == 2)
        precondition(NetworkOverviewSizing.exitColumnCount(width: 559) == 1)
        precondition(NetworkOverviewSizing.exitColumnCount(width: 0) == 1)
        precondition(!AppSection.allCases.map(\.rawValue).contains("网络出口"), "exit cards live on the scrollable overview, not a separate navigation page")
        var pageCache = AppSectionCache()
        precondition(pageCache.sections(including: nil) == [.overview], "only the first requested page is mounted")
        pageCache.visit(nil)
        pageCache.visit(.connections)
        pageCache.visit(.connections)
        precondition(pageCache.visited.count == 2, "revisiting a page cannot allocate a duplicate")
        precondition(pageCache.sections(including: .rules) == [.overview, .rules, .connections], "cached page identities keep a stable order")
        for section in AppSection.allCases { pageCache.visit(section) }
        precondition(pageCache.sections(including: .overview) == AppSection.allCases)
        precondition(pageCache.visited.count == AppSection.allCases.count, "navigation cache is bounded by real sections")

        let listRecords = try JSONDecoder().decode([ConnectionInfo].self, from: Data("""
        [
          {"id":1,"src_ip":"device-a","dst_host":"proxy.example","service":"Web","started_at":100000,"down":10,"via_proxy":true},
          {"id":2,"src_ip":"device-a","dst_host":"waiting.example","service":"Web","started_at":100000,"up":10},
          {"id":3,"src_ip":"device-b","dst_host":"direct.example","service":"Web","started_at":99999,"ended_at":100000,"down":20,"fallback":true},
          {"id":4,"src_ip":"device-b","service":"Web","started_at":99999,"ended_at":100000},
          {"id":5,"src_ip":"device-b","service":"Web","started_at":100000,"status":"dial_failed","via_proxy":true},
          {"id":6,"src_ip":"device-a","started_at":100000,"status":"rejected","rejected":true,"down":20,"service":"未解析域名"}
        ]
        """.utf8))
        let list = ConnectionListProjection(records: listRecords)
        precondition(list.total == 6 && list.connections.count == 6 && list.responseRate == "33%")
        for outcome in ["活跃", "等待响应", "成功", "无数据", "失败", "拒绝", "出口回退"] {
            precondition(list.counts[outcome] == 1, "outcome and fallback counts are accumulated once")
        }
        let successes = ConnectionListProjection(records: listRecords, filter: .init(outcome: "成功"))
        precondition(successes.connections.map(\.id) == [3] && successes.total == 6 && successes.counts == list.counts, "outcome selection cannot change chip counts")
        let proxyList = ConnectionListProjection(records: listRecords, filter: .init(route: "代理"))
        precondition(proxyList.connections.map(\.id) == [1, 5] && proxyList.responseRate == "50%")
        let directList = ConnectionListProjection(records: listRecords, filter: .init(outcome: "出口回退", device: "device-b", route: "直连"))
        precondition(directList.connections.map(\.id) == [3] && directList.total == 2)
        let labelSearch = ConnectionListProjection(records: listRecords, filter: .init(search: "手机"), labels: ["device-a": "我的手机"])
        precondition(labelSearch.connections.map(\.id) == [1, 2, 6])
        precondition(labelSearch.devices == ["device-a", "device-b"], "device choices must not disappear when searching")
        let domainSearch = ConnectionListProjection(records: listRecords, filter: .init(search: "DIRECT.EXAMPLE"))
        precondition(domainSearch.connections.map(\.id) == [3])
        let unresolved = ConnectionListProjection(records: listRecords, filter: .init(unresolvedOnly: true))
        precondition(unresolved.connections.map(\.id) == [6] && unresolved.responseRate == "0%")
        precondition(ConnectionListProjection(records: []).responseRate == "--")

        var quarterCalendar = Calendar(identifier: .gregorian)
        quarterCalendar.timeZone = TimeZone(identifier: "Asia/Shanghai")!
        let quarterNow = ISO8601DateFormatter().date(from: "2026-10-01T16:30:00Z")!
        func usage(_ date: String, exit: String?, bytes: Int64) -> DailyUsage {
            DailyUsage(egress: exit, proxyEndpoint: nil, destination: nil, service: nil, date: date,
                       device: "device", ingress: "gateway", up: bytes, down: 0, connections: 1, lastSeen: quarterNow)
        }
        let quarterRows = [usage("2026-09-30", exit: "proxy", bytes: 900),
                           usage("2026-10-01", exit: "proxy", bytes: 120),
                           usage("2026-10-02", exit: "direct", bytes: 80),
                           usage("2026-10-02", exit: nil, bytes: 20),
                           usage("2026-10-03", exit: "direct", bytes: 700)]
        let trafficSummary = NetworkTrafficSummary(rows: quarterRows, at: quarterNow, calendar: quarterCalendar)
        precondition(trafficSummary.hasData && trafficSummary.proxy == 120 && trafficSummary.direct == 80)
        precondition(trafficSummary.total == 220 && trafficSummary.unclassified == 20, "legacy traffic cannot be silently assigned to direct or proxy")
        precondition(!NetworkTrafficSummary(rows: nil).hasData)
        precondition(NetworkTrafficSummary(rows: []).hasData && NetworkTrafficSummary(rows: []).total == 0)
        let dayUsage = NetworkExitUsageSummary(rows: quarterRows, date: "2026-10-02")
        precondition(dayUsage.hasData && dayUsage.usage(for: "direct").total == 80)
        precondition(dayUsage.usage(for: "direct").connections == 1 && dayUsage.usage(for: "proxy").total == 0)
        precondition(dayUsage.usage(for: "unknown").total == 0, "unclassified records cannot be assigned to an exit")
        precondition(!NetworkExitUsageSummary(rows: nil, date: "2026-10-02").hasData)
        let manyRows = Array(repeating: quarterRows, count: 2000).flatMap { $0 }
        let largeUsage = NetworkExitUsageSummary(rows: manyRows, date: "2026-10-02")
        precondition(largeUsage.usage(for: "direct").total == 160_000 && largeUsage.usage(for: "direct").connections == 2000)

        func healthStats(probes: String = "[]", connections: String = "[]", exit: String = "proxy") throws -> RuntimeStats {
            try JSONDecoder().decode(RuntimeStats.self, from: Data("""
            {"egress":"\(exit)","uptime_sec":1,"relay":{"recent":\(connections)},"health":{"history":\(probes)}}
            """.utf8))
        }
        let freshProbe = "[{\"at\":100000,\"latency_ms\":12,\"ok\":true}]"
        let successfulDirect = "[{\"id\":1,\"started_at\":99999,\"ended_at\":100000,\"down\":10}]"
        let proxyOnly = NetworkGlobalHealth(stats: try healthStats(probes: freshProbe), proxyConfigured: true, isRunning: true, at: now)
        precondition(proxyOnly.healthyCount == 1 && proxyOnly.conditions == [.healthy, .unknown])
        precondition(proxyOnly.title == "检测未齐", "one healthy proxy cannot stand in for global health")
        let allHealthyStats = try healthStats(probes: freshProbe, connections: successfulDirect)
        let allHealthy = NetworkGlobalHealth(stats: allHealthyStats, proxyConfigured: true, isRunning: true, at: now)
        precondition(allHealthy.title == "全部正常" && allHealthy.healthyCount == 2)
        let directFailure = "[{\"id\":2,\"started_at\":99999,\"ended_at\":100000,\"status\":\"dial_failed\"}]"
        let partial = NetworkGlobalHealth(stats: try healthStats(probes: freshProbe, connections: directFailure), proxyConfigured: true, isRunning: true, at: now)
        precondition(partial.title == "部分异常" && partial.conditions == [.healthy, .degraded])
        let stopped = NetworkGlobalHealth(stats: allHealthyStats, proxyConfigured: true, isRunning: false, at: now)
        precondition(stopped.title == "网关已停止" && stopped.healthyCount == 0)
        let staleStats = try healthStats(probes: "[{\"at\":99960,\"latency_ms\":12,\"ok\":true}]")
        precondition(NetworkGlobalHealth(stats: staleStats, proxyConfigured: true, isRunning: true, at: now).healthyCount == 0)
        let futureStats = try healthStats(probes: "[{\"at\":100010,\"latency_ms\":12,\"ok\":true}]")
        precondition(NetworkGlobalHealth(stats: futureStats, proxyConfigured: true, isRunning: true, at: now).healthyCount == 0)
        let rejectedStats = try healthStats(connections: "[{\"id\":3,\"started_at\":100000,\"rejected\":true}]")
        precondition(NetworkGlobalHealth(stats: rejectedStats, proxyConfigured: false, isRunning: true, at: now).conditions == [.unknown])
        let directOnly = NetworkGlobalHealth(stats: try healthStats(connections: successfulDirect), proxyConfigured: false, isRunning: true, at: now)
        precondition(directOnly.conditions == [.healthy] && directOnly.title == "全部正常")
        let oldStats = try healthStats(connections: "[{\"id\":4,\"started_at\":99000,\"ended_at\":99000,\"down\":10}]")
        precondition(NetworkGlobalHealth(stats: oldStats, proxyConfigured: false, isRunning: true, at: now).healthyCount == 0)
        let futureFailure = try healthStats(connections: "[{\"id\":5,\"started_at\":99999,\"last_traffic_at\":99999,\"ended_at\":100010,\"status\":\"dial_failed\"}]")
        precondition(NetworkGlobalHealth(stats: futureFailure, proxyConfigured: false, isRunning: true, at: now, includeProbe: false).conditions == [.unknown], "history cannot backdate a later failure")
        let motionStart = NetworkFloatingTextMotion.frame(at: 0)
        let motionMiddle = NetworkFloatingTextMotion.frame(at: 0.5)
        let motionEnd = NetworkFloatingTextMotion.frame(at: 1)
        precondition(motionStart.lift < motionMiddle.lift && motionMiddle.lift < motionEnd.lift, "connection labels float upward")
        precondition(motionStart.opacity == 0 && motionMiddle.opacity == 1 && motionEnd.opacity == 0, "floating labels fade in and out")
        precondition(NetworkFloatingTextMotion.frame(at: -1).lift == 0)
        precondition(NetworkFloatingTextMotion.frame(at: 2).opacity == 0)
        let still = NetworkFloatingTextMotion.frame(at: 0.5, reducedMotion: true)
        precondition(still.lift == 0 && still.opacity == 1, "reduced motion keeps results readable without floating")

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
        precondition(pendingFeed.ingest([waiting], uptime: 2).count == 1, "outgoing data replays as waiting, not success")
        precondition(pendingFeed.ingest([waiting], uptime: 2).isEmpty, "waiting events are deduplicated")
        let answered = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":99,"started_at":0,"up":20,"down":80,"response_ms":450}"#.utf8))
        precondition(pendingFeed.ingest([answered], uptime: 3).count == 1)
        precondition(pendingFeed.ingest([answered], uptime: 4).isEmpty)
        let closedAnswer = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":99,"started_at":0,"up":20,"down":80,"response_ms":450,"ended_at":5}"#.utf8))
        precondition(pendingFeed.ingest([closedAnswer], uptime: 5).isEmpty, "closing a successful connection must not replay it")
        func result(_ fields: String) throws -> NetworkRequestPresentation {
            let connection = try JSONDecoder().decode(ConnectionInfo.self, from: Data("{\"id\":100,\"started_at\":0,\"dst_host\":\"example.com\",\(fields)}".utf8))
            return NetworkRequestPresentation(connection: connection)
        }
        let fast = try result("\"down\":20,\"response_ms\":450")
        let boundary = try result("\"down\":20,\"response_ms\":500")
        let slow = try result("\"down\":20,\"response_ms\":501")
        let closed = try result("\"down\":20,\"ended_at\":5,\"response_ms\":450")
        let missing = try result("\"down\":20")
        let rejected = try result("\"rejected\":true,\"response_ms\":450")
        let failed = try result("\"status\":\"dial_failed\",\"failure\":\"超时\",\"response_ms\":450")
        let noResponse = try result("\"up\":20,\"ended_at\":5")
        let empty = try result("\"ended_at\":5")
        let zero = try result("\"down\":20,\"response_ms\":0")
        let negative = try result("\"down\":20,\"response_ms\":-1")
        precondition(fast.label == "首响应 450 ms" && fast.tone == .success)
        precondition(zero.label == "首响应 0 ms" && zero.tone == .success)
        precondition(negative.label == "已响应 · 耗时未记录")
        precondition(boundary.tone == .success && slow.tone == .delayed)
        precondition(closed.label == fast.label && closed.eventPhase == fast.eventPhase, "closing an answered connection must not replay success again")
        precondition(missing.label == "已响应 · 耗时未记录", "older cores must not invent durations")
        precondition(rejected.label == "拒绝" && rejected.tone == .failure)
        precondition(failed.label == "失败·超时" && failed.tone == .failure, "a failure must not display stale timing as success")
        precondition(noResponse.label == "失败·未收到响应" && noResponse.tone == .failure)
        precondition(empty.label == "无数据" && empty.tone == .neutral)
        let pending = NetworkRequestPresentation(connection: waiting)
        let uppercase = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":100,"started_at":0,"down":20,"response_ms":450,"dst_host":"EXAMPLE.COM."}"#.utf8))
        precondition(pending.label == "等待响应" && pending.tone == .neutral)
        for palette in NetworkFlowPalette.allCases {
            precondition(palette.hue(for: slow) == .yellow)
            precondition(palette.hue(for: failed) == .red)
            precondition(palette.hue(for: rejected) == .red)
            precondition(palette.hue(for: pending) == .muted)
            precondition(palette.hue(for: fast) == palette.hue(for: NetworkRequestPresentation(connection: uppercase)))
        }
        precondition(NetworkFlowPalette.status.hue(for: fast) == .green)
        precondition(NetworkFlowPalette.status.hue(for: missing) == .green)
        precondition(NetworkFlowPalette.allCases.filter { $0 != .status }.allSatisfy { $0.hues.count == 6 && Set($0.hues).count > 1 })
        var resultFeed = NetworkEventFeed()
        precondition(resultFeed.ingest([waiting], uptime: 1).isEmpty)
        precondition(resultFeed.ingest([answered], uptime: 2).count == 1, "an initial waiting connection may later produce a live response")
        var failureFeed = NetworkEventFeed()
        _ = failureFeed.ingest([], uptime: 1)
        precondition(failureFeed.ingest([waiting], uptime: 2).count == 1)
        let failedWaiting = try JSONDecoder().decode(ConnectionInfo.self, from: Data(#"{"id":99,"started_at":0,"up":20,"down":0,"ended_at":5}"#.utf8))
        precondition(failureFeed.ingest([failedWaiting], uptime: 3).count == 1, "terminal failure updates the waiting status")
        precondition(failureFeed.ingest([failedWaiting], uptime: 4).isEmpty)
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
