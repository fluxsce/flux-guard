package fluxguard.zk;

import com.fasterxml.jackson.databind.JsonNode;
import org.apache.zookeeper.Watcher;
import org.apache.zookeeper.ZooKeeper;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

// 同一个集群共用一条会话。目录调用走这条官方客户端，不按请求再拨号。
final class Sessions {
    private static final ConcurrentHashMap<String, ZooKeeper> OPEN = new ConcurrentHashMap<>();

    private Sessions() {
    }

    static ZooKeeper open(JsonNode params) throws Exception {
        String ensemble = Address.ensemble(Session.text(params, "address"));
        if (ensemble.isEmpty()) {
            throw new IllegalArgumentException("请填写地址");
        }
        String user = Session.text(params, "user");
        String secret = Session.text(params, "secret");
        String key = ensemble + "\n" + user + "\n" + secret;
        ZooKeeper current = OPEN.get(key);
        if (alive(current)) {
            return current;
        }
        synchronized (OPEN) {
            current = OPEN.get(key);
            if (alive(current)) {
                return current;
            }
            if (current != null) {
                OPEN.remove(key, current);
                current.close();
            }
            ZooKeeper created = connect(ensemble, user, secret);
            OPEN.put(key, created);
            return created;
        }
    }

    private static boolean alive(ZooKeeper client) {
        return client != null && client.getState().isAlive();
    }

    private static ZooKeeper connect(String ensemble, String user, String secret) throws Exception {
        CountDownLatch ready = new CountDownLatch(1);
        ZooKeeper client = new ZooKeeper(ensemble, 30_000, event -> {
            if (event.getState() == Watcher.Event.KeeperState.SyncConnected) {
                ready.countDown();
            }
        });
        if (!ready.await(10, TimeUnit.SECONDS)) {
            client.close();
            throw new IllegalStateException("ZooKeeper 未连上");
        }
        if (!user.isEmpty() && !secret.isEmpty()) {
            client.addAuthInfo("digest", (user + ":" + secret).getBytes(StandardCharsets.UTF_8));
        }
        return client;
    }
}
