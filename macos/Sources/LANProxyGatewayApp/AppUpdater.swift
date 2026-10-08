import AppKit
import CryptoKit
import Foundation

/// Downloads with the user's network settings, never from a root subprocess.
struct AppUpdater {
    let client = GatewayClient()

    static var installationProblem: String? {
        let app = Bundle.main.bundleURL.resolvingSymlinksInPath()
        guard app.pathExtension == "app" else { return "开发运行不支持原地安装，请在已安装的 App 中更新。" }
        guard !app.path.hasPrefix("/Volumes/"), !app.path.contains("/AppTranslocation/") else {
            return "请先将 App 移到“应用程序”文件夹，再使用应用内更新。"
        }
        return nil
    }

    func latestRelease() async throws -> AppUpdateRelease {
        let url = URL(string: "https://api.github.com/repos/\(AppUpdateRelease.repository)/releases/latest")!
        let (data, response) = try await URLSession.shared.data(for: request(url, timeout: 20))
        try validate(response)
        return try JSONDecoder().decode(AppUpdateRelease.self, from: data)
    }

    /// The caller owns the returned directory and removes it after installation or cancellation.
    func prepare(_ release: AppUpdateRelease, directory: URL,
                 progress: @escaping @Sendable (Double?) -> Void,
                 phase: @escaping @Sendable (String) -> Void) async throws -> URL {
        let assets = try release.installationAssets()
        phase("正在下载更新包…")
        let delegate = UpdateDownloadProgress(expectedSize: assets.package.size, progress: progress)
        let session = URLSession(configuration: .default)
        defer { session.invalidateAndCancel() }
        let (download, response) = try await session.download(for: request(assets.package.browser_download_url, timeout: 300), delegate: delegate)
        try validate(response)
        let image = directory.appendingPathComponent("update.dmg")
        try FileManager.default.moveItem(at: download, to: image)
        try Task.checkCancellation()
        phase("正在校验更新包…")
        progress(nil)
        let (manifest, checksumResponse) = try await session.data(for: request(assets.checksums.browser_download_url, timeout: 30))
        try validate(checksumResponse)
        let expected = try AppUpdateRelease.checksum(for: assets.package.name, in: String(decoding: manifest, as: UTF8.self))
        let actual = try await Task.detached { try Self.sha256(image) }.value
        guard actual == expected else { throw AppUpdateError.message("更新包校验失败，请重试下载。") }
        try Task.checkCancellation()
        phase("正在准备安装…")
        return try await unpack(image, release: release, directory: directory)
    }

    func unpack(_ image: URL, release: AppUpdateRelease, directory: URL) async throws -> URL {
        let mount = directory.appendingPathComponent("volume")
        try FileManager.default.createDirectory(at: mount, withIntermediateDirectories: false)
        _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/hdiutil"),
                                        arguments: ["attach", image.path, "-readonly", "-nobrowse", "-noautoopen", "-mountpoint", mount.path])
        let staged = directory.appendingPathComponent("LAN Proxy Gateway.app")
        do {
            try Task.checkCancellation()
            _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/ditto"),
                                            arguments: [mount.appendingPathComponent("LAN Proxy Gateway.app").path, staged.path])
            try await validateBundle(staged, release: release)
        } catch {
            _ = try? await detach(mount)
            throw error
        }
        _ = try await detach(mount)
        try Task.checkCancellation()
        return staged
    }

    func install(_ staged: URL, at destination: URL) async throws {
        let script = Self.installationScript(staged: staged, destination: destination)
        if FileManager.default.isWritableFile(atPath: destination.deletingLastPathComponent().path),
           FileManager.default.isWritableFile(atPath: destination.path) {
            _ = try await client.runProcess(executable: URL(fileURLWithPath: "/bin/sh"), arguments: ["-c", script])
        } else {
            _ = try await client.runPrivilegedShell(script)
        }
    }

    // Stage on the same volume before touching the installed app. Roll back if the swap fails.
    static func installationScript(staged: URL, destination: URL) -> String {
        let token = UUID().uuidString
        let parent = destination.deletingLastPathComponent()
        let incoming = parent.appendingPathComponent(".lan-proxy-update-\(token).app")
        let backup = parent.appendingPathComponent(".lan-proxy-backup-\(token).app")
        return """
        set -eu
        source=\(quote(staged.path))
        target=\(quote(destination.path))
        incoming=\(quote(incoming.path))
        backup=\(quote(backup.path))
        cleanup() { /bin/rm -rf "$incoming"; }
        trap cleanup EXIT
        /usr/bin/ditto "$source" "$incoming"
        /usr/bin/codesign --verify --deep --strict "$incoming"
        /bin/mv "$target" "$backup"
        if /bin/mv "$incoming" "$target"; then
            /bin/rm -rf "$backup" || true
        else
            /bin/mv "$backup" "$target"
            exit 1
        fi
        """
    }

    /// Start the user-owned helper before installing; it only opens the app after an explicit signal.
    /// If install fails, deleting the signal directory makes it exit without relaunching.
    static func launchRelaunchHelper(directory: URL, destination: URL) throws {
        let script = """
        ready=\(quote(directory.appendingPathComponent("relaunch").path))
        workspace=\(quote(directory.path))
        count=0
        while [ -d "$workspace" ] && [ ! -f "$ready" ]; do
            count=$((count + 1))
            [ "$count" -lt 1800 ] || exit 1
            /bin/sleep 1
        done
        [ -f "$ready" ] || exit 0
        count=0
        while /bin/kill -0 \(ProcessInfo.processInfo.processIdentifier) 2>/dev/null; do
            count=$((count + 1))
            [ "$count" -lt 60 ] || exit 1
            /bin/sleep 1
        done
        /usr/bin/open -n \(quote(destination.path))
        /bin/rm -rf "$workspace"
        """
        let helper = Process()
        helper.executableURL = URL(fileURLWithPath: "/bin/sh")
        helper.arguments = ["-c", script]
        helper.standardInput = FileHandle.nullDevice
        helper.standardOutput = FileHandle.nullDevice
        helper.standardError = FileHandle.nullDevice
        try helper.run()
    }

    private func validateBundle(_ app: URL, release: AppUpdateRelease) async throws {
        let plist = app.appendingPathComponent("Contents/Info.plist")
        let info = try PropertyListSerialization.propertyList(from: Data(contentsOf: plist), format: nil) as? [String: Any]
        guard info?["CFBundleIdentifier"] as? String == "com.tght.lan-proxy-gateway",
              info?["CFBundleShortVersionString"] as? String == release.version,
              info?["CFBundleExecutable"] as? String == "LANProxyGatewayApp" else {
            throw AppUpdateError.message("更新包中的应用标识或版本不匹配。")
        }
        let appBinary = app.appendingPathComponent("Contents/MacOS/LANProxyGatewayApp")
        let core = app.appendingPathComponent("Contents/Resources/gateway")
        for binary in [appBinary, core] {
            guard FileManager.default.isExecutableFile(atPath: binary.path) else {
                throw AppUpdateError.message("更新包不完整，缺少可执行的 App 或核心。")
            }
            _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/lipo"),
                                            arguments: [binary.path, "-verify_arch", AppUpdateRelease.architecture])
        }
        _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/codesign"),
                                        arguments: ["--verify", "--deep", "--strict", app.path])
    }

    static func sha256(_ file: URL) throws -> String {
        let handle = try FileHandle(forReadingFrom: file)
        defer { try? handle.close() }
        var hash = SHA256()
        while let chunk = try handle.read(upToCount: 1_048_576), !chunk.isEmpty { hash.update(data: chunk) }
        return hash.finalize().map { String(format: "%02x", $0) }.joined()
    }

    private func detach(_ mount: URL) async throws -> CommandResult {
        do {
            return try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/hdiutil"), arguments: ["detach", mount.path])
        } catch {
            return try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/hdiutil"), arguments: ["detach", "-force", mount.path])
        }
    }

    private func request(_ url: URL, timeout: TimeInterval) -> URLRequest {
        var request = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: timeout)
        request.setValue("lan-proxy-gateway-app", forHTTPHeaderField: "User-Agent")
        return request
    }

    private func validate(_ response: URLResponse) throws {
        guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
            let code = (response as? HTTPURLResponse)?.statusCode ?? 0
            throw AppUpdateError.message("更新服务器返回错误（HTTP \(code)），请稍后重试。")
        }
    }

    private static func quote(_ value: String) -> String {
        "'" + value.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }
}

private final class UpdateDownloadProgress: NSObject, URLSessionDownloadDelegate, @unchecked Sendable {
    let expectedSize: Int64
    let progress: @Sendable (Double?) -> Void
    init(expectedSize: Int64, progress: @escaping @Sendable (Double?) -> Void) {
        self.expectedSize = expectedSize
        self.progress = progress
    }
    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask,
                    didWriteData bytesWritten: Int64, totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64) {
        let total = totalBytesExpectedToWrite > 0 ? totalBytesExpectedToWrite : expectedSize
        progress(total > 0 ? min(1, Double(totalBytesWritten) / Double(total)) : nil)
    }
    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {}
}
