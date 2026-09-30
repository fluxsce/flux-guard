package fluxguard.agent;

import java.lang.management.RuntimeMXBean;
import java.net.InetAddress;

// Identity 缓存进程生命周期内不变的资料。主机名只解析一次，避免每轮反向 DNS。
final class Identity {
    String hostName = "";
    String ipAddress = "";
    String processId = "";
    String mainClass = "";
    String javaVersion = "";
    long startTime;

    private static Identity cached;

    static Identity get(RuntimeMXBean runtime) {
        Identity current = cached;
        if (current != null) {
            return current;
        }
        synchronized (Identity.class) {
            if (cached != null) {
                return cached;
            }
            Identity next = new Identity();
            next.processId = processId(runtime);
            next.mainClass = mainClass();
            next.javaVersion = System.getProperty("java.version", "");
            next.startTime = runtime.getStartTime();
            try {
                InetAddress local = InetAddress.getLocalHost();
                next.hostName = local.getHostName();
                next.ipAddress = local.getHostAddress();
            } catch (Exception ignored) {
                next.hostName = "";
                next.ipAddress = "";
            }
            cached = next;
            return next;
        }
    }

    private static String processId(RuntimeMXBean runtime) {
        try {
            Class<?> type = Class.forName("java.lang.ProcessHandle");
            Object current = type.getMethod("current").invoke(null);
            Object pid = type.getMethod("pid").invoke(current);
            return String.valueOf(pid);
        } catch (Throwable ignored) {
            String name = runtime.getName();
            int at = name.indexOf('@');
            if (at > 0) {
                return name.substring(0, at);
            }
            return name;
        }
    }

    private static String mainClass() {
        String command = System.getProperty("sun.java.command", "");
        if (command == null) {
            return "";
        }
        command = command.trim();
        int space = command.indexOf(' ');
        if (space > 0) {
            command = command.substring(0, space);
        }
        return command;
    }
}
