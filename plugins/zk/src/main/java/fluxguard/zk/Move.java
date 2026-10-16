package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.zookeeper.CreateMode;
import org.apache.zookeeper.ZKUtil;
import org.apache.zookeeper.ZooKeeper;
import org.apache.zookeeper.data.ACL;
import org.apache.zookeeper.data.Stat;

import java.util.List;

final class Move {
    private static final int MAX_NODES = 10000;
    private static final int MAX_DEPTH = 64;

    private Move() {
    }

    static ObjectNode run(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            if ("/".equals(path)) {
                throw new IllegalArgumentException("不能移动根节点");
            }
            String dest = PathName.field(params.path("args"), "dest");
            if (dest.equals(path) || dest.startsWith(path + "/")) {
                throw new IllegalArgumentException("不能移动到自身或子节点下");
            }
            String parent = parent(dest);
            if (client.exists(parent, false) == null) {
                throw new IllegalArgumentException("目标上级不存在");
            }
            if (client.exists(dest, false) != null) {
                throw new IllegalArgumentException("目标已存在");
            }
            int[] count = {0};
            try {
                copy(client, path, dest, 0, count);
            } catch (Exception err) {
                discard(client, dest);
                throw err;
            }
            ZKUtil.deleteRecursive(client, path);
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", dest);
            body.put("nodes", count[0]);
            return body;
        });
    }

    private static void copy(ZooKeeper client, String from, String to, int depth, int[] count) throws Exception {
        if (depth > MAX_DEPTH) {
            throw new IllegalArgumentException("目录层级过深");
        }
        if (count[0] >= MAX_NODES) {
            throw new IllegalArgumentException("节点过多");
        }
        Stat stat = new Stat();
        byte[] data = client.getData(from, false, stat);
        List<ACL> acl = client.getACL(from, new Stat());
        CreateMode mode = stat.getEphemeralOwner() == 0 ? CreateMode.PERSISTENT : CreateMode.EPHEMERAL;
        client.create(to, data == null ? new byte[0] : data, acl, mode);
        count[0]++;
        List<String> names = client.getChildren(from, false);
        if (names == null) {
            return;
        }
        for (String name : names) {
            String child = "/".equals(from) ? "/" + name : from + "/" + name;
            copy(client, child, to + "/" + name, depth + 1, count);
        }
    }

    private static void discard(ZooKeeper client, String path) {
        try {
            if (client.exists(path, false) != null) {
                ZKUtil.deleteRecursive(client, path);
            }
        } catch (Exception ignored) {
            // 半成品清不掉时，仍把复制失败的原因交回去。
        }
    }

    private static String parent(String path) {
        int cut = path.lastIndexOf('/');
        return cut <= 0 ? "/" : path.substring(0, cut);
    }
}
