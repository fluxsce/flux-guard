package fluxguard.agent;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

// Report 把一批 JSON 发到中央的上报接口。5xx 和网络失败再送一次。
final class Report {
    private final Config cfg;

    Report(Config cfg) {
        this.cfg = cfg;
    }

    void post(String json) throws Exception {
        Exception last = null;
        for (int attempt = 0; attempt < 2; attempt++) {
            try {
                int status = send(json);
                if (status >= 200 && status < 300) {
                    return;
                }
                last = new Exception("服务端 " + status);
                if (status < 500) {
                    throw last;
                }
            } catch (Exception e) {
                last = e;
            }
        }
        if (last != null) {
            throw last;
        }
    }

    private int send(String json) throws Exception {
        byte[] body = json.getBytes(StandardCharsets.UTF_8);
        URL url = new URL(cfg.serverUrl + "/fluxguard/api/ingest/v1/batches");
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setConnectTimeout(20000);
        conn.setReadTimeout(20000);
        conn.setRequestMethod("POST");
        conn.setDoOutput(true);
        conn.setRequestProperty("Authorization", "Bearer " + cfg.token);
        conn.setRequestProperty("Content-Type", "application/json");
        conn.setRequestProperty("Connection", "close");
        conn.setFixedLengthStreamingMode(body.length);
        try {
            try (OutputStream out = conn.getOutputStream()) {
                out.write(body);
            }
            int status = conn.getResponseCode();
            drain(status >= 400 ? conn.getErrorStream() : conn.getInputStream());
            return status;
        } finally {
            conn.disconnect();
        }
    }

    private static void drain(InputStream in) {
        if (in == null) {
            return;
        }
        try (InputStream stream = in) {
            ByteArrayOutputStream sink = new ByteArrayOutputStream();
            byte[] buf = new byte[1024];
            int n;
            int total = 0;
            while ((n = stream.read(buf)) >= 0 && total < 4096) {
                sink.write(buf, 0, n);
                total += n;
            }
        } catch (Exception ignored) {
            // 应答正文只用来排空连接。
        }
    }
}
