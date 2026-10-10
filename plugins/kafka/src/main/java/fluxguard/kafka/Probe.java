package fluxguard.kafka;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.Config;
import org.apache.kafka.clients.admin.ConsumerGroupDescription;
import org.apache.kafka.clients.admin.ConsumerGroupListing;
import org.apache.kafka.clients.admin.DescribeLogDirsResult;
import org.apache.kafka.clients.admin.ListConsumerGroupOffsetsSpec;
import org.apache.kafka.clients.admin.ListOffsetsResult;
import org.apache.kafka.clients.admin.ListTopicsOptions;
import org.apache.kafka.clients.admin.OffsetSpec;
import org.apache.kafka.clients.admin.TopicDescription;
import org.apache.kafka.clients.consumer.OffsetAndMetadata;
import org.apache.kafka.common.Node;
import org.apache.kafka.common.TopicPartition;
import org.apache.kafka.common.TopicPartitionInfo;
import org.apache.kafka.common.config.ConfigResource;

import java.util.ArrayList;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

final class Probe {
    private static final int TOPIC_CAP = 80;
    private static final int GROUP_CAP = 80;
    private static final int PART_CAP = 40;

    private Probe() {
    }

    static ObjectNode collect(JsonNode params) {
        ObjectNode result = Session.JSON.createObjectNode();
        result.put("up", false);
        result.put("role", "cluster");
        ObjectNode values = result.putObject("values");
        String secret = Session.text(params, "secret");
        if (Clients.servers(Session.text(params, "address")).isEmpty()) {
            result.put("error", "请填写地址");
            return result;
        }
        try (AdminClient admin = Clients.open(params)) {
            fill(admin, result, values);
        } catch (Exception err) {
            result.put("up", false);
            result.put("error", Clients.redact(err.getMessage(), secret));
        }
        return result;
    }

    private static void fill(AdminClient admin, ObjectNode result, ObjectNode values) throws Exception {
        var cluster = admin.describeCluster();
        String clusterId = cluster.clusterId().get(20, TimeUnit.SECONDS);
        Collection<Node> nodes = cluster.nodes().get(20, TimeUnit.SECONDS);
        Node controller = cluster.controller().get(20, TimeUnit.SECONDS);
        int controllerId = controller == null ? -1 : controller.id();

        List<String> topicNames = new ArrayList<>(admin.listTopics(new ListTopicsOptions().listInternal(true)).names().get(20, TimeUnit.SECONDS));
        topicNames.sort(String::compareTo);
        Map<String, TopicDescription> topics = topicNames.isEmpty()
                ? Map.of()
                : admin.describeTopics(topicNames).allTopicNames().get(20, TimeUnit.SECONDS);

        int minIsr = brokerMinIsr(admin, controllerId);
        String version = brokerVersion(admin, controllerId);
        Map<String, Integer> topicMin = topicMinIsr(admin, topicNames, minIsr);

        long offlinePartitions = 0;
        long under = 0;
        long skew = 0;
        long below = 0;
        Map<Integer, int[]> placement = new HashMap<>();
        List<ObjectNode> partRows = new ArrayList<>();
        List<ObjectNode> topicRows = new ArrayList<>();
        for (String name : topicNames) {
            TopicDescription topic = topics.get(name);
            if (topic == null) {
                continue;
            }
            int parts = topic.partitions().size();
            int offline = 0;
            int underCount = 0;
            int skewCount = 0;
            int belowCount = 0;
            int replicaMin = 0;
            int replicaMax = 0;
            int need = topicMin.getOrDefault(name, minIsr);
            for (TopicPartitionInfo part : topic.partitions()) {
                int replicas = part.replicas().size();
                int isr = part.isr().size();
                if (replicaMin == 0 || replicas < replicaMin) {
                    replicaMin = replicas;
                }
                if (replicas > replicaMax) {
                    replicaMax = replicas;
                }
                boolean leaderMissing = part.leader() == null || part.leader().id() < 0;
                boolean skewed = !leaderMissing && !part.replicas().isEmpty() && part.leader().id() != part.replicas().get(0).id();
                if (leaderMissing) {
                    offline++;
                }
                if (isr < replicas) {
                    underCount++;
                }
                if (skewed) {
                    skewCount++;
                }
                if (need > 0 && isr < need) {
                    belowCount++;
                }
                for (Node replica : part.replicas()) {
                    placement.computeIfAbsent(replica.id(), key -> new int[2])[1]++;
                }
                if (!leaderMissing) {
                    placement.computeIfAbsent(part.leader().id(), key -> new int[2])[0]++;
                }
                String problem = problemOf(leaderMissing, skewed, isr, replicas, need);
                if (!problem.isEmpty()) {
                    ObjectNode bad = Session.JSON.createObjectNode();
                    bad.put("topic", name);
                    bad.put("part", part.partition());
                    bad.put("leader", leaderMissing ? -1 : part.leader().id());
                    bad.put("isr", isr);
                    bad.put("replicas", replicas);
                    bad.put("problem", problem);
                    bad.put("rank", rankOf(problem));
                    partRows.add(bad);
                }
            }
            offlinePartitions += offline;
            under += underCount;
            skew += skewCount;
            below += belowCount;
            boolean keep = topic.isInternal() ? (offline + underCount + skewCount + belowCount) > 0 : true;
            if (!keep) {
                continue;
            }
            ObjectNode row = Session.JSON.createObjectNode();
            row.put("name", name);
            row.put("partitions", parts);
            row.put("offline", offline);
            row.put("under", underCount);
            row.put("skew", skewCount);
            row.put("replicas", replicaMin);
            if (replicaMax != replicaMin) {
                row.put("replicaMax", replicaMax);
            }
            if (belowCount > 0) {
                row.put("belowMin", belowCount);
            }
            if (topic.isInternal()) {
                row.put("internal", true);
            }
            topicRows.add(row);
        }
        topicRows.sort(Comparator.comparingInt((ObjectNode row) -> row.path("offline").asInt()).reversed()
                .thenComparing(Comparator.comparingInt((ObjectNode row) -> row.path("under").asInt()).reversed())
                .thenComparing(row -> row.path("name").asText()));

        List<ConsumerGroupListing> listings = new ArrayList<>(admin.listConsumerGroups().all().get(20, TimeUnit.SECONDS));
        listings.sort(Comparator.comparing(ConsumerGroupListing::groupId));
        List<String> groupIds = listings.stream().map(ConsumerGroupListing::groupId).limit(GROUP_CAP).toList();
        Map<String, ConsumerGroupDescription> described = groupIds.isEmpty()
                ? Map.of()
                : admin.describeConsumerGroups(groupIds).all().get(20, TimeUnit.SECONDS);

        long maxLag = -1;
        long totalLag = 0;
        boolean anyLag = false;
        long rebalance = 0;
        List<ObjectNode> groupRows = new ArrayList<>();
        for (String groupId : groupIds) {
            ConsumerGroupDescription group = described.get(groupId);
            if (group == null) {
                continue;
            }
            String state = groupState(group);
            if ("PreparingRebalance".equals(state) || "CompletingRebalance".equals(state)) {
                rebalance++;
            }
            Lag lag = lagOf(admin, groupId);
            if (lag.known) {
                anyLag = true;
                totalLag += lag.total;
                if (lag.max > maxLag) {
                    maxLag = lag.max;
                }
            }
            ObjectNode row = Session.JSON.createObjectNode();
            row.put("name", groupId);
            row.put("lag", lag.known ? lag.total : -1);
            row.put("maxLag", lag.known ? lag.max : -1);
            row.put("part", lag.part);
            row.put("topic", lag.topic);
            if (lag.topics > 1) {
                row.put("topicCount", lag.topics);
            }
            row.put("state", state);
            row.put("members", group.members().size());
            row.put("behind", lag.known ? lag.behind : -1);
            groupRows.add(row);
        }
        if (!anyLag) {
            maxLag = -1;
            totalLag = -1;
        }
        groupRows.sort(Comparator.comparingLong((ObjectNode row) -> row.path("maxLag").asLong()).reversed()
                .thenComparing(row -> row.path("name").asText()));

        Map<Integer, Long> logBytes = logBytes(admin, nodes);
        long logTotal = -1;
        int online = 0;
        ArrayNode brokers = Session.JSON.createArrayNode();
        List<Node> ordered = new ArrayList<>(nodes);
        ordered.sort(Comparator.comparingInt(Node::id));
        for (Node node : ordered) {
            boolean up = logBytes.containsKey(node.id()) || node.id() == controllerId;
            if (up) {
                online++;
            }
            long bytes = logBytes.getOrDefault(node.id(), -1L);
            if (bytes >= 0) {
                logTotal = logTotal < 0 ? bytes : logTotal + bytes;
            }
            ObjectNode row = brokers.addObject();
            row.put("id", node.id());
            row.put("host", node.host());
            row.put("port", node.port());
            row.put("controller", node.id() == controllerId);
            row.put("up", up);
            row.put("logBytes", bytes);
            int[] slot = placement.get(node.id());
            row.put("leaders", slot == null ? 0 : slot[0]);
            row.put("replicas", slot == null ? 0 : slot[1]);
        }

        values.put("brokerCount", nodes.size());
        values.put("onlineBrokers", online);
        values.put("offlineBrokers", Math.max(0, nodes.size() - online));
        values.put("offlinePartitions", offlinePartitions);
        values.put("underReplicated", under);
        values.put("preferredReplicas", skew);
        values.put("underMinIsr", below);
        values.put("topicCount", topicNames.size());
        values.put("groupCount", listings.size());
        values.put("rebalanceGroups", rebalance);
        values.put("maxLag", maxLag);
        values.put("totalLag", totalLag);
        values.put("logBytes", logTotal);

        ObjectNode detail = result.putObject("detail");
        if (clusterId != null && !clusterId.isEmpty()) {
            detail.put("clusterId", clusterId);
        }
        if (!version.isEmpty()) {
            result.put("version", version);
        }
        detail.set("brokers", brokers);
        String topicNote = "";
        if (topicRows.size() > TOPIC_CAP) {
            topicNote = "主题共 " + topicRows.size() + " 个，这里列出前 " + TOPIC_CAP + " 个，优先离线和未同步。";
            topicRows = topicRows.subList(0, TOPIC_CAP);
        }
        if (listings.size() > GROUP_CAP) {
            detail.put("groupNote", "消费组共 " + listings.size() + " 个，这里列出前 " + GROUP_CAP + " 个。");
        }
        if (!topicNote.isEmpty()) {
            detail.put("topicNote", topicNote);
        }
        ArrayNode topicJson = detail.putArray("topics");
        topicRows.forEach(topicJson::add);
        ArrayNode groupJson = detail.putArray("groups");
        groupRows.forEach(groupJson::add);
        partRows.sort(Comparator.comparingInt((ObjectNode row) -> row.path("rank").asInt()).reversed()
                .thenComparing(row -> row.path("topic").asText())
                .thenComparingInt(row -> row.path("part").asInt()));
        if (partRows.size() > PART_CAP) {
            detail.put("partitionNote", "不健康分区共 " + partRows.size() + " 个，这里列出前 " + PART_CAP + " 个，优先离线和未同步。");
            partRows = partRows.subList(0, PART_CAP);
        }
        ArrayNode partJson = detail.putArray("partitions");
        for (ObjectNode row : partRows) {
            row.remove("rank");
            partJson.add(row);
        }

        result.put("up", true);
        result.put("summary", "在线 " + online + "/" + nodes.size() + " · Topic " + topicNames.size() + " · 离线分区 " + offlinePartitions);
    }

    private static int brokerMinIsr(AdminClient admin, int controllerId) {
        if (controllerId < 0) {
            return 1;
        }
        try {
            ConfigResource resource = new ConfigResource(ConfigResource.Type.BROKER, Integer.toString(controllerId));
            Config config = admin.describeConfigs(List.of(resource)).all().get(10, TimeUnit.SECONDS).get(resource);
            String text = config.get("min.insync.replicas") == null ? "" : config.get("min.insync.replicas").value();
            int value = Integer.parseInt(text);
            return value > 0 ? value : 1;
        } catch (Exception ignored) {
            return 1;
        }
    }

    private static String brokerVersion(AdminClient admin, int controllerId) {
        if (controllerId < 0) {
            return "";
        }
        try {
            ConfigResource resource = new ConfigResource(ConfigResource.Type.BROKER, Integer.toString(controllerId));
            Config config = admin.describeConfigs(List.of(resource)).all().get(10, TimeUnit.SECONDS).get(resource);
            if (config.get("inter.broker.protocol.version") == null) {
                return "";
            }
            String text = config.get("inter.broker.protocol.version").value();
            return text == null ? "" : text;
        } catch (Exception ignored) {
            return "";
        }
    }

    private static Map<String, Integer> topicMinIsr(AdminClient admin, List<String> names, int fallback) {
        Map<String, Integer> out = new HashMap<>();
        List<String> slice = names.size() > 64 ? names.subList(0, 64) : names;
        if (slice.isEmpty()) {
            return out;
        }
        try {
            List<ConfigResource> resources = slice.stream().map(name -> new ConfigResource(ConfigResource.Type.TOPIC, name)).toList();
            Map<ConfigResource, Config> configs = admin.describeConfigs(resources).all().get(15, TimeUnit.SECONDS);
            for (Map.Entry<ConfigResource, Config> entry : configs.entrySet()) {
                var item = entry.getValue().get("min.insync.replicas");
                if (item == null || item.value() == null || item.value().isEmpty()) {
                    out.put(entry.getKey().name(), fallback);
                    continue;
                }
                out.put(entry.getKey().name(), Integer.parseInt(item.value()));
            }
        } catch (Exception ignored) {
            for (String name : slice) {
                out.put(name, fallback);
            }
        }
        return out;
    }

    private static Lag lagOf(AdminClient admin, String groupId) {
        Lag lag = new Lag();
        try {
            Map<TopicPartition, OffsetAndMetadata> committed = admin.listConsumerGroupOffsets(
                    Map.of(groupId, new ListConsumerGroupOffsetsSpec())).partitionsToOffsetAndMetadata(groupId).get(12, TimeUnit.SECONDS);
            if (committed.isEmpty()) {
                return lag;
            }
            Map<TopicPartition, OffsetSpec> spec = new LinkedHashMap<>();
            committed.keySet().forEach(part -> spec.put(part, OffsetSpec.latest()));
            Map<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> ends = admin.listOffsets(spec).all().get(12, TimeUnit.SECONDS);
            Map<String, Integer> topics = new HashMap<>();
            for (Map.Entry<TopicPartition, OffsetAndMetadata> entry : committed.entrySet()) {
                ListOffsetsResult.ListOffsetsResultInfo end = ends.get(entry.getKey());
                if (end == null || entry.getValue() == null || entry.getValue().offset() < 0) {
                    continue;
                }
                long diff = Math.max(0, end.offset() - entry.getValue().offset());
                lag.known = true;
                lag.total += diff;
                if (diff > 0) {
                    lag.behind++;
                }
                topics.merge(entry.getKey().topic(), 1, Integer::sum);
                if (diff >= lag.max) {
                    lag.max = diff;
                    lag.part = entry.getKey().partition();
                    lag.topic = entry.getKey().topic();
                }
            }
            lag.topics = topics.size();
        } catch (Exception ignored) {
            return new Lag();
        }
        return lag;
    }

    private static Map<Integer, Long> logBytes(AdminClient admin, Collection<Node> nodes) {
        Map<Integer, Long> out = new HashMap<>();
        if (nodes.isEmpty()) {
            return out;
        }
        try {
            List<Integer> ids = nodes.stream().map(Node::id).toList();
            DescribeLogDirsResult described = admin.describeLogDirs(ids);
            long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(12);
            for (Map.Entry<Integer, org.apache.kafka.common.KafkaFuture<Map<String, org.apache.kafka.clients.admin.LogDirDescription>>> entry : described.descriptions().entrySet()) {
                long left = deadline - System.nanoTime();
                if (left <= 0) {
                    break;
                }
                Map<String, org.apache.kafka.clients.admin.LogDirDescription> dirs = entry.getValue().get(left, TimeUnit.NANOSECONDS);
                long sum = 0;
                boolean any = false;
                for (var dir : dirs.values()) {
                    if (dir.error() != null) {
                        continue;
                    }
                    for (var replica : dir.replicaInfos().values()) {
                        sum += Math.max(0, replica.size());
                        any = true;
                    }
                }
                if (any) {
                    out.put(entry.getKey(), sum);
                }
            }
        } catch (Exception ignored) {
            return out;
        }
        return out;
    }

    private static String groupState(ConsumerGroupDescription group) {
        if (group.state() == null) {
            return "";
        }
        return switch (group.state().toString()) {
            case "STABLE" -> "Stable";
            case "EMPTY" -> "Empty";
            case "DEAD" -> "Dead";
            case "PREPARING_REBALANCE" -> "PreparingRebalance";
            case "COMPLETING_REBALANCE" -> "CompletingRebalance";
            default -> group.state().toString();
        };
    }

    private static String problemOf(boolean offline, boolean skewed, int isr, int replicas, int need) {
        if (offline) {
            return "离线";
        }
        if (need > 0 && isr < need) {
            return "低于最小 ISR";
        }
        if (isr < replicas) {
            return "未同步";
        }
        if (skewed) {
            return "首选偏离";
        }
        return "";
    }

    private static int rankOf(String problem) {
        return switch (problem) {
            case "离线" -> 4;
            case "低于最小 ISR" -> 3;
            case "未同步" -> 2;
            case "首选偏离" -> 1;
            default -> 0;
        };
    }

    private static final class Lag {
        boolean known;
        long total;
        long max = -1;
        int part = -1;
        String topic = "";
        int topics;
        int behind;
    }
}
