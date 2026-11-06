import SwiftUI

/// Agents that load Code's extension from their own config folder. Installing
/// there keeps the user's binary on PATH untouched, so `which pi` still points
/// at their own install.
struct AgentsSettingsView: View {
    @State private var extensions = AgentExtensions.all()
    @State private var statuses: [String: AgentExtension.Status] = [:]
    @State private var error: String?

    var body: some View {
        Form {
            Section {
                ForEach(extensions, id: \.destination.path) { ext in
                    HStack(alignment: .top, spacing: 12) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(ext.agent)
                            Text(ext.destination.path.abbreviatingHome)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            Text(status(of: ext))
                                .font(.caption)
                                .foregroundStyle(color(of: ext))
                        }
                        Spacer()
                        Button(title(for: ext)) { install(ext) }
                    }
                }
                if let error {
                    Label(error, systemImage: "exclamationmark.triangle.fill")
                        .symbolRenderingMode(.multicolor)
                        .font(.caption)
                }
            } footer: {
                Text("Code's extension lets the agent report what it is doing, so the sidebar shows its status. Install it again after updating Code.")
                    .settingsFooter()
            }
        }
        .formStyle(.grouped)
        .onAppear(perform: refresh)
    }

    private func refresh() {
        statuses = Dictionary(uniqueKeysWithValues: extensions.map { ($0.destination.path, $0.status()) })
    }

    private func status(of ext: AgentExtension) -> String {
        switch statuses[ext.destination.path] {
        case .installed: "Installed"
        case .outdated: "Update available"
        case .missing: "Not installed"
        case nil: ""
        }
    }

    private func color(of ext: AgentExtension) -> Color {
        statuses[ext.destination.path] == .installed ? .secondary : .orange
    }

    private func title(for ext: AgentExtension) -> String {
        statuses[ext.destination.path] == .installed ? "Reinstall" : "Install"
    }

    private func install(_ ext: AgentExtension) {
        do {
            try ext.install()
            error = nil
        } catch {
            self.error = "\(error)"
        }
        refresh()
    }
}
