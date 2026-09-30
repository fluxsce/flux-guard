package fluxguard.agent;

import com.sun.management.GarbageCollectionNotificationInfo;
import java.lang.management.GarbageCollectorMXBean;
import java.lang.management.ManagementFactory;
import java.util.HashMap;
import java.util.Map;
import javax.management.Notification;
import javax.management.NotificationEmitter;
import javax.management.NotificationListener;
import javax.management.openmbean.CompositeData;

// Pauses 记下两次上报之间每次回收的停顿。累计次数仍走 MXBean，这里只补最长停顿。
final class Pauses {
    long count;
    long totalMs;
    long maxMs;

    private static final Object LOCK = new Object();
    private static final Map<String, Pauses> OPEN = new HashMap<String, Pauses>();
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
                synchronized (LOCK) {
                    Pauses row = OPEN.get(name);
                    if (row == null) {
                        row = new Pauses();
                        OPEN.put(name, row);
                    }
                    row.count++;
                    row.totalMs += duration;
                    if (duration > row.maxMs) {
                        row.maxMs = duration;
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

    static Map<String, Pauses> drain() {
        synchronized (LOCK) {
            if (OPEN.isEmpty()) {
                return new HashMap<String, Pauses>();
            }
            Map<String, Pauses> copy = new HashMap<String, Pauses>(OPEN);
            OPEN.clear();
            return copy;
        }
    }
}
