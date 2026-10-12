package fluxguard.zk;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

// 监听中央给出的本机管道。每条连接单独处理，采集和目录操作可以同时进行。
public final class Main {
    public static void main(String[] args) throws Exception {
        // 官方客户端默认只收 1MB。目录内容按本插件的返回上限读取。
        System.setProperty("jute.maxbuffer", Integer.toString(Outcome.PAYLOAD));
        String address = System.getenv("FG_PLUGIN_ADDRESS");
        if (address == null || address.isBlank()) {
            System.exit(1);
        }
        ExecutorService calls = Executors.newFixedThreadPool(64, runnable -> {
            Thread thread = new Thread(runnable, "zk-call");
            thread.setDaemon(true);
            return thread;
        });
        PipeServer.listen(address.trim(), calls);
    }

    private Main() {
    }
}
