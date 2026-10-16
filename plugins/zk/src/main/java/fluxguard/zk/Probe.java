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
            String host = address.split(",")[0];
            String text = Mntr.command(host, "mntr");
            if (!closed(text)) {
                Sample sample = readSample(text);
                if (known(sample)) {
                    sample.source = "mntr";
                    apply(result, values, sample);
                    return result;
                }
            }
            Sample srvr = readSrvr(Mntr.command(host, "srvr"));
            if (known(srvr)) {
                srvr.source = "srvr";
                apply(result, values, srvr);
                return result;
            }
            result.put("error", closed(text) ? "ZooKeeper 未打开 mntr" : "mntr 应答无法识别");
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
                case "min_latency" -> sample.minLatency = parseDouble(val);
                case "max_latency" -> sample.maxLatency = parseDouble(val);
                case "outstanding_requests" -> sample.outstanding = parseLong(val);
                case "znode_count" -> sample.znodes = parseLong(val);
                case "followers" -> sample.followers = parseLong(val);
                case "watch_count" -> sample.watches = parseLong(val);
                case "num_alive_connections" -> sample.connections = parseLong(val);
                case "packets_received" -> sample.received = parseLong(val);
                case "packets_sent" -> sample.sent = parseLong(val);
                case "zxid" -> sample.zxid = val;
                case "version" -> sample.version = val;
                case "server_state" -> sample.role = val;
                default -> {
                }
            }
        }
        return sample;
    }

    private static Sample readSrvr(String text) {
        Sample sample = new Sample();
        for (String line : text.split("\n")) {
            line = line.trim();
            int colon = line.indexOf(':');
            if (colon <= 0) {
                continue;
            }
            String key = line.substring(0, colon).trim().toLowerCase();
            String val = line.substring(colon + 1).trim();
            sample.seen = true;
            switch (key) {
                case "zookeeper version" -> sample.version = versionOf(val);
                case "latency min/avg/max" -> latencyOf(sample, val);
                case "received" -> sample.received = parseLong(val);
                case "sent" -> sample.sent = parseLong(val);
                case "connections" -> sample.connections = parseLong(val);
                case "outstanding" -> sample.outstanding = parseLong(val);
                case "zxid" -> sample.zxid = val;
                case "mode" -> sample.role = val.toLowerCase();
                case "node count" -> sample.znodes = parseLong(val);
                default -> {
                }
            }
        }
        return sample;
    }

    private static boolean known(Sample sample) {
        return sample.seen && (!sample.role.isEmpty() || sample.znodes != 0 || !sample.version.isEmpty());
    }

    private static String versionOf(String val) {
        int comma = val.indexOf(',');
        if (comma >= 0) {
            return val.substring(0, comma).trim();
        }
        return val;
    }

    private static void latencyOf(Sample sample, String val) {
        String[] parts = val.split("/");
        if (parts.length != 3) {
            sample.latency = parseDouble(val);
            return;
        }
        sample.minLatency = parseDouble(parts[0]);
        sample.latency = parseDouble(parts[1]);
        sample.maxLatency = parseDouble(parts[2]);
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
        ObjectNode detail = result.putObject("detail");
        if (!sample.source.isEmpty()) {
            detail.put("source", sample.source);
        }
        detail.put("minLatency", sample.minLatency);
        detail.put("maxLatency", sample.maxLatency);
        detail.put("connections", sample.connections);
        detail.put("received", sample.received);
        detail.put("sent", sample.sent);
        if (!sample.zxid.isEmpty()) {
            detail.put("zxid", sample.zxid);
        }
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
        private double minLatency;
        private double maxLatency;
        private long outstanding;
        private long znodes;
        private long followers;
        private long watches;
        private long connections;
        private long received;
        private long sent;
        private String version = "";
        private String role = "";
        private String zxid = "";
        private String source = "";
    }
}
