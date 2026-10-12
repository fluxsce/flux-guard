package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.List;

final class Children {
    private Children() {
    }

    static ObjectNode list(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            List<String> names = client.getChildren(path, false);
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            ArrayNode list = body.putArray("children");
            if (names != null) {
                for (String name : names) {
                    list.add(name);
                }
            }
            return body;
        });
    }
}
