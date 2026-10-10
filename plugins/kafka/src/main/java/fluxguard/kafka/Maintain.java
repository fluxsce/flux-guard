package fluxguard.kafka;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.NewTopic;

import java.util.List;
import java.util.Optional;

final class Maintain {
    private Maintain() {
    }

    static ObjectNode run(JsonNode params) {
        ObjectNode result = Session.JSON.createObjectNode();
        String op = Session.text(params, "op");
        String secret = Session.text(params, "secret");
        JsonNode args = params.path("args");
        try (AdminClient admin = Clients.open(params)) {
            switch (op) {
                case "create_topic" -> createTopic(admin, args);
                case "delete_topic" -> deleteTopic(admin, args);
                default -> {
                    result.put("error", "这种组件没有这项操作");
                    return result;
                }
            }
            result.put("output", op);
        } catch (Exception err) {
            result.put("error", Clients.redact(err.getMessage(), secret));
        }
        return result;
    }

    private static void createTopic(AdminClient admin, JsonNode args) throws Exception {
        String name = args.path("name").asText("");
        if (name.isEmpty()) {
            throw new IllegalArgumentException("请填写主题名");
        }
        Optional<Integer> partitions = args.path("partitions").asText("").isEmpty()
                ? Optional.empty()
                : Optional.of(Integer.parseInt(args.path("partitions").asText()));
        Optional<Short> replication = args.path("replication_factor").asText("").isEmpty()
                ? Optional.empty()
                : Optional.of(Short.parseShort(args.path("replication_factor").asText()));
        admin.createTopics(List.of(new NewTopic(name, partitions, replication))).all().get();
    }

    private static void deleteTopic(AdminClient admin, JsonNode args) throws Exception {
        String name = args.path("name").asText("");
        if (name.isEmpty()) {
            throw new IllegalArgumentException("请填写主题名");
        }
        admin.deleteTopics(List.of(name)).all().get();
    }
}
