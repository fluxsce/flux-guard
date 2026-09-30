package fluxguard.agent;

import java.lang.instrument.Instrumentation;
import java.util.concurrent.atomic.AtomicBoolean;

// Agent 挂进目标 JVM，按间隔采集堆、GC 和线程并上报。
// 也可以用 java -jar 在当前进程里跑同一套采集。
public final class Agent {
    private static final AtomicBoolean STARTED = new AtomicBoolean();

    private Agent() {
    }

    public static void premain(String args, Instrumentation inst) {
        start(args, true);
    }

    public static void agentmain(String args, Instrumentation inst) {
        start(args, true);
    }

    public static void main(String[] args) {
        String joined = args == null ? "" : String.join(";", args);
        if (!start(joined, false)) {
            System.exit(1);
        }
    }

    static boolean start(String args, boolean daemon) {
        if (!STARTED.compareAndSet(false, true)) {
            return true;
        }
        Config cfg = Config.load(args);
        if (daemon) {
            Thread worker = new Thread(() -> loop(cfg), "flux-guard-agent-java");
            worker.setDaemon(true);
            worker.start();
            return true;
        }
        return loop(cfg);
    }

    static boolean loop(Config cfg) {
        Report report = new Report(cfg);
        while (!Thread.currentThread().isInterrupted()) {
            try {
                report.post(Collect.snapshot(cfg.appName));
            } catch (Exception e) {
                System.err.println("flux-guard-agent-java 上报: " + e.getMessage());
                if (cfg.once) {
                    return false;
                }
            }
            if (cfg.once) {
                return true;
            }
            try {
                Thread.sleep(cfg.intervalSec * 1000L);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return false;
            }
        }
        return false;
    }
}
