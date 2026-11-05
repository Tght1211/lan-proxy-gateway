import Foundation

@main struct AppUpdaterTests {
    static func main() async {
        do {
            try testReleaseSelection()
            try testChecksums()
            try await testBundleInstallation()
            print("App updater release, checksum, DMG, installation and rollback tests passed")
        } catch {
            FileHandle.standardError.write(Data("Updater test failed: \(error.localizedDescription)\n".utf8))
            exit(1)
        }
    }

    static func release(_ tag: String = "v4.5.3", names: [String]? = nil) throws -> AppUpdateRelease {
        let names = names ?? ["LANProxyGateway-\(tag)-macos-universal.dmg", "SHA256SUMS"]
        let assets = names.map { ["name": $0, "browser_download_url": "https://github.com/Tght1211/lan-proxy-gateway/releases/download/\(tag)/\($0)", "size": 1024] as [String: Any] }
        let data = try JSONSerialization.data(withJSONObject: ["tag_name": tag, "draft": false, "prerelease": false, "assets": assets])
        return try JSONDecoder().decode(AppUpdateRelease.self, from: data)
    }

    static func require(_ condition: @autoclosure () throws -> Bool) throws {
        let result = try condition()
        precondition(result)
    }

    static func expectFailure(_ action: () throws -> Void) {
        do { try action(); preconditionFailure("Expected rejection") } catch {}
    }

    static func testReleaseSelection() throws {
        let current = try release()
        try require(current.isNewer(than: "4.5.2"))
        try require(current.isNewer(than: "4.5.3-preview"))
        try require(!current.isNewer(than: "4.5.3"))
        try require(!current.isNewer(than: "4.6.0"))
        try require(try release("v4.10.0").isNewer(than: "4.9.9"))
        try require(current.isNewer(than: nil))
        try require(!((try release("")).isNewer(than: "4.5.2")))
        try require(!((try release("v4.5.4-beta")).isNewer(than: "4.5.2")))
        try require(try current.installationAssets(architecture: "arm64").package.name.hasSuffix("universal.dmg"))
        let arm = try release(names: ["LANProxyGateway-v4.5.3-macos-arm64.dmg", "SHA256SUMS"])
        try require(try arm.installationAssets(architecture: "arm64").package.name.hasSuffix("arm64.dmg"))
        expectFailure { _ = try arm.installationAssets(architecture: "x86_64") }
        expectFailure { _ = try release(names: ["LANProxyGateway-v4.5.3-macos-universal.dmg"]).installationAssets() }
        let wrongOrigin = """
        {"tag_name":"v4.5.3","draft":false,"prerelease":false,"assets":[
        {"name":"LANProxyGateway-v4.5.3-macos-universal.dmg","browser_download_url":"https://example.com/package.dmg","size":1024},
        {"name":"SHA256SUMS","browser_download_url":"https://example.com/SHA256SUMS","size":100}]}
        """
        let invalid = try JSONDecoder().decode(AppUpdateRelease.self, from: Data(wrongOrigin.utf8))
        expectFailure { _ = try invalid.installationAssets() }
    }

    static func testChecksums() throws {
        let hash = String(repeating: "a", count: 64)
        try require(try AppUpdateRelease.checksum(for: "app.dmg", in: "\(hash)  app.dmg\n") == hash)
        try require(try AppUpdateRelease.checksum(for: "app.dmg", in: "\(hash.uppercased()) *app.dmg\n") == hash)
        expectFailure { _ = try AppUpdateRelease.checksum(for: "app.dmg", in: "\(hash)  other.dmg") }
        expectFailure { _ = try AppUpdateRelease.checksum(for: "app.dmg", in: "wrong  app.dmg") }
        expectFailure { _ = try AppUpdateRelease.checksum(for: "app.dmg", in: "\(hash)  app.dmg\n\(hash)  app.dmg") }
        let file = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: file) }
        try Data("abc".utf8).write(to: file)
        try require(try AppUpdater.sha256(file) == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
    }

    static func testBundleInstallation() async throws {
        let client = GatewayClient()
        let fm = FileManager.default
        // Quotes, spaces and shell substitutions must remain literal in paths.
        let root = fm.temporaryDirectory.appendingPathComponent("updater test '\(UUID().uuidString) $(false)")
        try fm.createDirectory(at: root, withIntermediateDirectories: false)
        defer { try? fm.removeItem(at: root) }
        let payload = root.appendingPathComponent("payload")
        let app = payload.appendingPathComponent("LAN Proxy Gateway.app")
        let contents = app.appendingPathComponent("Contents")
        try fm.createDirectory(at: contents.appendingPathComponent("MacOS"), withIntermediateDirectories: true)
        try fm.createDirectory(at: contents.appendingPathComponent("Resources"), withIntermediateDirectories: false)
        let binary = contents.appendingPathComponent("MacOS/LANProxyGatewayApp")
        // A small real Mach-O fixture exercises architecture and code-signature validation.
        let source = root.appendingPathComponent("fixture.c")
        try Data("int main(void) { return 0; }".utf8).write(to: source)
        _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/clang"),
                                        arguments: ["-arch", AppUpdateRelease.architecture, source.path, "-o", binary.path])
        try fm.copyItem(at: binary, to: contents.appendingPathComponent("Resources/gateway"))
        let info = ["CFBundleIdentifier": "com.tght.lan-proxy-gateway", "CFBundleExecutable": "LANProxyGatewayApp", "CFBundleShortVersionString": "4.5.3", "CFBundlePackageType": "APPL"]
        try PropertyListSerialization.data(fromPropertyList: info, format: .xml, options: 0).write(to: contents.appendingPathComponent("Info.plist"))
        _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/codesign"), arguments: ["--force", "--deep", "--sign", "-", app.path])
        let image = root.appendingPathComponent("update.dmg")
        _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/hdiutil"), arguments: ["create", "-srcfolder", payload.path, "-format", "UDZO", image.path])
        let work = root.appendingPathComponent("work")
        try fm.createDirectory(at: work, withIntermediateDirectories: false)
        let updater = AppUpdater()
        let staged = try await updater.unpack(image, release: release(), directory: work)
        try require(fm.isExecutableFile(atPath: staged.appendingPathComponent("Contents/Resources/gateway").path))
        let invalidWork = root.appendingPathComponent("invalid")
        try fm.createDirectory(at: invalidWork, withIntermediateDirectories: false)
        do {
            _ = try await updater.unpack(image, release: release("v4.5.4"), directory: invalidWork)
            preconditionFailure("Mismatched version accepted")
        } catch {}
        // Both success and failure must detach the DMG.
        let mounts = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/hdiutil"), arguments: ["info"])
        try require(!mounts.text.contains(work.appendingPathComponent("volume").path))
        try require(!mounts.text.contains(invalidWork.appendingPathComponent("volume").path))

        let installed = root.appendingPathComponent("Installed App.app")
        try fm.createDirectory(at: installed, withIntermediateDirectories: false)
        try Data("old".utf8).write(to: installed.appendingPathComponent("old-marker"))
        // Inject a failure into the final rename to exercise the actual rollback branch.
        let failingSwap = AppUpdater.installationScript(staged: staged, destination: installed)
            .replacingOccurrences(of: "if /bin/mv \"$incoming\" \"$target\"; then", with: "if /usr/bin/false; then")
        do {
            _ = try await client.runProcess(executable: URL(fileURLWithPath: "/bin/sh"), arguments: ["-c", failingSwap])
            preconditionFailure("Swap failure did not propagate")
        } catch {}
        try require(fm.fileExists(atPath: installed.appendingPathComponent("old-marker").path))
        try await updater.install(staged, at: installed)
        try require(fm.isExecutableFile(atPath: installed.appendingPathComponent("Contents/MacOS/LANProxyGatewayApp").path))
        try require(!fm.fileExists(atPath: installed.appendingPathComponent("old-marker").path))
        // A tampered payload must never replace the currently installed bundle.
        try Data("tampered".utf8).write(to: staged.appendingPathComponent("Contents/Resources/gateway"))
        do { try await updater.install(staged, at: installed); preconditionFailure("Tampered bundle installed") } catch {}
        _ = try await client.runProcess(executable: URL(fileURLWithPath: "/usr/bin/codesign"), arguments: ["--verify", "--deep", "--strict", installed.path])
        let leftovers = try fm.contentsOfDirectory(atPath: root.path)
        try require(!leftovers.contains { $0.hasPrefix(".lan-proxy-") })
    }
}
