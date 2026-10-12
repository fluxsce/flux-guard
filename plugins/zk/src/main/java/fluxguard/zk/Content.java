package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.zookeeper.data.Stat;

import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.util.Base64;

final class Content {
    private Content() {
    }

    static ObjectNode read(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            Stat stat = new Stat();
            byte[] data = client.getData(path, false, stat);
            if (data == null) {
                data = new byte[0];
            }
            if (data.length > Outcome.PAYLOAD) {
                throw new IllegalArgumentException("返回超过 10MB");
            }
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            Outcome.putStat(body, stat);
            putData(body, data);
            return body;
        });
    }

    private static void putData(ObjectNode body, byte[] data) {
        String text = utf8(data);
        if (text != null) {
            body.put("data", text);
            return;
        }
        String encoded = Base64.getEncoder().encodeToString(data);
        if (encoded.length() > Outcome.PAYLOAD) {
            throw new IllegalArgumentException("返回超过 10MB");
        }
        body.put("dataBase64", encoded);
    }

    private static String utf8(byte[] data) {
        try {
            return StandardCharsets.UTF_8.newDecoder()
                    .onMalformedInput(CodingErrorAction.REPORT)
                    .onUnmappableCharacter(CodingErrorAction.REPORT)
                    .decode(ByteBuffer.wrap(data))
                    .toString();
        } catch (CharacterCodingException err) {
            return null;
        }
    }
}
