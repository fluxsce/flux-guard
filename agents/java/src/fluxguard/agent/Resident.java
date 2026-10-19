package fluxguard.agent;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.concurrent.TimeUnit;

// Resident 读取本进程常驻内存（RES）。读不到返回 -1，调用方不要把它写成 0。
// Linux 读 /proc。Windows 用 tasklist 的工作集，其它系统用 ps 的 rss。后两者才起子进程。
final class Resident {
    private Resident() {
    }

    static long bytes(String processId) {
        long fromProc = procStatus();
        if (fromProc != 0) {
            return fromProc;
        }
        if (!digits(processId)) {
            return -1;
        }
        String os = System.getProperty("os.name", "");
        if (os != null && os.toLowerCase().contains("win")) {
            return tasklist(processId);
        }
        return ps(processId);
    }

    // 0 表示这台机器没有 /proc，可以改走命令。-1 表示有 /proc 但这次没读到。
    private static long procStatus() {
        Path path = Paths.get("/proc/self/status");
        if (!Files.isRegularFile(path)) {
            return 0;
        }
        try (java.io.BufferedReader reader = Files.newBufferedReader(path, StandardCharsets.UTF_8)) {
            String line;
            while ((line = reader.readLine()) != null) {
                if (!line.startsWith("VmRSS:")) {
                    continue;
                }
                long kb = leadingLong(line.substring("VmRSS:".length()).trim());
                return kb > 0 ? kb * 1024L : -1;
            }
        } catch (IOException ignored) {
            return -1;
        }
        return -1;
    }

    private static long tasklist(String processId) {
        String text = command("tasklist", "/NH", "/FO", "CSV", "/FI", "PID eq " + processId);
        if (text == null || text.isEmpty()) {
            return -1;
        }
        long best = -1;
        int from = 0;
        while (from < text.length()) {
            int end = text.indexOf('\n', from);
            if (end < 0) {
                end = text.length();
            }
            String line = text.substring(from, end).trim();
            from = end + 1;
            if (line.isEmpty() || !line.contains(processId)) {
                continue;
            }
            String field = lastCsvField(line);
            if (field == null) {
                continue;
            }
            long bytes = kiloField(field);
            if (bytes > best) {
                best = bytes;
            }
        }
        return best;
    }

    private static long ps(String processId) {
        String text = command("ps", "-p", processId, "-o", "rss=");
        if (text == null) {
            return -1;
        }
        long kb = leadingLong(text.trim());
        return kb > 0 ? kb * 1024L : -1;
    }

    private static String command(String... args) {
        Process proc = null;
        try {
            proc = new ProcessBuilder(args).redirectErrorStream(true).start();
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            byte[] buf = new byte[256];
            InputStream in = proc.getInputStream();
            long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2);
            while (System.nanoTime() < deadline && out.size() < 4096) {
                if (in.available() > 0) {
                    int n = in.read(buf, 0, Math.min(buf.length, 4096 - out.size()));
                    if (n < 0) {
                        break;
                    }
                    if (n > 0) {
                        out.write(buf, 0, n);
                    }
                    continue;
                }
                try {
                    proc.exitValue();
                    int n;
                    while (out.size() < 4096 && (n = in.read(buf, 0, Math.min(buf.length, 4096 - out.size()))) > 0) {
                        out.write(buf, 0, n);
                    }
                    break;
                } catch (IllegalThreadStateException still) {
                    Thread.sleep(30);
                }
            }
            if (proc.isAlive()) {
                return null;
            }
            return out.toString(StandardCharsets.UTF_8.name());
        } catch (Exception ignored) {
            return null;
        } finally {
            if (proc != null) {
                try {
                    proc.getInputStream().close();
                } catch (IOException ignored) {
                }
                if (proc.isAlive()) {
                    proc.destroyForcibly();
                }
            }
        }
    }

    private static boolean digits(String processId) {
        if (processId == null || processId.isEmpty() || processId.length() > 20) {
            return false;
        }
        for (int i = 0; i < processId.length(); i++) {
            char c = processId.charAt(i);
            if (c < '0' || c > '9') {
                return false;
            }
        }
        return true;
    }

    private static long leadingLong(String text) {
        int i = 0;
        while (i < text.length() && text.charAt(i) == ' ') {
            i++;
        }
        int start = i;
        while (i < text.length() && text.charAt(i) >= '0' && text.charAt(i) <= '9') {
            i++;
        }
        if (i == start) {
            return -1;
        }
        try {
            return Long.parseLong(text.substring(start, i));
        } catch (NumberFormatException e) {
            return -1;
        }
    }

    // tasklist 最后一列是工作集，形如 54,321 K。没有 K 就不换算，避免把别的数字当成 KB。
    private static long kiloField(String field) {
        String text = field.trim();
        if (text.isEmpty()) {
            return -1;
        }
        char tail = text.charAt(text.length() - 1);
        if (tail != 'K' && tail != 'k') {
            return -1;
        }
        text = text.substring(0, text.length() - 1);
        StringBuilder digits = new StringBuilder();
        for (int i = 0; i < text.length(); i++) {
            char c = text.charAt(i);
            if (c >= '0' && c <= '9') {
                digits.append(c);
            }
        }
        if (digits.length() == 0 || digits.length() > 15) {
            return -1;
        }
        try {
            long kb = Long.parseLong(digits.toString());
            return kb > 0 ? kb * 1024L : -1;
        } catch (NumberFormatException e) {
            return -1;
        }
    }

    private static String lastCsvField(String line) {
        StringBuilder cur = new StringBuilder();
        boolean quote = false;
        boolean any = false;
        for (int i = 0; i < line.length(); i++) {
            char c = line.charAt(i);
            if (c == '"') {
                quote = !quote;
                any = true;
                continue;
            }
            if (c == ',' && !quote) {
                cur.setLength(0);
                any = true;
                continue;
            }
            cur.append(c);
            any = true;
        }
        if (!any) {
            return null;
        }
        return cur.toString();
    }
}
