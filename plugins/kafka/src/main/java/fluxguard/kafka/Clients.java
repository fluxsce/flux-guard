package fluxguard.kafka;

import com.fasterxml.jackson.databind.JsonNode;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.AdminClientConfig;

import java.util.Properties;

final class Clients {
    private Clients() {
    }

    static AdminClient open(JsonNode params) {
        Properties props = new Properties();
        props.put(AdminClientConfig.BOOTSTRAP_SERVERS_CONFIG, servers(Session.text(params, "address")));
        props.put(AdminClientConfig.CLIENT_ID_CONFIG, "fluxguard-kafka");
        props.put(AdminClientConfig.REQUEST_TIMEOUT_MS_CONFIG, "20000");
        props.put(AdminClientConfig.DEFAULT_API_TIMEOUT_MS_CONFIG, "20000");
        security(props, params);
        return AdminClient.create(props);
    }

    static String servers(String address) {
        String[] parts = address.split(",");
        StringBuilder out = new StringBuilder();
        for (String part : parts) {
            String item = part.trim();
            if (item.isEmpty()) {
                continue;
            }
            if (!item.contains(":")) {
                item = item + ":9092";
            }
            if (out.length() > 0) {
                out.append(',');
            }
            out.append(item);
        }
        return out.toString();
    }

    private static void security(Properties props, JsonNode params) {
        String option = Session.text(params, "option");
        int cut = option.indexOf(':');
        String deploy = cut < 0 ? option : option.substring(0, cut);
        String auth = cut < 0 ? "" : option.substring(cut + 1);
        String user = Session.text(params, "user");
        String secret = Session.text(params, "secret");
        String mechanism = switch (auth) {
            case "plain" -> "PLAIN";
            case "scram256" -> "SCRAM-SHA-256";
            case "scram512" -> "SCRAM-SHA-512";
            default -> "";
        };
        if (mechanism.isEmpty() && auth.isEmpty() && (!user.isEmpty() || !secret.isEmpty())) {
            mechanism = "PLAIN";
        }
        boolean tls = "tls".equals(deploy);
        if (tls && !mechanism.isEmpty()) {
            props.put("security.protocol", "SASL_SSL");
        } else if (tls) {
            props.put("security.protocol", "SSL");
        } else if (!mechanism.isEmpty()) {
            props.put("security.protocol", "SASL_PLAINTEXT");
        }
        if (mechanism.isEmpty()) {
            return;
        }
        props.put("sasl.mechanism", mechanism);
        String module = "PLAIN".equals(mechanism)
                ? "org.apache.kafka.common.security.plain.PlainLoginModule"
                : "org.apache.kafka.common.security.scram.ScramLoginModule";
        props.put("sasl.jaas.config", module + " required username=\"" + escape(user) + "\" password=\"" + escape(secret) + "\";");
    }

    static String escape(String value) {
        return value.replace("\\", "\\\\").replace("\"", "\\\"");
    }

    static String redact(String message, String secret) {
        if (message == null || message.isEmpty()) {
            return "Kafka 没有应答";
        }
        if (secret != null && !secret.isEmpty()) {
            message = message.replace(secret, "");
        }
        return message.length() > 200 ? message.substring(0, 200) : message;
    }
}
