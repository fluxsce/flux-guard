package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.zookeeper.ZooKeeper;
import org.apache.zookeeper.data.Stat;

final class Outcome {
    static final int PAYLOAD = 10 << 20;

    private Outcome() {
    }

    interface Body {
        ObjectNode build(ZooKeeper client, String path) throws Exception;
    }

    static ObjectNode of(JsonNode params, Body body) {
        ObjectNode result = Session.JSON.createObjectNode();
        String secret = Session.text(params, "secret");
        try {
            String path = PathName.of(params.path("args"));
            ObjectNode built = body.build(Sessions.open(params), path);
            String output = Session.JSON.writeValueAsString(built);
            if (output.length() > PAYLOAD) {
                throw new IllegalArgumentException("返回超过 10MB");
            }
            result.put("output", output);
        } catch (Exception err) {
            result.put("error", redact(err.getMessage(), secret));
        }
        return result;
    }

    static void putStat(ObjectNode body, Stat stat) {
        body.put("czxid", stat.getCzxid());
        body.put("mzxid", stat.getMzxid());
        body.put("ctime", stat.getCtime());
        body.put("mtime", stat.getMtime());
        body.put("version", stat.getVersion());
        body.put("cversion", stat.getCversion());
        body.put("aversion", stat.getAversion());
        body.put("dataLength", stat.getDataLength());
        body.put("numChildren", stat.getNumChildren());
        body.put("ephemeralOwner", stat.getEphemeralOwner());
        body.put("pzxid", stat.getPzxid());
    }

    static String redact(String message, String secret) {
        if (message == null || message.isEmpty()) {
            return "ZooKeeper 没有应答";
        }
        if (secret != null && !secret.isEmpty()) {
            message = message.replace(secret, "");
        }
        return message.length() > 200 ? message.substring(0, 200) : message;
    }
}
