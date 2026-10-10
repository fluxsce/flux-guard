package fluxguard.kafka;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.charset.StandardCharsets;

final class Session {
    static final ObjectMapper JSON = new ObjectMapper();

    private Session() {
    }

    static void serve(BytePipe pipe) {
        while (true) {
            byte[] raw;
            try {
                raw = Frame.read(pipe);
            } catch (Exception end) {
                return;
            }
            ObjectNode reply = JSON.createObjectNode();
            reply.put("v", 1);
            reply.put("ok", false);
            reply.put("result", "");
            try {
                JsonNode call = JSON.readTree(raw);
                String id = text(call, "id");
                String traceId = text(call, "traceId");
                reply.put("id", id);
                reply.put("traceId", traceId);
                if (call.path("v").asInt() != 1 || id.isEmpty() || !id.equals(traceId)) {
                    fail(reply, "invalid_request", "插件应答无效");
                } else if ("probe".equals(text(call, "method"))) {
                    reply.put("ok", true);
                    reply.put("result", JSON.writeValueAsString(Probe.collect(call.path("params"))));
                } else if ("maintain".equals(text(call, "method"))) {
                    reply.put("ok", true);
                    reply.put("result", JSON.writeValueAsString(Maintain.run(call.path("params"))));
                } else {
                    fail(reply, "method_not_found", "这种组件没有这项操作");
                }
            } catch (Exception err) {
                fail(reply, "internal", "插件应答无效");
            }
            try {
                Frame.write(pipe, JSON.writeValueAsString(reply).getBytes(StandardCharsets.UTF_8));
            } catch (Exception end) {
                return;
            }
        }
    }

    private static void fail(ObjectNode reply, String code, String message) {
        reply.put("ok", false);
        reply.put("error", message);
        reply.put("errorCode", code);
        reply.put("result", "");
    }

    static String text(JsonNode node, String field) {
        JsonNode value = node.path(field);
        return value.isMissingNode() || value.isNull() ? "" : value.asText("");
    }
}
