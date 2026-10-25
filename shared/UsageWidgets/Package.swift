// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "GtmuxUsageWidgets",
    platforms: [.iOS(.v16), .macOS(.v14)],
    products: [.library(name: "GtmuxUsageWidgets", targets: ["GtmuxUsageWidgets"])],
    targets: [
        .target(name: "GtmuxUsageWidgets", resources: [.process("Resources")]),
        .target(name: "GtmuxUsageWidgetHost"),
        .testTarget(name: "GtmuxUsageWidgetsTests", dependencies: ["GtmuxUsageWidgets", "GtmuxUsageWidgetHost"]),
    ]
)
