import Foundation

/// Accepts connections from `cde` on a Unix socket and hands every event it
/// reads to `deliver` on the main queue. Each client is read separately, so a
/// slow or broken one never holds up the others.
final class AgentSocketListener {
    /// Longer lines are not events `cde` would send, so the client is dropped
    /// rather than buffered without limit.
    static let maximumLineLength = 64 * 1024

    let url: URL
    private let deliver: @MainActor (AgentEvent) -> Void
    private let queue = DispatchQueue(label: "dev.phucngo.code.agent-socket")
    private static let queueKey = DispatchSpecificKey<Void>()
    private var acceptSource: DispatchSourceRead?
    private var clients: [Int32: Client] = [:]

    private final class Client {
        let source: DispatchSourceRead
        var buffer = Data()
        init(source: DispatchSourceRead) { self.source = source }
    }

    init(url: URL = AgentEnvironment.socketURL(), deliver: @escaping @MainActor (AgentEvent) -> Void) {
        self.url = url
        self.deliver = deliver
        queue.setSpecific(key: Self.queueKey, value: ())
    }

    /// A Code that crashed or was killed left its socket behind.
    static func removeStaleSockets(in directory: URL = AgentEnvironment.socketDirectory) {
        for socket in AgentEnvironment.sockets(in: directory) where !AgentEnvironment.isRunning(socket.pid) {
            unlink(socket.url.path)
        }
    }

    deinit {
        stop()
    }

    func start() throws {
        let path = url.path
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        unlink(path)
        guard let fd = UnixSocket.make() else { throw POSIXError.current }
        let previousMask = umask(0o177)
        let bound = UnixSocket.withAddress(path) { bind(fd, $0, $1) }
        umask(previousMask)
        guard let bound else {
            close(fd)
            throw POSIXError(.ENAMETOOLONG)
        }
        guard bound == 0, chmod(path, 0o600) == 0, listen(fd, 16) == 0, fcntl(fd, F_SETFL, O_NONBLOCK) == 0 else {
            let error = POSIXError.current
            close(fd)
            unlink(path)
            throw error
        }
        let source = DispatchSource.makeReadSource(fileDescriptor: fd, queue: queue)
        source.setEventHandler { [weak self] in self?.acceptClients(on: fd) }
        source.setCancelHandler { close(fd) }
        acceptSource = source
        source.resume()
    }

    func stop() {
        guard let source = acceptSource else { return }
        acceptSource = nil
        source.cancel()
        // Runs on the socket queue unless we are already on it, which would
        // otherwise deadlock.
        let cancelClients = {
            self.clients.values.forEach { $0.source.cancel() }
            self.clients.removeAll()
        }
        if DispatchQueue.getSpecific(key: Self.queueKey) != nil {
            cancelClients()
        } else {
            queue.sync(execute: cancelClients)
        }
        unlink(url.path)
    }

    private func acceptClients(on listener: Int32) {
        while true {
            let fd = accept(listener, nil, nil)
            guard fd >= 0 else { return }
            UnixSocket.prepare(fd)
            _ = fcntl(fd, F_SETFL, O_NONBLOCK)
            let source = DispatchSource.makeReadSource(fileDescriptor: fd, queue: queue)
            clients[fd] = Client(source: source)
            source.setEventHandler { [weak self] in self?.read(from: fd) }
            source.setCancelHandler { close(fd) }
            source.resume()
        }
    }

    private func read(from fd: Int32) {
        guard let client = clients[fd] else { return }
        var chunk = [UInt8](repeating: 0, count: 4096)
        let count = Darwin.read(fd, &chunk, chunk.count)
        if count < 0, errno == EAGAIN || errno == EINTR { return }
        guard count > 0 else {
            handle(client.buffer)
            disconnect(fd)
            return
        }
        client.buffer.append(contentsOf: chunk[..<count])
        while let newline = client.buffer.firstIndex(of: UInt8(ascii: "\n")) {
            guard client.buffer.distance(from: client.buffer.startIndex, to: newline) <= Self.maximumLineLength else {
                rejectOversizedLine(fd)
                return
            }
            handle(client.buffer[client.buffer.startIndex..<newline])
            // Removed in place rather than kept as a slice, so a long-lived
            // client never pins the buffer's high-water mark.
            client.buffer.removeSubrange(client.buffer.startIndex...newline)
        }
        if client.buffer.count > Self.maximumLineLength {
            rejectOversizedLine(fd)
        }
    }

    private func rejectOversizedLine(_ fd: Int32) {
        logger.error("agent socket: dropped a client sending a line over \(Self.maximumLineLength) bytes")
        disconnect(fd)
    }

    private func disconnect(_ fd: Int32) {
        clients.removeValue(forKey: fd)?.source.cancel()
    }

    private func handle(_ line: Data) {
        guard !line.isEmpty else { return }
        do {
            let event = try AgentEvent(line: Data(line))
            DispatchQueue.main.async { [deliver] in deliver(event) }
        } catch {
            logger.error("agent socket: dropped a malformed line: \(error, privacy: .public)")
        }
    }
}

private extension POSIXError {
    static var current: POSIXError { POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO) }
}
