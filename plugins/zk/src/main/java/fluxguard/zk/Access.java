package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.zookeeper.ZooDefs;
import org.apache.zookeeper.data.ACL;
import org.apache.zookeeper.data.Id;
import org.apache.zookeeper.data.Stat;
import org.apache.zookeeper.server.auth.DigestAuthenticationProvider;

import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Pattern;

final class Access {
    private static final int MAX_ENTRIES = 32;
    private static final Pattern STORED_DIGEST = Pattern.compile("^[^:]+:[A-Za-z0-9+/]{27}=$");

    private Access() {
    }

    static ObjectNode read(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            Stat stat = new Stat();
            List<ACL> list = client.getACL(path, stat);
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            body.put("aversion", stat.getAversion());
            ArrayNode rows = body.putArray("acl");
            if (list != null) {
                for (ACL acl : list) {
                    ObjectNode row = rows.addObject();
                    row.put("scheme", acl.getId().getScheme());
                    row.put("id", acl.getId().getId());
                    row.put("perms", letters(acl.getPerms()));
                }
            }
            return body;
        });
    }

    static ObjectNode write(JsonNode params) {
        return Outcome.of(params, (client, path) -> {
            List<ACL> list = parse(Session.text(params.path("args"), "acl"));
            Stat stat = client.setACL(path, list, version(params));
            ObjectNode body = Session.JSON.createObjectNode();
            body.put("path", path);
            body.put("aversion", stat.getAversion());
            return body;
        });
    }

    private static List<ACL> parse(String raw) {
        if (raw.length() > 64 * 1024) {
            throw new IllegalArgumentException("权限数据过长");
        }
        JsonNode root;
        try {
            root = Session.JSON.readTree(raw);
        } catch (Exception err) {
            throw new IllegalArgumentException("权限格式无效");
        }
        if (!root.isArray() || root.isEmpty() || root.size() > MAX_ENTRIES) {
            throw new IllegalArgumentException("权限格式无效");
        }
        List<ACL> list = new ArrayList<>();
        for (JsonNode row : root) {
            String scheme = Session.text(row, "scheme");
            String id = Session.text(row, "id");
            boolean idOk = !id.isEmpty() || "auth".equals(scheme);
            if (!scheme.matches("[A-Za-z][A-Za-z0-9._-]{0,31}") || !idOk || id.length() > 1024 || id.indexOf('\0') >= 0) {
                throw new IllegalArgumentException("权限标识无效");
            }
            if ("digest".equals(scheme)) {
                id = digest(id);
            }
            list.add(new ACL(bits(Session.text(row, "perms")), new Id(scheme, id)));
        }
        return list;
    }

    private static String digest(String id) {
        if (!id.contains(":")) {
            throw new IllegalArgumentException("摘要需要用户名和密码");
        }
        if (STORED_DIGEST.matcher(id).matches()) {
            return id;
        }
        try {
            return DigestAuthenticationProvider.generateDigest(id);
        } catch (NoSuchAlgorithmException err) {
            throw new IllegalArgumentException("摘要无效");
        }
    }

    private static int bits(String perms) {
        int bits = 0;
        for (int i = 0; i < perms.length(); i++) {
            switch (perms.charAt(i)) {
                case 'c' -> bits |= ZooDefs.Perms.CREATE;
                case 'r' -> bits |= ZooDefs.Perms.READ;
                case 'w' -> bits |= ZooDefs.Perms.WRITE;
                case 'd' -> bits |= ZooDefs.Perms.DELETE;
                case 'a' -> bits |= ZooDefs.Perms.ADMIN;
                default -> throw new IllegalArgumentException("权限无效");
            }
        }
        if (bits == 0) {
            throw new IllegalArgumentException("权限无效");
        }
        return bits;
    }

    private static String letters(int perms) {
        StringBuilder out = new StringBuilder();
        if ((perms & ZooDefs.Perms.CREATE) != 0) {
            out.append('c');
        }
        if ((perms & ZooDefs.Perms.READ) != 0) {
            out.append('r');
        }
        if ((perms & ZooDefs.Perms.WRITE) != 0) {
            out.append('w');
        }
        if ((perms & ZooDefs.Perms.DELETE) != 0) {
            out.append('d');
        }
        if ((perms & ZooDefs.Perms.ADMIN) != 0) {
            out.append('a');
        }
        return out.toString();
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
}
