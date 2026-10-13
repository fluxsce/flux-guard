package fluxguard.agent;

import com.sun.management.GarbageCollectionNotificationInfo;
import com.sun.management.GcInfo;
import java.lang.management.GarbageCollectorMXBean;
import java.lang.management.ManagementFactory;
import java.lang.management.MemoryUsage;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import javax.management.Notification;
import javax.management.NotificationEmitter;
import javax.management.NotificationListener;
import javax.management.openmbean.CompositeData;

// Pauses 记下两次上报之间的回收。停顿和周期分开。
// 开销分母是上一拍到这一拍的实际毫秒，第一拍没有分母就不算。
// p50、p99 是本区间时长的最近秩，不是全天分位。
final class Pauses {
    long pauseCount;
    long pauseTotalMs;
    long pauseMaxMs = -1;
    long pauseP50Ms = -1;
    long pauseP99Ms = -1;
    long heapBefore = -1;
    long heapAfter = -1;
    // promoted 是本区间年轻代晋升合计。有一次测不到就整段记为未知，不能拿部分合计冒充。
    long promoted = -1;
    boolean promotionUnknown;
    String cause = "";
    long cycleCount;
    long cycleTotalMs;

    private final List<Long> pauses = new ArrayList<Long>();

    private static final Object LOCK = new Object();
    private static final Map<String, Pauses> OPEN = new HashMap<String, Pauses>();
    private static Map<String, Pauses> HELD;
    private static long lastAt;
    private static long pendingAt;
    private static boolean hooked;

    static void install() {
        synchronized (LOCK) {
            if (hooked) {
                return;
            }
            hooked = true;
        }
        NotificationListener listener = new NotificationListener() {
            @Override
            public void handleNotification(Notification notification, Object handback) {
                if (!GarbageCollectionNotificationInfo.GARBAGE_COLLECTION_NOTIFICATION.equals(notification.getType())) {
                    return;
                }
                if (!(notification.getUserData() instanceof CompositeData)) {
                    return;
                }
                GarbageCollectionNotificationInfo info = GarbageCollectionNotificationInfo.from((CompositeData) notification.getUserData());
                long duration = info.getGcInfo().getDuration();
                if (duration < 0) {
                    return;
                }
                String name = info.getGcName();
                if (name == null || name.isEmpty()) {
                    name = "gc";
                }
                String action = info.getGcAction();
                boolean stop = stopTheWorld(name, action);
                synchronized (LOCK) {
                    Pauses row = OPEN.get(name);
                    if (row == null) {
                        row = new Pauses();
                        OPEN.put(name, row);
                    }
                    if (!stop) {
                        row.cycleCount++;
                        row.cycleTotalMs += duration;
                        return;
                    }
                    row.pauses.add(Long.valueOf(duration));
                    row.pauseCount++;
                    row.pauseTotalMs += duration;
                    GcInfo gc = info.getGcInfo();
                    if (youngCollection(name, action)) {
                        row.addPromotion(promoted(gc.getMemoryUsageBeforeGc(), gc.getMemoryUsageAfterGc()));
                    }
                    if (duration > row.pauseMaxMs) {
                        row.pauseMaxMs = duration;
                        row.cause = info.getGcCause() == null ? "" : info.getGcCause();
                        row.heapBefore = heapUsed(gc.getMemoryUsageBeforeGc());
                        row.heapAfter = heapUsed(gc.getMemoryUsageAfterGc());
                    }
                }
            }
        };
        for (GarbageCollectorMXBean gc : ManagementFactory.getGarbageCollectorMXBeans()) {
            if (gc instanceof NotificationEmitter) {
                try {
                    ((NotificationEmitter) gc).addNotificationListener(listener, null, null);
                } catch (Exception ignored) {
                    // 个别回收器不接受监听时，仍保留累计次数。
                }
            }
        }
    }

    // drain 先拿走当前区间，但不推进时钟。上报成功后 ack，失败后 restore，避免丢停顿或拉低下一拍开销。
    static Map<String, Pauses> drain(long nowMs) {
        synchronized (LOCK) {
            if (HELD != null) {
                mergeHeld();
            }
            Map<String, Pauses> copy = new HashMap<String, Pauses>(OPEN);
            OPEN.clear();
            HELD = copy;
            pendingAt = nowMs;
            for (Pauses row : copy.values()) {
                row.finish();
            }
            return copy;
        }
    }

    static void ack() {
        synchronized (LOCK) {
            HELD = null;
            if (pendingAt > lastAt) {
                lastAt = pendingAt;
            }
            pendingAt = 0;
        }
    }

    static void restore() {
        synchronized (LOCK) {
            if (HELD != null) {
                mergeHeld();
            }
            pendingAt = 0;
        }
    }

    // overheadPercent 用上次成功上报到这一拍的实际间隔。第一拍、失败未确认或时钟回退返回 -1。
    static double overheadPercent(long nowMs, long stwMs) {
        long prev;
        synchronized (LOCK) {
            prev = lastAt;
        }
        if (prev <= 0 || nowMs <= prev || stwMs < 0) {
            return -1;
        }
        return (stwMs * 100.0) / (double) (nowMs - prev);
    }

    private static void mergeHeld() {
        for (Map.Entry<String, Pauses> entry : HELD.entrySet()) {
            Pauses incoming = OPEN.get(entry.getKey());
            if (incoming == null) {
                OPEN.put(entry.getKey(), entry.getValue());
                continue;
            }
            entry.getValue().absorb(incoming);
            OPEN.put(entry.getKey(), entry.getValue());
        }
        HELD = null;
    }

    private void addPromotion(long delta) {
        if (promotionUnknown) {
            return;
        }
        if (delta < 0) {
            promotionUnknown = true;
            promoted = -1;
            return;
        }
        if (promoted < 0) {
            promoted = 0;
        }
        promoted += delta;
    }

    private void absorb(Pauses extra) {
        pauses.addAll(extra.pauses);
        pauseCount += extra.pauseCount;
        pauseTotalMs += extra.pauseTotalMs;
        cycleCount += extra.cycleCount;
        cycleTotalMs += extra.cycleTotalMs;
        if (extra.pauseMaxMs > pauseMaxMs) {
            pauseMaxMs = extra.pauseMaxMs;
            cause = extra.cause;
            heapBefore = extra.heapBefore;
            heapAfter = extra.heapAfter;
        }
        if (promotionUnknown || extra.promotionUnknown) {
            promotionUnknown = true;
            promoted = -1;
            return;
        }
        if (extra.promoted >= 0) {
            if (promoted < 0) {
                promoted = 0;
            }
            promoted += extra.promoted;
        }
    }

    private void finish() {
        if (pauses.isEmpty()) {
            return;
        }
        long[] values = new long[pauses.size()];
        for (int i = 0; i < pauses.size(); i++) {
            values[i] = pauses.get(i).longValue();
        }
        Arrays.sort(values);
        pauseP50Ms = nearestRank(values, 0.50);
        pauseP99Ms = nearestRank(values, 0.99);
    }

    // nearestRank 是最近秩：rank = ceil(p * n)，取排序后的这一位。n=1 时 p99 就是这一次。
    static long nearestRank(long[] sorted, double p) {
        int n = sorted.length;
        if (n == 0) {
            return -1;
        }
        int rank = (int) Math.ceil(p * n);
        if (rank < 1) {
            rank = 1;
        }
        if (rank > n) {
            rank = n;
        }
        return sorted[rank - 1];
    }

    // stopTheWorld 认停顿。周期、并发以及认不出的通知都不算停顿，避免把周期耗时送进预警。
    static boolean stopTheWorld(String name, String action) {
        String actionText = action == null ? "" : action.toLowerCase();
        String nameText = name == null ? "" : name.toLowerCase();
        if (actionText.contains("concurrent")) {
            return false;
        }
        if (nameText.contains("cycle")) {
            return false;
        }
        if (nameText.contains("pause")) {
            return true;
        }
        if (actionText.contains("minor") || actionText.contains("major")) {
            return true;
        }
        if (nameText.contains("concurrent")) {
            return false;
        }
        return nameText.contains("young")
                || nameText.contains("scavenge")
                || nameText.contains("parnew")
                || nameText.equals("copy")
                || nameText.contains("marksweep")
                || nameText.contains("tenured")
                || nameText.contains("old");
    }

    static boolean youngCollection(String name, String action) {
        String actionText = action == null ? "" : action.toLowerCase();
        String nameText = name == null ? "" : name.toLowerCase();
        if (actionText.contains("major") || actionText.contains("concurrent")) {
            return false;
        }
        if (nameText.contains("old") || nameText.contains("tenured") || nameText.contains("marksweep") || nameText.contains("major")) {
            return false;
        }
        if (actionText.contains("minor")) {
            return true;
        }
        return nameText.contains("young")
                || nameText.contains("scavenge")
                || nameText.contains("parnew")
                || nameText.equals("copy")
                || (nameText.contains("minor") && nameText.contains("pause"));
    }

    private static long heapUsed(Map<String, MemoryUsage> usage) {
        if (usage == null) {
            return -1;
        }
        long sum = 0;
        boolean any = false;
        for (Map.Entry<String, MemoryUsage> entry : usage.entrySet()) {
            if (!heapPool(entry.getKey())) {
                continue;
            }
            MemoryUsage value = entry.getValue();
            if (value == null || value.getUsed() < 0) {
                continue;
            }
            sum += value.getUsed();
            any = true;
        }
        return any ? sum : -1;
    }

    private static long promoted(Map<String, MemoryUsage> before, Map<String, MemoryUsage> after) {
        long from = oldUsed(before);
        long to = oldUsed(after);
        if (from < 0 || to < 0 || to < from) {
            return -1;
        }
        return to - from;
    }

    private static long oldUsed(Map<String, MemoryUsage> usage) {
        if (usage == null) {
            return -1;
        }
        long sum = 0;
        boolean any = false;
        for (Map.Entry<String, MemoryUsage> entry : usage.entrySet()) {
            String name = entry.getKey() == null ? "" : entry.getKey().toLowerCase();
            if (!name.contains("old") && !name.contains("tenured")) {
                continue;
            }
            MemoryUsage value = entry.getValue();
            if (value == null || value.getUsed() < 0) {
                continue;
            }
            sum += value.getUsed();
            any = true;
        }
        return any ? sum : -1;
    }

    private static boolean heapPool(String name) {
        if (name == null) {
            return false;
        }
        String text = name.toLowerCase();
        if (text.contains("metaspace") || text.contains("class") || text.contains("code")) {
            return false;
        }
        return text.contains("eden") || text.contains("survivor") || text.contains("old") || text.contains("tenured") || text.contains("young") || text.equals("heap");
    }
}
