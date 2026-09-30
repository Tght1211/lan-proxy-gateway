import Foundation

/// Traffic provides clues, not hardware identity. Platform-specific evidence
/// beats game-store browsing; conflicting platforms stay unidentified.
enum DeviceIdentification {
    static func label(connections: [ConnectionInfo], services: [UsageAggregate], at now: Date) -> String? {
        let cutoff = now.addingTimeInterval(-1800)
        let recent = connections.filter {
            !$0.rejected && $0.status != "dial_failed" && $0.down > 0 &&
            ($0.lastTrafficAt ?? $0.startedAt) >= cutoff
        }
        let hosts = Set(recent.map { $0.dstHost.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: ".")) })
        let names = Set(services.filter { $0.total > 0 && ($0.lastTrafficAt ?? $0.lastSeen) >= cutoff }.map(\.name))
        let phoneDomains = ["play-fe.googleapis.com", "android.clients.google.com", "heytapmobi.com", "heytapdownload.com", "push.hicloud.com", "push.xiaomi.com"]
        let desktopDomains = ["client-update.steamstatic.com", "windowsupdate.com", "update.microsoft.com", "swscan.apple.com", "swdist.apple.com"]
        let phone = hosts.contains { host in phoneDomains.contains { matches(host, suffix: $0) } } ||
            !names.isDisjoint(with: ["heytapmobi.com", "heytapdownload.com"])
        let desktop = hosts.contains { host in desktopDomains.contains { matches(host, suffix: $0) } }
        if phone && desktop { return nil }
        if phone { return "手机" }
        if desktop { return "电脑" }

        // Messaging/social services run on phones and computers. They contradict
        // a confident console guess but cannot identify a phone on their own.
        let social = ["微信", "抖音", "小红书", "TikTok", "Meta", "Instagram"]
        if !names.isDisjoint(with: social) || recent.contains(where: { social.contains($0.service) }) { return nil }
        guard hosts.count >= 3 else { return nil }
        let ps = hosts.filter { matches($0, suffix: "playstation.net") || matches($0, suffix: "sonyentertainmentnetwork.com") }.count
        let nintendo = hosts.filter { matches($0, suffix: "nintendo.net") || matches($0, suffix: "nintendowifi.net") }.count
        // Require multiple platform endpoints and a dominant recent fingerprint.
        if Double(ps) / Double(hosts.count) >= 0.8 { return "PlayStation" }
        if Double(nintendo) / Double(hosts.count) >= 0.8 { return "Switch" }
        return nil
    }
    private static func matches(_ host: String, suffix: String) -> Bool {
        host == suffix || host.hasSuffix("." + suffix)
    }
}
