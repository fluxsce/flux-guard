package fluxguard.kafka;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.AlterConfigOp;
import org.apache.kafka.clients.admin.Config;
import org.apache.kafka.clients.admin.ConfigEntry;
import org.apache.kafka.clients.admin.ListOffsetsResult;
import org.apache.kafka.clients.admin.ListTopicsOptions;
import org.apache.kafka.clients.admin.NewPartitions;
import org.apache.kafka.clients.admin.NewTopic;
import org.apache.kafka.clients.admin.OffsetSpec;
import org.apache.kafka.clients.admin.TopicDescription;
import org.apache.kafka.clients.admin.TopicListing;
import org.apache.kafka.clients.consumer.OffsetAndMetadata;
import org.apache.kafka.common.TopicPartition;
import org.apache.kafka.common.TopicPartitionInfo;
import org.apache.kafka.common.config.ConfigResource;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.TimeUnit;

final class Maintain {
    private static final int CAP = 200;
    private static final Set<String> CONFIG_KEYS = Set.of(
            "retention.ms", "retention.bytes", "cleanup.policy", "min.insync.replicas", "max.message.bytes");

    private Maintain() {
    }

    static ObjectNode run(JsonNode params) {
        ObjectNode result = Session.JSON.createObjectNode();
        String op = Session.text(params, "op");
        String secret = Session.text(params, "secret");
        JsonNode args = params.path("args");
        try (AdminClient admin = Clients.open(params)) {
            switch (op) {
                case "topics" -> put(result, topics(admin));
                case "topic_config" -> put(result, topicConfig(admin, name(args)));
                case "create_topic" -> {
                    createTopic(admin, args);
                    ack(result);
                }
                case "delete_topic" -> {
                    deleteTopic(admin, name(args));
                    ack(result);
                }
                case "add_partitions" -> {
                    addPartitions(admin, args);
                    ack(result);
                }
                case "alter_config" -> {
                    alterConfig(admin, args);
                    ack(result);
                }
                case "groups" -> put(result, groups(admin));
                case "delete_group" -> {
                    deleteGroup(admin, name(args));
                    ack(result);
                }
                case "reset_offsets" -> {
                    resetOffsets(admin, args);
                    ack(result);
                }
                default -> result.put("error", "这种组件没有这项操作");
            }
        } catch (Exception err) {
            result.put("error", Clients.redact(err.getMessage(), secret));
        }
        return result;
    }

    private static void put(ObjectNode result, ObjectNode body) throws Exception {
        result.put("output", Session.JSON.writeValueAsString(body));
    }

    private static void ack(ObjectNode result) {
        result.put("output", "{\"ok\":true}");
    }

    private static ObjectNode topics(AdminClient admin) throws Exception {
        List<TopicListing> listings = new ArrayList<>(admin.listTopics(new ListTopicsOptions().listInternal(true)).listings().get(20, TimeUnit.SECONDS));
        listings.sort(Comparator.comparing(TopicListing::name));
        String note = "";
        if (listings.size() > CAP) {
            note = "主题很多，这里只列出前 " + CAP + " 个。";
            listings = listings.subList(0, CAP);
        }
        List<String> names = new ArrayList<>();
        for (TopicListing listing : listings) {
            names.add(listing.name());
        }
        Map<String, TopicDescription> described = names.isEmpty()
                ? Map.of()
                : admin.describeTopics(names).allTopicNames().get(20, TimeUnit.SECONDS);
        ArrayNode rows = Session.JSON.createArrayNode();
        for (TopicListing listing : listings) {
            TopicDescription topic = described.get(listing.name());
            int parts = topic == null ? 0 : topic.partitions().size();
            int replicas = 0;
            if (topic != null && !topic.partitions().isEmpty()) {
                replicas = topic.partitions().get(0).replicas().size();
            }
            ObjectNode row = rows.addObject();
            row.put("name", listing.name());
            row.put("partitions", parts);
            row.put("replicas", replicas);
            if (listing.isInternal()) {
                row.put("internal", true);
            }
        }
        ObjectNode body = Session.JSON.createObjectNode();
        body.set("topics", rows);
        if (!note.isEmpty()) {
            body.put("note", note);
        }
        return body;
    }

    private static ObjectNode topicConfig(AdminClient admin, String name) throws Exception {
        requireTopic(name);
        ConfigResource resource = new ConfigResource(ConfigResource.Type.TOPIC, name);
        Config config = admin.describeConfigs(List.of(resource)).all().get(20, TimeUnit.SECONDS).get(resource);
        ObjectNode body = Session.JSON.createObjectNode();
        body.put("name", name);
        body.put("retentionMs", entry(config, "retention.ms"));
        body.put("retentionBytes", entry(config, "retention.bytes"));
        body.put("cleanup", entry(config, "cleanup.policy"));
        body.put("minIsr", entry(config, "min.insync.replicas"));
        body.put("maxMessageBytes", entry(config, "max.message.bytes"));
        return body;
    }

    private static String entry(Config config, String key) {
        ConfigEntry item = config.get(key);
        if (item == null || item.value() == null) {
            return "";
        }
        return item.value();
    }

    private static void createTopic(AdminClient admin, JsonNode args) throws Exception {
        String topic = name(args);
        requireTopic(topic);
        Optional<Integer> partitions = optionalInt(args, "partitions", 1, 1024, "分区数要在 1 到 1024 之间");
        Optional<Short> replication = optionalShort(args, "replication_factor", 1, 20, "副本因子要在 1 到 20 之间");
        admin.createTopics(List.of(new NewTopic(topic, partitions, replication))).all().get(20, TimeUnit.SECONDS);
    }

    private static void deleteTopic(AdminClient admin, String name) throws Exception {
        requireTopic(name);
        if (name.startsWith("__")) {
            throw new IllegalArgumentException("不能删除内部主题");
        }
        admin.deleteTopics(List.of(name)).all().get(20, TimeUnit.SECONDS);
    }

    private static void addPartitions(AdminClient admin, JsonNode args) throws Exception {
        String topic = name(args);
        requireTopic(topic);
        if (topic.startsWith("__")) {
            throw new IllegalArgumentException("不能修改内部主题");
        }
        int total = requiredInt(args, "partitions", 1, 1024, "分区数要在 1 到 1024 之间");
        admin.createPartitions(Map.of(topic, NewPartitions.increaseTo(total))).all().get(20, TimeUnit.SECONDS);
    }

    private static void alterConfig(AdminClient admin, JsonNode args) throws Exception {
        String topic = name(args);
        requireTopic(topic);
        if (topic.startsWith("__")) {
            throw new IllegalArgumentException("不能修改内部主题");
        }
        String key = Session.text(args, "key");
        String value = Session.text(args, "value");
        if (!CONFIG_KEYS.contains(key)) {
            throw new IllegalArgumentException("不能修改这项配置");
        }
        checkConfig(key, value);
        ConfigResource resource = new ConfigResource(ConfigResource.Type.TOPIC, topic);
        AlterConfigOp op = new AlterConfigOp(new ConfigEntry(key, value), AlterConfigOp.OpType.SET);
        admin.incrementalAlterConfigs(Map.of(resource, List.of(op))).all().get(20, TimeUnit.SECONDS);
    }

    private static void checkConfig(String key, String value) {
        switch (key) {
            case "cleanup.policy" -> {
                if (!value.equals("delete") && !value.equals("compact") && !value.equals("delete,compact")) {
                    throw new IllegalArgumentException("清理策略只能是 delete、compact 或 delete,compact");
                }
            }
            case "retention.ms", "retention.bytes" -> numberAtLeast(value, -1, "保留量无效");
            case "min.insync.replicas" -> numberAtLeast(value, 1, "最小同步副本无效");
            case "max.message.bytes" -> numberAtLeast(value, 1, "单条大小无效");
            default -> throw new IllegalArgumentException("不能修改这项配置");
        }
    }

    private static ObjectNode groups(AdminClient admin) throws Exception {
        var listings = new ArrayList<>(admin.listConsumerGroups().all().get(20, TimeUnit.SECONDS));
        listings.sort(Comparator.comparing(item -> item.groupId()));
        String note = "";
        if (listings.size() > CAP) {
            note = "消费组很多，这里只列出前 " + CAP + " 个。";
            listings = new ArrayList<>(listings.subList(0, CAP));
        }
        ArrayNode rows = Session.JSON.createArrayNode();
        for (var listing : listings) {
            ObjectNode row = rows.addObject();
            row.put("name", listing.groupId());
            listing.state().ifPresent(state -> row.put("state", state.toString()));
        }
        ObjectNode body = Session.JSON.createObjectNode();
        body.set("groups", rows);
        if (!note.isEmpty()) {
            body.put("note", note);
        }
        return body;
    }

    private static void deleteGroup(AdminClient admin, String name) throws Exception {
        requireGroup(name);
        admin.deleteConsumerGroups(List.of(name)).all().get(20, TimeUnit.SECONDS);
    }

    private static void resetOffsets(AdminClient admin, JsonNode args) throws Exception {
        String group = name(args);
        requireGroup(group);
        String topic = Session.text(args, "topic");
        requireTopic(topic);
        String to = Session.text(args, "to");
        OffsetSpec spec = switch (to) {
            case "earliest" -> OffsetSpec.earliest();
            case "latest" -> OffsetSpec.latest();
            default -> throw new IllegalArgumentException("位点只能重置到最早或最新");
        };
        TopicDescription described = admin.describeTopics(List.of(topic)).allTopicNames().get(20, TimeUnit.SECONDS).get(topic);
        if (described == null || described.partitions().isEmpty()) {
            throw new IllegalArgumentException("这个主题没有分区");
        }
        Map<TopicPartition, OffsetSpec> request = new LinkedHashMap<>();
        for (TopicPartitionInfo part : described.partitions()) {
            request.put(new TopicPartition(topic, part.partition()), spec);
        }
        ListOffsetsResult listed = admin.listOffsets(request);
        Map<TopicPartition, OffsetAndMetadata> offsets = new HashMap<>();
        for (TopicPartition part : request.keySet()) {
            long offset = listed.partitionResult(part).get(20, TimeUnit.SECONDS).offset();
            offsets.put(part, new OffsetAndMetadata(offset));
        }
        admin.alterConsumerGroupOffsets(group, offsets).all().get(20, TimeUnit.SECONDS);
    }

    private static String name(JsonNode args) {
        return Session.text(args, "name").trim();
    }

    private static void requireTopic(String name) {
        if (!validName(name)) {
            throw new IllegalArgumentException("主题名无效");
        }
    }

    private static void requireGroup(String name) {
        if (!validName(name)) {
            throw new IllegalArgumentException("消费组名无效");
        }
    }

    private static boolean validName(String name) {
        if (name.isEmpty() || name.length() > 200 || ".".equals(name) || "..".equals(name)) {
            return false;
        }
        for (int i = 0; i < name.length(); i++) {
            char c = name.charAt(i);
            boolean ok = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-';
            if (!ok) {
                return false;
            }
        }
        return true;
    }

    private static Optional<Integer> optionalInt(JsonNode args, String key, int min, int max, String message) {
        String text = Session.text(args, key).trim();
        if (text.isEmpty()) {
            return Optional.empty();
        }
        return Optional.of(parseInt(text, min, max, message));
    }

    private static Optional<Short> optionalShort(JsonNode args, String key, int min, int max, String message) {
        String text = Session.text(args, key).trim();
        if (text.isEmpty()) {
            return Optional.empty();
        }
        return Optional.of((short) parseInt(text, min, max, message));
    }

    private static int requiredInt(JsonNode args, String key, int min, int max, String message) {
        String text = Session.text(args, key).trim();
        if (text.isEmpty()) {
            throw new IllegalArgumentException(message);
        }
        return parseInt(text, min, max, message);
    }

    private static int parseInt(String text, int min, int max, String message) {
        try {
            int value = Integer.parseInt(text);
            if (value < min || value > max) {
                throw new IllegalArgumentException(message);
            }
            return value;
        } catch (NumberFormatException err) {
            throw new IllegalArgumentException(message);
        }
    }

    private static void numberAtLeast(String text, long min, String message) {
        try {
            long value = Long.parseLong(text);
            if (value < min) {
                throw new IllegalArgumentException(message);
            }
        } catch (NumberFormatException err) {
            throw new IllegalArgumentException(message);
        }
    }
}
