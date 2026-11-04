import UIKit
import React
import React_RCTAppDelegate
import ReactAppDependencyProvider
import UserNotifications

@main
class AppDelegate: UIResponder, UIApplicationDelegate, UNUserNotificationCenterDelegate {
  var window: UIWindow?

  var reactNativeDelegate: ReactNativeDelegate?
  var reactNativeFactory: RCTReactNativeFactory?
  var launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  private var headlessResponseStarted = false

  func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
  ) -> Bool {
    let delegate = ReactNativeDelegate()
    let factory = RCTReactNativeFactory(delegate: delegate)
    delegate.dependencyProvider = RCTAppDependencyProvider()

    reactNativeDelegate = delegate
    reactNativeFactory = factory

    // Route notification taps/presentation through the push module → JS.
    UNUserNotificationCenter.current().delegate = self
    UsageWidgetRefresh.shared.register()

    self.launchOptions = launchOptions

    return true
  }

  // MARK: hardware keyboard (iPad) — the keymap JS registered, as main-menu commands.

  func application(_ app: UIApplication, open url: URL,
                   options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
    RCTLinkingManager.application(app, open: url, options: options)
  }
  // The responder chain ends here, so a command from any first responder reaches it.

  override func buildMenu(with builder: UIMenuBuilder) {
    super.buildMenu(with: builder)
    guard builder.system == .main else { return }
    KeyCommands.install(into: builder)
  }

  override var keyCommands: [UIKeyCommand]? { KeyCommands.responderCommands() }

  @objc func gtmuxKeyCommand(_ sender: UIKeyCommand) {
    guard let id = sender.propertyList as? String else { return }
    KeyCommands.shared?.emit(id)
  }

  // MARK: APNs registration → JS (react-native-community/push-notification-ios)

  func application(
    _ application: UIApplication,
    didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
  ) {
    RNCPushNotificationIOS.didRegisterForRemoteNotifications(withDeviceToken: deviceToken)
  }

  // (didFailToRegister… is intentionally omitted: it only reports registration
  // failures to JS, which the MVP doesn't act on, and its ObjC→Swift name import
  // is unstable. The token/notification/tap paths below are what matter.)

  func application(
    _ application: UIApplication,
    didReceiveRemoteNotification userInfo: [AnyHashable: Any],
    fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void
  ) {
    RNCPushNotificationIOS.didReceiveRemoteNotification(userInfo, fetchCompletionHandler: completionHandler)
  }

  // MARK: foreground presentation + tap (UNUserNotificationCenterDelegate)

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    willPresent notification: UNNotification,
    withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
  ) {
    // We surface foreground alerts as an in-app banner via SSE, so keep the
    // system banner quiet here (badge/sound only).
    completionHandler([.badge, .sound])
  }

  func userNotificationCenter(
    _ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void
  ) {
    // Delivers the tapped notification to JS (handled by the push module's
    // 'notification' listener), which deep-links to the agent's Detail.
    let payload = bridgeNotificationResponse(response)
    // A background action can launch the process without connecting a UI scene.
    // Start the existing RN host with the action as its launch notification so
    // getInitialNotification can deliver it after JS listeners are installed.
    if window == nil {
      var options = launchOptions ?? [:]
      options[.remoteNotification] = payload
      launchOptions = options
      if UIApplication.shared.applicationState == .background,
         !headlessResponseStarted, let factory = reactNativeFactory {
        headlessResponseStarted = true
        factory.startReactNative(withModuleName: "GtmuxMobile", in: nil, launchOptions: options)
      }
    }
    RNCPushNotificationIOS.didReceiveRemoteNotification(payload)
    completionHandler()
  }
}

class ReactNativeDelegate: RCTDefaultReactNativeFactoryDelegate {
  override func sourceURL(for bridge: RCTBridge) -> URL? {
    self.bundleURL()
  }

  override func bundleURL() -> URL? {
#if DEBUG
    RCTBundleURLProvider.sharedSettings().jsBundleURL(forBundleRoot: "index")
#else
    Bundle.main.url(forResource: "main", withExtension: "jsbundle")
#endif
  }
}

// A window belongs to its scene: the scene can change size while the app is running.
// Keep the factory and delegate on AppDelegate for the lifetime of the React bridge.
class SceneDelegate: UIResponder, UIWindowSceneDelegate {
  var window: UIWindow?

  func scene(_ scene: UIScene, willConnectTo session: UISceneSession,
             options connectionOptions: UIScene.ConnectionOptions) {
    guard let scene = scene as? UIWindowScene,
          let app = UIApplication.shared.delegate as? AppDelegate,
          let factory = app.reactNativeFactory else { return }
    var options = app.launchOptions ?? [:]
    // React Native and the push module read their cold-start data from launchOptions.
    if let url = connectionOptions.urlContexts.first?.url { options[.url] = url }
    if let activity = connectionOptions.userActivities.first {
      options[.userActivityDictionary] = ["UIApplicationLaunchOptionsUserActivityKey": activity]
    }
    if let response = connectionOptions.notificationResponse {
      options[.remoteNotification] = bridgeNotificationResponse(response)
    }
    let sceneWindow = UIWindow(windowScene: scene)
    window = sceneWindow
    app.window = sceneWindow
    // RCTDeviceInfo captures the key window when it initializes and observes its
    // frame. Make the scene window key before creating the React root so resizing
    // is observed even if the module initializes synchronously.
    sceneWindow.rootViewController = UIViewController()
    sceneWindow.makeKeyAndVisible()
    factory.startReactNative(withModuleName: "GtmuxMobile", in: sceneWindow, launchOptions: options)
  }

  func scene(_ scene: UIScene, openURLContexts contexts: Set<UIOpenURLContext>) {
    for context in contexts {
      var options: [UIApplication.OpenURLOptionsKey: Any] = [:]
      if let source = context.options.sourceApplication { options[.sourceApplication] = source }
      if let annotation = context.options.annotation { options[.annotation] = annotation }
      RCTLinkingManager.application(UIApplication.shared, open: context.url, options: options)
    }
  }

  func scene(_ scene: UIScene, continue userActivity: NSUserActivity) {
    RCTLinkingManager.application(UIApplication.shared, continue: userActivity, restorationHandler: { _ in })
  }
}

// Both UIKit entry points must encode the same response identity and action.
private func bridgeNotificationResponse(_ response: UNNotificationResponse) -> [AnyHashable: Any] {
  let action = response.actionIdentifier == UNNotificationDefaultActionIdentifier ? nil : response.actionIdentifier
  return notificationResponsePayload(
    userInfo: response.notification.request.content.userInfo,
    requestID: response.notification.request.identifier,
    actionID: action,
    userText: (response as? UNTextInputNotificationResponse)?.userText
  )
}
