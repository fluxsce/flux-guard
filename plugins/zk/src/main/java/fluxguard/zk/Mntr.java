package fluxguard.zk;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.charset.StandardCharsets;

final class Mntr {
    private Mntr() {
    }

    static String read(String hostPort) throws IOException {
        Host host = host(hostPort);
        try (Socket socket = new Socket()) {
            socket.connect(new InetSocketAddress(host.name, host.port), 3000);
            socket.setSoTimeout(3000);
            socket.getOutputStream().write("mntr\n".getBytes(StandardCharsets.US_ASCII));
            socket.shutdownOutput();
            byte[] body = socket.getInputStream().readNBytes(64 * 1024);
            return new String(body, StandardCharsets.UTF_8);
        }
    }

    private static Host host(String hostPort) throws IOException {
        if (hostPort.startsWith("[")) {
            int end = hostPort.indexOf(']');
            if (end < 0 || end + 2 >= hostPort.length() || hostPort.charAt(end + 1) != ':') {
                throw new IOException("地址无效");
            }
            return new Host(hostPort.substring(1, end), Integer.parseInt(hostPort.substring(end + 2)));
        }
        int colon = hostPort.lastIndexOf(':');
        if (colon <= 0 || colon == hostPort.length() - 1) {
            throw new IOException("地址无效");
        }
        return new Host(hostPort.substring(0, colon), Integer.parseInt(hostPort.substring(colon + 1)));
    }

    private record Host(String name, int port) {
    }
}
