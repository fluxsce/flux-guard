package fluxguard.agent;

import java.lang.management.BufferPoolMXBean;
import java.lang.management.ClassLoadingMXBean;
import java.lang.management.GarbageCollectorMXBean;
import java.lang.management.ManagementFactory;
import java.lang.management.MemoryPoolMXBean;
import java.lang.management.MemoryUsage;
import java.lang.management.RuntimeMXBean;
import java.lang.management.ThreadInfo;
import java.lang.management.ThreadMXBean;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;

// Collect 读取本进程的 MXBean，组成协议里的 java 批次。
final class Collect {
    private Collect() {
    }

    static String snapshot(String appName) {
        long now = System.currentTimeMillis();
        String at = Instant.ofEpochMilli(now).toString();
        Pauses.install();
        RuntimeMXBean runtime = ManagementFactory.getRuntimeMXBean();
        Identity who = Identity.get(runtime);
        MemoryUsage heap = ManagementFactory.getMemoryMXBean().getHeapMemoryUsage();
        MemoryUsage nonHeap = ManagementFactory.getMemoryMXBean().getNonHeapMemoryUsage();
        MemoryUsage meta = metaspace();
        ThreadMXBean threads = ManagementFactory.getThreadMXBean();
        String pid = who.processId;
        String host = who.hostName;
        String ip = who.ipAddress;
        String mainClass = who.mainClass;
        String name = appName == null ? "" : appName.trim();
        if (name.isEmpty()) {
            name = simpleName(mainClass);
        }
        if (name.isEmpty()) {
            name = "java";
        }
        List<String> samples = new ArrayList<String>();
        add(samples, "jvm.heap.used.bytes", heap.getUsed(), null, at);
        add(samples, "jvm.heap.committed.bytes", heap.getCommitted(), null, at);
        add(samples, "jvm.heap.max.bytes", heap.getMax(), null, at);
        add(samples, "jvm.nonheap.used.bytes", nonHeap.getUsed(), null, at);
        add(samples, "jvm.nonheap.committed.bytes", nonHeap.getCommitted(), null, at);
        add(samples, "jvm.nonheap.max.bytes", nonHeap.getMax(), null, at);
        if (meta != null) {
            add(samples, "jvm.metaspace.used.bytes", meta.getUsed(), null, at);
            add(samples, "jvm.metaspace.committed.bytes", meta.getCommitted(), null, at);
        }
        for (GarbageCollectorMXBean gc : ManagementFactory.getGarbageCollectorMXBeans()) {
            long count = gc.getCollectionCount();
            long time = gc.getCollectionTime();
            if (count < 0 && time < 0) {
                continue;
            }
            String collector = gc.getName();
            if (count >= 0) {
                add(samples, "jvm.gc.count", count, collector, at);
            }
            if (time >= 0) {
                add(samples, "jvm.gc.time.ms", time, collector, at);
            }
        }
        add(samples, "jvm.thread.count", threads.getThreadCount(), null, at);
        add(samples, "jvm.thread.daemon.count", threads.getDaemonThreadCount(), null, at);
        add(samples, "jvm.thread.peak.count", threads.getPeakThreadCount(), null, at);
        long[] states = threadStates(threads);
        add(samples, "jvm.thread.runnable.count", states[0], null, at);
        add(samples, "jvm.thread.blocked.count", states[1], null, at);
        add(samples, "jvm.thread.waiting.count", states[2], null, at);
        for (Map.Entry<String, long[]> pool : pools().entrySet()) {
            addLabeled(samples, "jvm.memory.pool.used.bytes", pool.getValue()[0], "pool", pool.getKey(), at);
            addLabeled(samples, "jvm.memory.pool.committed.bytes", pool.getValue()[1], "pool", pool.getKey(), at);
        }
        long directMax = directMemoryLimit();
        if (directMax > 0) {
            addLabeled(samples, "jvm.memory.pool.max.bytes", directMax, "pool", "direct", at);
        }
        double cpu = processCpuPercent();
        if (cpu >= 0) {
            add(samples, "jvm.cpu.process.percent", Math.round(cpu * 1000.0) / 1000.0, null, at);
        }
        ClassLoadingMXBean classes = ManagementFactory.getClassLoadingMXBean();
        add(samples, "jvm.class.loaded", classes.getLoadedClassCount(), null, at);
        add(samples, "jvm.class.unloaded", classes.getUnloadedClassCount(), null, at);
        for (Map.Entry<String, Pauses> pause : Pauses.drain().entrySet()) {
            if (pause.getValue().count <= 0) {
                continue;
            }
            addLabeled(samples, "jvm.gc.pause.count", pause.getValue().count, "collector", pause.getKey(), at);
            addLabeled(samples, "jvm.gc.pause.max.ms", pause.getValue().maxMs, "collector", pause.getKey(), at);
            addLabeled(samples, "jvm.gc.pause.time.ms", pause.getValue().totalMs, "collector", pause.getKey(), at);
        }

        StringBuilder body = new StringBuilder();
        body.append('{');
        field(body, "batchId", UUID.randomUUID().toString(), true);
        field(body, "agentCode", "java", true);
        field(body, "sentAt", at, true);
        body.append("\"samples\":[");
        for (int i = 0; i < samples.size(); i++) {
            if (i > 0) {
                body.append(',');
            }
            body.append(samples.get(i));
        }
        body.append("],\"java\":{");
        field(body, "appName", name, true);
        field(body, "mainClass", mainClass, !mainClass.isEmpty());
        field(body, "processId", pid, true);
        field(body, "javaVersion", who.javaVersion, true);
        field(body, "startTime", Instant.ofEpochMilli(who.startTime).toString(), true);
        field(body, "hostName", host, !host.isEmpty());
        field(body, "ipAddress", ip, !ip.isEmpty());
        trimComma(body);
        body.append("}}");
        return body.toString();
    }

    private static void add(List<String> samples, String metric, long value, String collector, String at) {
        addLabeled(samples, metric, value, "collector", collector, at);
    }

    private static void add(List<String> samples, String metric, double value, String collector, String at) {
        addLabeled(samples, metric, value, "collector", collector, at);
    }

    private static void addLabeled(List<String> samples, String metric, double value, String labelKey, String labelValue, String at) {
        StringBuilder row = new StringBuilder();
        row.append('{');
        field(row, "metricName", metric, true);
        row.append("\"metricValue\":").append(value).append(',');
        if (labelKey != null && labelValue != null && !labelValue.isEmpty()) {
            row.append("\"labels\":{").append(Json.quote(labelKey)).append(':').append(Json.quote(labelValue)).append("},");
        }
        field(row, "observedAt", at, true);
        trimComma(row);
        row.append('}');
        samples.add(row.toString());
    }

    private static void field(StringBuilder body, String key, String value, boolean include) {
        if (!include || value == null) {
            return;
        }
        body.append(Json.quote(key)).append(':').append(Json.quote(value)).append(',');
    }

    private static void trimComma(StringBuilder body) {
        int n = body.length();
        if (n > 0 && body.charAt(n - 1) == ',') {
            body.setLength(n - 1);
        }
    }

    private static MemoryUsage metaspace() {
        for (MemoryPoolMXBean pool : ManagementFactory.getMemoryPoolMXBeans()) {
            if ("Metaspace".equals(pool.getName())) {
                return pool.getUsage();
            }
        }
        return null;
    }

    private static long[] threadStates(ThreadMXBean threads) {
        long[] ids = threads.getAllThreadIds();
        long runnable = 0;
        long blocked = 0;
        long waiting = 0;
        if (ids.length == 0) {
            return new long[]{0, 0, 0};
        }
        ThreadInfo[] infos = threads.getThreadInfo(ids, 0);
        if (infos == null) {
            return new long[]{runnable, blocked, waiting};
        }
        for (ThreadInfo info : infos) {
            if (info == null || info.getThreadState() == null) {
                continue;
            }
            switch (info.getThreadState()) {
                case RUNNABLE:
                    runnable++;
                    break;
                case BLOCKED:
                    blocked++;
                    break;
                case WAITING:
                case TIMED_WAITING:
                    waiting++;
                    break;
                default:
                    break;
            }
        }
        return new long[]{runnable, blocked, waiting};
    }

    private static long directLimit = Long.MIN_VALUE;

    private static long directMemoryLimit() {
        long current = directLimit;
        if (current != Long.MIN_VALUE) {
            return current;
        }
        synchronized (Collect.class) {
            if (directLimit != Long.MIN_VALUE) {
                return directLimit;
            }
            long resolved = reflectDirectLimit();
            if (resolved <= 0) {
                resolved = flagDirectLimit();
            }
            if (resolved <= 0) {
                resolved = Runtime.getRuntime().maxMemory();
            }
            directLimit = resolved;
            return directLimit;
        }
    }

    private static long reflectDirectLimit() {
        String[] names = {"jdk.internal.misc.VM", "sun.misc.VM"};
        for (String name : names) {
            try {
                Class<?> type = Class.forName(name);
                Object value = type.getMethod("maxDirectMemory").invoke(null);
                if (value instanceof Number) {
                    return ((Number) value).longValue();
                }
            } catch (Throwable ignored) {
                // JDK 17 默认不开放这个包，退回启动参数。
            }
        }
        return -1;
    }

    private static long flagDirectLimit() {
        for (String arg : ManagementFactory.getRuntimeMXBean().getInputArguments()) {
            String prefix = "-XX:MaxDirectMemorySize=";
            if (arg != null && arg.startsWith(prefix)) {
                return parseSize(arg.substring(prefix.length()));
            }
        }
        return -1;
    }

    private static long parseSize(String text) {
        if (text == null) {
            return -1;
        }
        text = text.trim().toLowerCase();
        if (text.isEmpty()) {
            return -1;
        }
        long scale = 1;
        char tail = text.charAt(text.length() - 1);
        if (tail == 'k' || tail == 'm' || tail == 'g') {
            text = text.substring(0, text.length() - 1);
            if (tail == 'k') {
                scale = 1024L;
            } else if (tail == 'm') {
                scale = 1024L * 1024L;
            } else {
                scale = 1024L * 1024L * 1024L;
            }
        }
        try {
            return Long.parseLong(text) * scale;
        } catch (NumberFormatException e) {
            return -1;
        }
    }

    private static Map<String, long[]> pools() {
        Map<String, long[]> rows = new LinkedHashMap<String, long[]>();
        for (MemoryPoolMXBean pool : ManagementFactory.getMemoryPoolMXBeans()) {
            String kind = poolKind(pool.getName());
            if (kind.isEmpty()) {
                continue;
            }
            MemoryUsage usage = pool.getUsage();
            if (usage == null || usage.getUsed() < 0) {
                continue;
            }
            long[] row = rows.get(kind);
            if (row == null) {
                row = new long[2];
                rows.put(kind, row);
            }
            row[0] += usage.getUsed();
            if (usage.getCommitted() > 0) {
                row[1] += usage.getCommitted();
            }
        }
        for (BufferPoolMXBean pool : ManagementFactory.getPlatformMXBeans(BufferPoolMXBean.class)) {
            String kind = "direct".equals(pool.getName()) ? "direct" : "mapped".equals(pool.getName()) ? "mapped" : "";
            if (kind.isEmpty() || pool.getMemoryUsed() < 0) {
                continue;
            }
            long[] row = new long[]{pool.getMemoryUsed(), pool.getTotalCapacity()};
            rows.put(kind, row);
        }
        return rows;
    }

    private static String poolKind(String name) {
        if (name == null) {
            return "";
        }
        String lower = name.toLowerCase();
        if (lower.contains("eden")) {
            return "eden";
        }
        if (lower.contains("survivor")) {
            return "survivor";
        }
        if (lower.contains("old") || lower.contains("tenured")) {
            return "old";
        }
        if (lower.contains("codeheap") || lower.contains("code cache")) {
            return "code";
        }
        if (lower.contains("compressed class")) {
            return "classspace";
        }
        return "";
    }

    private static double processCpuPercent() {
        java.lang.management.OperatingSystemMXBean bean = ManagementFactory.getOperatingSystemMXBean();
        if (!(bean instanceof com.sun.management.OperatingSystemMXBean)) {
            return -1;
        }
        double load = ((com.sun.management.OperatingSystemMXBean) bean).getProcessCpuLoad();
        if (load < 0) {
            return -1;
        }
        return load * 100.0;
    }

    private static String simpleName(String mainClass) {
        if (mainClass == null || mainClass.isEmpty()) {
            return "";
        }
        int dot = mainClass.lastIndexOf('.');
        int slash = Math.max(mainClass.lastIndexOf('/'), mainClass.lastIndexOf('\\'));
        int cut = Math.max(dot, slash);
        if (cut >= 0 && cut + 1 < mainClass.length()) {
            return mainClass.substring(cut + 1);
        }
        return mainClass;
    }
}
