package fluxguard.zk;

final class Address {
    private Address() {
    }

    static String ensemble(String address) {
        String[] parts = address.split(",");
        StringBuilder out = new StringBuilder();
        for (String part : parts) {
            String item = endpoint(part.trim());
            if (item.isEmpty()) {
                continue;
            }
            if (out.length() > 0) {
                out.append(',');
            }
            out.append(item);
        }
        return out.toString();
    }

    private static String endpoint(String item) {
        if (item.isEmpty()) {
            return "";
        }
        if (item.startsWith("[")) {
            int end = item.indexOf(']');
            if (end < 0) {
                return "";
            }
            if (end == item.length() - 1) {
                return item + ":2181";
            }
            return item;
        }
        if (item.indexOf(':') < 0) {
            return item + ":2181";
        }
        return item;
    }
}
