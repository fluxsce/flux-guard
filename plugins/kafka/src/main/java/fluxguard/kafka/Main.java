package fluxguard.kafka;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

// 监听中央给出的本机管道。每条连接单独处理，采集和维护可以同时进行。
public final class Main {
    public static void main(String[] args) throws Exception {
        String address = System.getenv("FG_PLUGIN_ADDRESS");
        if (address == null || address.isBlank()) {
            System.exit(1);
        }
        ExecutorService calls = Executors.newFixedThreadPool(64, runnable -> {
            Thread thread = new Thread(runnable, "kafka-call");
            thread.setDaemon(true);
            return thread;
        });
        PipeServer.listen(address.trim(), calls);
    }

    private Main() {
    }
}
