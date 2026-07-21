// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "LANProxyGatewayApp",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "LANProxyGatewayApp", targets: ["LANProxyGatewayApp"])
    ],
    targets: [
        .executableTarget(
            name: "LANProxyGatewayApp",
            path: "Sources/LANProxyGatewayApp"
        )
    ]
)
