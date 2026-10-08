import Foundation

struct AppUpdateRelease: Decodable {
    struct Asset: Decodable {
        let name: String
        let browser_download_url: URL
        let size: Int64
    }
    let tag_name: String
    let assets: [Asset]
    let draft: Bool
    let prerelease: Bool

    static let repository = "Tght1211/lan-proxy-gateway"
    static let architecture: String = {
        #if arch(arm64)
        return "arm64"
        #else
        return "x86_64"
        #endif
    }()

    var version: String { String(tag_name.hasPrefix("v") ? tag_name.dropFirst() : tag_name[...]) }

    func isNewer(than current: String?) -> Bool {
        guard !version.contains("-"), let target = Self.versionParts(version), !draft, !prerelease else { return false }
        guard let current, let installed = Self.versionParts(current) else { return true }
        return installed.lexicographicallyPrecedes(target)
    }

    private static func versionParts(_ version: String) -> [Int]? {
        let normalized = version.hasPrefix("v") ? String(version.dropFirst()) : version
        // A local preview of a release can still update to that final release.
        guard let base = normalized.split(separator: "-", maxSplits: 1).first else { return nil }
        let pieces = base.split(separator: ".", omittingEmptySubsequences: false)
        let parts = pieces.compactMap { Int($0) }
        guard pieces.count == 3, parts.count == 3, parts.allSatisfy({ $0 >= 0 }) else { return nil }
        return parts + [normalized.contains("-") ? 0 : 1]
    }

    func installationAssets(architecture: String = Self.architecture) throws -> (package: Asset, checksums: Asset) {
        let names = ["LANProxyGateway-\(tag_name)-macos-universal.dmg", "LANProxyGateway-\(tag_name)-macos-\(architecture).dmg"]
        guard let package = names.compactMap({ name in assets.first { $0.name == name } }).first,
              let checksums = assets.first(where: { $0.name == "SHA256SUMS" }), package.size > 0 else {
            throw AppUpdateError.message("此版本缺少适合当前 Mac 的更新包或校验文件。")
        }
        for asset in [package, checksums] {
            guard asset.browser_download_url.scheme == "https",
                  asset.browser_download_url.host == "github.com",
                  asset.browser_download_url.path == "/\(Self.repository)/releases/download/\(tag_name)/\(asset.name)" else {
                throw AppUpdateError.message("更新包地址无效。")
            }
        }
        return (package, checksums)
    }

    static func checksum(for name: String, in manifest: String) throws -> String {
        let matches = manifest.split(whereSeparator: \.isNewline).compactMap { line -> String? in
            let fields = line.split(maxSplits: 1, whereSeparator: \.isWhitespace)
            guard fields.count == 2 else { return nil }
            let filename = fields[1].trimmingCharacters(in: .whitespaces)
            guard filename == name || filename == "*" + name else { return nil }
            return String(fields[0]).lowercased()
        }
        guard matches.count == 1, let hash = matches.first, hash.count == 64,
              hash.allSatisfy({ "0123456789abcdef".contains($0) }) else {
            throw AppUpdateError.message("更新包的 SHA-256 校验信息缺失或无效。")
        }
        return hash
    }
}

enum AppUpdateError: LocalizedError {
    case message(String)
    var errorDescription: String? {
        switch self { case .message(let text): return text }
    }
}
