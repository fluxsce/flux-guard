package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.zookeeper.data.Stat;

final class Meta {
    private Meta() {
    }

    static ObjectNode read(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            Stat stat = client.exists(path, false);
            if (stat == null) {
                throw new IllegalArgumentException("节点不存在");
            }
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            Outcome.putStat(body, stat);
            return body;
        });
    }
}
