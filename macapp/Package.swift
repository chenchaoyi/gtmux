// swift-tools-version:5.9
import PackageDescription

// GtmuxBar — the native macOS menu-bar app for gtmux. A pure consumer of the
// gtmux CLI (`agents --json` + `focus`); gtmux-core stays the data source.
let package = Package(
    name: "GtmuxBar",
    platforms: [.macOS(.v14)],
    products: [
        .executable(name: "GtmuxBar", targets: ["GtmuxBarLauncher"]),
        .library(name: "GtmuxBarCore", targets: ["GtmuxBar"]),
        .library(name: "MacUsageWidgetSupport", targets: ["MacUsageWidgetSupport"]),
    ],
    dependencies: [.package(path: "../shared/UsageWidgets")],
    targets: [
        .target(name: "MacUsageWidgetSupport", dependencies: [.product(name: "GtmuxUsageWidgets", package: "UsageWidgets")], path: "WidgetShared"),
        .target(name: "GtmuxBar", dependencies: ["MacUsageWidgetSupport", .product(name: "GtmuxUsageWidgets", package: "UsageWidgets")], path: "Sources/GtmuxBar"),
        .executableTarget(name: "GtmuxBarLauncher", dependencies: ["GtmuxBar"], path: "Sources/GtmuxBarLauncher"),
        .testTarget(name: "GtmuxBarTests", dependencies: ["GtmuxBar"], path: "Tests/GtmuxBarTests"),
        .testTarget(name: "MacUsageWidgetSupportTests", dependencies: ["MacUsageWidgetSupport", .product(name: "GtmuxUsageWidgets", package: "UsageWidgets")], path: "Tests/MacUsageWidgetSupportTests"),
    ]
)
