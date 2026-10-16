package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.zookeeper.CreateMode;
import org.apache.zookeeper.ZKUtil;
import org.apache.zookeeper.ZooDefs;
import org.apache.zookeeper.data.ACL;

import java.nio.charset.StandardCharsets;
import java.util.List;

final class Write {
    private static final int MAX_DATA = 1024 * 1024;

    private Write() {
    }

    static ObjectNode create(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            byte[] data = bytes(params);
            CreateMode mode = mode(Session.text(params.path("args"), "mode"));
            List<ACL> acl = Session.text(params, "user").isEmpty()
                    ? ZooDefs.Ids.OPEN_ACL_UNSAFE
                    : ZooDefs.Ids.CREATOR_ALL_ACL;
            String created = client.create(path, data, acl, mode);
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", created);
            return body;
        });
    }

    static ObjectNode set(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            byte[] data = bytes(params);
            var stat = client.setData(path, data, version(params));
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            Outcome.putStat(body, stat);
            return body;
        });
    }

    static ObjectNode delete(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            if ("/".equals(path)) {
                throw new IllegalArgumentException("不能删除根节点");
            }
            String recursive = Session.text(params.path("args"), "recursive");
            if (!recursive.isEmpty() && !"Y".equals(recursive)) {
                throw new IllegalArgumentException("删除范围无效");
            }
            if ("Y".equals(recursive)) {
                ZKUtil.deleteRecursive(client, path);
            } else {
                client.delete(path, version(params));
            }
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            body.put("deleted", true);
            return body;
        });
    }

    private static byte[] bytes(JsonNode params) {
        byte[] data = Session.text(params.path("args"), "data").getBytes(StandardCharsets.UTF_8);
        if (data.length > MAX_DATA) {
            throw new IllegalArgumentException("节点数据超过 1MB");
        }
        return data;
    }

    private static int version(JsonNode params) {
        String raw = Session.text(params.path("args"), "version");
        if (raw.isEmpty()) {
            return -1;
        }
        try {
            return Integer.parseInt(raw);
        } catch (NumberFormatException err) {
            throw new IllegalArgumentException("版本无效");
        }
    }

    private static CreateMode mode(String raw) {
        return switch (raw) {
            case "", "persistent" -> CreateMode.PERSISTENT;
            case "ephemeral" -> CreateMode.EPHEMERAL;
            case "persistent_sequential" -> CreateMode.PERSISTENT_SEQUENTIAL;
            case "ephemeral_sequential" -> CreateMode.EPHEMERAL_SEQUENTIAL;
            default -> throw new IllegalArgumentException("节点类型无效");
        };
    }
}
