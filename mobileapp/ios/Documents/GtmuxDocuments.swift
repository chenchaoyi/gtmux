import UIKit
import React

@objc(GtmuxDocuments)
final class GtmuxDocuments: NSObject {
  private weak var current: UIViewController?
  @objc static func requiresMainQueueSetup() -> Bool { true }

  @objc(open:resolver:rejecter:)
  func open(_ request: [String: Any], resolver resolve: @escaping RCTPromiseResolveBlock,
            rejecter reject: @escaping RCTPromiseRejectBlock) {
    DispatchQueue.main.async { [weak self] in
      guard let self, let presenter = RCTPresentedViewController(), self.current?.presentingViewController == nil,
            DocumentTransport(request: request) != nil else { reject("preview_unavailable", "File preview is unavailable", nil); return }
      let document = DocumentPreviewController(request: request)
      let navigation = UINavigationController(rootViewController: document)
      if presenter.traitCollection.userInterfaceIdiom == .pad {
        navigation.modalPresentationStyle = .formSheet
        let size = presenter.view.bounds.size
        navigation.preferredContentSize = CGSize(width: min(1000, max(540, size.width - 64)), height: min(900, max(400, size.height - 64)))
      } else { navigation.modalPresentationStyle = .fullScreen }
      self.current = navigation
      presenter.present(navigation, animated: true) { resolve(nil) }
    }
  }
}
