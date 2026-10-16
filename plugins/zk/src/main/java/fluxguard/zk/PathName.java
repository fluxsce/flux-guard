package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;

final class PathName {
    private PathName() {
    }

    static String of(JsonNode args) {
        return field(args, "path");
    }

    static String field(JsonNode args, String name) {
        String path = Session.text(args, name);
        if (path.isEmpty() || path.charAt(0) != '/' || path.indexOf('\0') >= 0) {
            throw new IllegalArgumentException("路径无效");
        }
        if ("/".equals(path)) {
            return path;
        }
        if (path.endsWith("/")) {
            throw new IllegalArgumentException("路径无效");
        }
        for (String part : path.substring(1).split("/", -1)) {
            if (part.isEmpty() || ".".equals(part) || "..".equals(part)) {
                throw new IllegalArgumentException("路径无效");
            }
        }
        return path;
    }
}
