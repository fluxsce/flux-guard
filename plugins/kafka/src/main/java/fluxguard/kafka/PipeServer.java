package fluxguard.kafka;

import java.io.IOException;
import java.util.concurrent.ExecutorService;

final class PipeServer {
    private PipeServer() {
    }

    static void listen(String address, ExecutorService calls) throws IOException {
        if (System.getProperty("os.name", "").toLowerCase().contains("windows")) {
            WinPipe.listen(address, calls);
            return;
        }
        UnixPipe.listen(address, calls);
    }
}
