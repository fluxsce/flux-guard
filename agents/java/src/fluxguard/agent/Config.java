package fluxguard.agent;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.LinkedHashMap;
import java.util.Map;

// Config 读取上报地址、令牌和间隔。先读 YAML，再用代理参数覆盖。
final class Config {
    String serverUrl = "http://127.0.0.1:8080";
    String token = "agt_dev_local";
    int intervalSec = 15;
    String appName = "";
    String instanceName = "";
    boolean once;

    static Config load(String args) {
        Config cfg = new Config();
        Map<String, String> values = new LinkedHashMap<String, String>();
        String configPath = "configs/java.yaml";
        Map<String, String> fromArgs = parseArgs(args);
        if (fromArgs.containsKey("config")) {
            configPath = fromArgs.get("config");
        }
        values.putAll(readYaml(configPath));
        values.putAll(fromArgs);
        apply(cfg, values);
        if (cfg.serverUrl.endsWith("/")) {
            cfg.serverUrl = cfg.serverUrl.substring(0, cfg.serverUrl.length() - 1);
        }
        if (cfg.intervalSec <= 0) {
            cfg.intervalSec = 15;
        }
        return cfg;
    }

    private static void apply(Config cfg, Map<String, String> values) {
        String url = first(values, "serverUrl", "server_url");
        if (url != null && !url.isEmpty()) {
            cfg.serverUrl = url;
        }
        String token = first(values, "token", "agent_token");
        if (token != null && !token.isEmpty()) {
            cfg.token = token;
        }
        String interval = first(values, "intervalSec", "interval_sec");
        if (interval != null && !interval.isEmpty()) {
            try {
                cfg.intervalSec = Integer.parseInt(interval.trim());
            } catch (NumberFormatException ignored) {
                cfg.intervalSec = 15;
            }
        }
        String app = first(values, "appName", "app_name");
        if (app != null) {
            cfg.appName = app.trim();
        }
        String instance = first(values, "instanceName", "instance_name");
        if (instance != null) {
            cfg.instanceName = instance.trim();
        }
        String once = first(values, "once");
        cfg.once = "true".equalsIgnoreCase(once) || "1".equals(once);
    }

    private static String first(Map<String, String> values, String... keys) {
        for (String key : keys) {
            if (values.containsKey(key)) {
                return values.get(key);
            }
        }
        return null;
    }

    static Map<String, String> parseArgs(String raw) {
        Map<String, String> out = new LinkedHashMap<String, String>();
        if (raw == null || raw.trim().isEmpty()) {
            return out;
        }
        String[] parts = raw.split(";");
        for (String part : parts) {
            String item = part.trim();
            if (item.isEmpty()) {
                continue;
            }
            int eq = item.indexOf('=');
            if (eq <= 0) {
                continue;
            }
            out.put(item.substring(0, eq).trim(), item.substring(eq + 1).trim());
        }
        return out;
    }

    private static Map<String, String> readYaml(String path) {
        Map<String, String> out = new LinkedHashMap<String, String>();
        Path file = Paths.get(path);
        if (!Files.isRegularFile(file)) {
            return out;
        }
        try (BufferedReader reader = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = reader.readLine()) != null) {
                String text = stripComment(line).trim();
                if (text.isEmpty() || text.startsWith("-") || text.endsWith(":")) {
                    continue;
                }
                int colon = text.indexOf(':');
                if (colon <= 0) {
                    continue;
                }
                String key = text.substring(0, colon).trim();
                String value = text.substring(colon + 1).trim();
                if (value.startsWith("\"") && value.endsWith("\"") && value.length() >= 2) {
                    value = value.substring(1, value.length() - 1);
                }
                if (!key.isEmpty()) {
                    out.put(key, value);
                }
            }
        } catch (IOException ignored) {
            return out;
        }
        return out;
    }

    private static String stripComment(String line) {
        boolean quote = false;
        for (int i = 0; i < line.length(); i++) {
            char c = line.charAt(i);
            if (c == '"') {
                quote = !quote;
            } else if (c == '#' && !quote) {
                return line.substring(0, i);
            }
        }
        return line;
    }
}
