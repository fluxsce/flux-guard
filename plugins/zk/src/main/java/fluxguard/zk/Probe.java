package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

final class Probe {
    private Probe() {
    }

    static ObjectNode collect(JsonNode params) {
        ObjectNode result = Session.JSON.createObjectNode();
        result.put("up", false);
        ObjectNode values = result.putObject("values");
        String secret = Session.text(params, "secret");
        String address = Address.ensemble(Session.text(params, "address"));
        if (address.isEmpty()) {
            result.put("error", "请填写地址");
            return result;
        }
        try {
            String text = Mntr.read(address.split(",")[0]);
            if (closed(text)) {
                result.put("error", "ZooKeeper 未打开 mntr");
                return result;
            }
            apply(result, values, readSample(text));
        } catch (Exception err) {
            result.put("up", false);
            result.put("error", Outcome.redact(err.getMessage(), secret));
        }
        return result;
    }

    private static boolean closed(String text) {
        return text.contains("not executed") || text.trim().startsWith("mntr is not");
    }

    private static Sample readSample(String text) {
        Sample sample = new Sample();
        for (String line : text.split("\n")) {
            line = line.trim();
            int tab = line.indexOf('\t');
            if (tab <= 0) {
                continue;
            }
            String key = line.substring(0, tab);
            if (key.startsWith("zk_")) {
                key = key.substring(3);
            }
            String val = line.substring(tab + 1).trim();
            sample.seen = true;
            switch (key) {
                case "avg_latency" -> sample.latency = parseDouble(val);
                case "outstanding_requests" -> sample.outstanding = parseLong(val);
                case "znode_count" -> sample.znodes = parseLong(val);
                case "followers" -> sample.followers = parseLong(val);
                case "watch_count" -> sample.watches = parseLong(val);
                case "version" -> sample.version = val;
                case "server_state" -> sample.role = val;
                default -> {
                }
            }
        }
        return sample;
    }

    private static void apply(ObjectNode result, ObjectNode values, Sample sample) {
        if (!sample.seen || (sample.role.isEmpty() && sample.znodes == 0 && sample.version.isEmpty())) {
            result.put("error", "mntr 应答无法识别");
            return;
        }
        values.put("avgLatency", sample.latency);
        values.put("outstandingRequests", sample.outstanding);
        values.put("znodeCount", sample.znodes);
        values.put("followers", sample.followers);
        values.put("watchCount", sample.watches);
        result.put("up", true);
        result.put("version", sample.version);
        result.put("role", sample.role);
        result.put("summary", String.format("角色 %s · 节点 %d · 延迟 %.0fms", sample.role, sample.znodes, sample.latency));
    }

    private static double parseDouble(String raw) {
        try {
            return Double.parseDouble(raw);
        } catch (NumberFormatException err) {
            return 0;
        }
    }

    private static long parseLong(String raw) {
        try {
            return Long.parseLong(raw);
        } catch (NumberFormatException err) {
            return 0;
        }
    }

    private static final class Sample {
        private boolean seen;
        private double latency;
        private long outstanding;
        private long znodes;
        private long followers;
        private long watches;
        private String version = "";
        private String role = "";
    }
}
