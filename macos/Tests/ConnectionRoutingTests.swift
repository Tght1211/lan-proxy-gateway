import Foundation

@main struct ConnectionRoutingTests {
    static func main() async throws {
        testHostValidation()
        try testRuleUpdates()
        try await testSaveAndConcurrentChanges()
        try await testFailures()
        print("Connection routing shortcut tests passed")
    }

    static func testHostValidation() {
        precondition(ConnectionRoutingShortcut(host: " API.P01.LP1.Tabiji.SRV.Nintendo.NET.\n")?.host == "api.p01.lp1.tabiji.srv.nintendo.net")
        precondition(ConnectionRoutingShortcut(host: "xn--fiqs8s.example") != nil)
        precondition(ConnectionRoutingShortcut(host: "localhost") != nil)
        for host in ["", "--", "192.168.2.2", "198.18.0.1", "2001:db8::1", "[::1]", "example.com:443",
                     "https://example.com", "a..example.com", "-a.example", "a-.example", "a b.example",
                     "*.example.com", "example.com/path", "a\n.example", "例子.example",
                     String(repeating: "a", count: 64) + ".example", String(repeating: "a.", count: 127) + "example"] {
            precondition(ConnectionRoutingShortcut(host: host) == nil, "invalid domain accepted: \(host)")
        }
    }

    static func testRuleUpdates() throws {
        let shortcut = ConnectionRoutingShortcut(host: "api.p01.lp1.tabiji.srv.nintendo.net.")!
        let device = RoutingRule(type: "src-ip", value: "192.168.2.2", action: "proxy", group: "设备")
        let suffix = RoutingRule(type: "domain-suffix", value: "nintendo.net", action: "proxy", group: "手动")
        let learned = RoutingRule(type: "domain", value: shortcut.host, action: "proxy", group: "自动学习 · Nintendo", learned: true)
        let unrelated = RoutingRule(type: "domain", value: "other.example", action: "direct", group: "保留", learned: true)
        let original = [device, suffix, learned, unrelated]
        let updated = shortcut.updating(original, action: .direct)
        precondition(updated.count == original.count)
        precondition(updated.first?.id == learned.id)
        precondition(updated.first?.type == "domain" && updated.first?.value == shortcut.host)
        precondition(updated.first?.action == "direct" && updated.first?.learned == false)
        precondition(updated.first?.group == "流量快捷规则")
        precondition(Array(updated.dropFirst()) == [device, suffix, unrelated])
        precondition(shortcut.action(in: original) == nil)
        precondition(shortcut.action(in: updated) == .direct)
        let repeated = shortcut.updating(updated, action: .direct)
        precondition(repeated == updated)
        let proxied = shortcut.updating(updated, action: .proxy)
        precondition(proxied.count == updated.count && shortcut.action(in: proxied) == .proxy)
        precondition(Array(proxied.dropFirst()) == Array(updated.dropFirst()))

        var duplicate = learned
        duplicate.value = shortcut.host.uppercased() + "."
        let deduplicated = shortcut.updating(original + [duplicate], action: .direct)
        precondition(deduplicated == updated)
        let manual = RoutingRule(type: "domain", value: shortcut.host, action: "proxy", group: "游戏专用")
        precondition(shortcut.updating([manual], action: .direct).first?.group == "游戏专用")
        let added = shortcut.updating([device, suffix, unrelated], action: .direct)
        precondition(added.count == 4 && Array(added.dropFirst()) == [device, suffix, unrelated])
        let decoded = try JSONDecoder().decode([RoutingRule].self, from: JSONEncoder().encode(updated))
        precondition(ConnectionRoutingShortcut.sameRules(updated, decoded))
        precondition(!ConnectionRoutingShortcut.sameRules(updated, Array(updated.reversed())))
        precondition(!ConnectionRoutingShortcut.sameRules(updated, proxied))
    }

    static func testSaveAndConcurrentChanges() async throws {
        let shortcut = ConnectionRoutingShortcut(host: "sports.example")!
        let initial = [RoutingRule(type: "domain", value: "keep.example", action: "proxy", group: "保留")]
        let learned = RoutingRule(type: "domain", value: "new.example", action: "proxy", group: "自动学习", learned: true)
        var stored = initial
        var reads = 0
        var writes = 0
        try await shortcut.save(action: .direct, readRules: {
            reads += 1
            if reads == 2 { stored.append(learned) }
            return try JSONDecoder().decode([RoutingRule].self, from: JSONEncoder().encode(stored))
        }, writeRules: { rules in
            writes += 1
            stored = rules
        })
        precondition(reads == 5 && writes == 1)
        precondition(stored.count == 3 && shortcut.action(in: stored) == .direct)
        precondition(ConnectionRoutingShortcut.sameRules(Array(stored.dropFirst()), initial + [learned]))
    }

    static func testFailures() async throws {
        let shortcut = ConnectionRoutingShortcut(host: "sports.example")!
        var reads = 0
        var writes = 0
        do {
            try await shortcut.save(action: .direct, readRules: {
                reads += 1
                return [RoutingRule(type: "domain", value: "changing-\(reads).example", action: "proxy")]
            }, writeRules: { _ in writes += 1 })
            preconditionFailure("continuously changing rules must not be overwritten")
        } catch ConnectionRoutingError.rulesChanged {
            precondition(reads == 6 && writes == 0)
        }
        do {
            try await shortcut.save(action: .proxy, readRules: { [] }, writeRules: { _ in })
            preconditionFailure("an unapplied write must not report success")
        } catch ConnectionRoutingError.notApplied {}
        do {
            try await shortcut.save(action: .direct, readRules: { throw ConnectionRoutingError.unavailable },
                                    writeRules: { _ in preconditionFailure("failed read must not write") })
            preconditionFailure("read errors must propagate")
        } catch ConnectionRoutingError.unavailable {}
        do {
            try await shortcut.save(action: .direct, readRules: { [] },
                                    writeRules: { _ in throw ConnectionRoutingError.unavailable })
            preconditionFailure("write errors must propagate")
        } catch ConnectionRoutingError.unavailable {}
    }
}
