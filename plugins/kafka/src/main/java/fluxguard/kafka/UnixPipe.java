package fluxguard.kafka;

import java.io.IOException;
import java.net.StandardProtocolFamily;
import java.net.UnixDomainSocketAddress;
import java.nio.ByteBuffer;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;

final class UnixPipe {
    private UnixPipe() {
    }

    static void listen(String address, ExecutorService calls) throws IOException {
        Path path = Path.of(address);
        Files.deleteIfExists(path);
        try (ServerSocketChannel server = ServerSocketChannel.open(StandardProtocolFamily.UNIX)) {
            server.bind(UnixDomainSocketAddress.of(path));
            while (!Thread.currentThread().isInterrupted()) {
                SocketChannel channel = server.accept();
                calls.submit(() -> {
                    try (Connection pipe = new Connection(channel)) {
                        Session.serve(pipe);
                    } catch (Exception ignored) {
                        // 这一条连接结束不影响其它调用。
                    }
                });
            }
        }
    }

    private static final class Connection implements BytePipe {
        private final SocketChannel channel;

        Connection(SocketChannel channel) {
            this.channel = channel;
        }

        @Override
        public byte[] readFully(int n) throws IOException {
            ByteBuffer buf = ByteBuffer.allocate(n);
            while (buf.hasRemaining()) {
                if (channel.read(buf) < 0) {
                    throw new Frame.EOF();
                }
            }
            return buf.array();
        }

        @Override
        public void writeFully(byte[] payload) throws IOException {
            ByteBuffer buf = ByteBuffer.wrap(payload);
            while (buf.hasRemaining()) {
                if (channel.write(buf) < 0) {
                    throw new IOException("write failed");
                }
            }
        }

        @Override
        public void close() throws IOException {
            channel.close();
        }
    }
}
