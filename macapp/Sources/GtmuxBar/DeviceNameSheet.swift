import SwiftUI

/// A roster label, not an asserted system device name. Administration stays on the Mac.
struct DeviceNameSheet: View {
    let device: PairedDevice
    @ObservedObject var l10n: L10n
    @ObservedObject var store: PairStore
    let close: () -> Void
    @State private var name: String
    @State private var error = false
    @FocusState private var focused: Bool

    init(device: PairedDevice, l10n: L10n, store: PairStore, close: @escaping () -> Void) {
        self.device = device
        self.l10n = l10n
        self.store = store
        self.close = close
        _name = State(initialValue: device.displayName)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text(l10n.tr("Edit device name", "编辑设备名称")).font(.headline)
            Text(l10n.tr("Use a name that helps you identify this device. Its system name stays unchanged.",
                         "设置一个便于识别的名称，不会修改设备的系统名称。"))
                .font(.callout).foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            TextField(l10n.tr("Device name", "设备名称"), text: $name)
                .textFieldStyle(.roundedBorder).focused($focused)
                .disabled(store.busy).onSubmit(save)
            if error {
                Text(l10n.tr("Could not save the name. Check the Mac connection and try again.",
                             "名称未保存。请检查 Mac 连接后重试。"))
                    .font(.callout).foregroundStyle(.red)
            }
            HStack {
                Spacer()
                Button(l10n.tr("Cancel", "取消"), action: close)
                    .keyboardShortcut(.cancelAction).disabled(store.busy)
                Button(l10n.tr("Save", "保存"), action: save)
                    .keyboardShortcut(.defaultAction)
                    .disabled(store.busy || name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        }
        .padding(20).frame(width: 360)
        .onAppear { focused = true }
    }

    private func save() {
        guard !store.busy else { return }
        error = false
        store.rename(device.id, name: name) { ok in
            if ok { close() } else { error = true }
        }
    }
}
