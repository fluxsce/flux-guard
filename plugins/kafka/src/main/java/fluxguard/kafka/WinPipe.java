package fluxguard.kafka;

import com.sun.jna.platform.win32.Kernel32;
import com.sun.jna.platform.win32.WinBase;
import com.sun.jna.platform.win32.WinError;
import com.sun.jna.platform.win32.WinNT;
import com.sun.jna.ptr.IntByReference;

import java.io.IOException;
import java.util.concurrent.ExecutorService;

final class WinPipe {
    private static final int ERROR_PIPE_CONNECTED = 535;

    private WinPipe() {
    }

    static void listen(String address, ExecutorService calls) {
        while (!Thread.currentThread().isInterrupted()) {
            WinNT.HANDLE handle = Kernel32.INSTANCE.CreateNamedPipe(
                    address,
                    WinBase.PIPE_ACCESS_DUPLEX,
                    WinBase.PIPE_TYPE_BYTE | WinBase.PIPE_READMODE_BYTE | WinBase.PIPE_WAIT,
                    WinBase.PIPE_UNLIMITED_INSTANCES,
                    64 * 1024,
                    64 * 1024,
                    0,
                    null);
            if (handle == null || WinBase.INVALID_HANDLE_VALUE.equals(handle)) {
                // 这一次创建失败不能结束进程。主线程一返回，守护线程会把整个进程带走。
                pause();
                continue;
            }
            boolean connected = Kernel32.INSTANCE.ConnectNamedPipe(handle, null)
                    || Kernel32.INSTANCE.GetLastError() == ERROR_PIPE_CONNECTED;
            if (!connected) {
                Kernel32.INSTANCE.CloseHandle(handle);
                continue;
            }
            calls.submit(() -> {
                try (Connection pipe = new Connection(handle)) {
                    Session.serve(pipe);
                } catch (Exception ignored) {
                    // 这一条连接结束不影响其它调用。
                } finally {
                    Kernel32.INSTANCE.DisconnectNamedPipe(handle);
                    Kernel32.INSTANCE.CloseHandle(handle);
                }
            });
        }
    }

    private static void pause() {
        try {
            Thread.sleep(20);
        } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
        }
    }

    private static final class Connection implements BytePipe {
        private final WinNT.HANDLE handle;

        Connection(WinNT.HANDLE handle) {
            this.handle = handle;
        }

        @Override
        public byte[] readFully(int n) throws IOException {
            byte[] out = new byte[n];
            byte[] chunk = new byte[Math.min(n, 64 * 1024)];
            int off = 0;
            while (off < n) {
                int size = Math.min(chunk.length, n - off);
                IntByReference read = new IntByReference();
                boolean ok = Kernel32.INSTANCE.ReadFile(handle, chunk, size, read, null);
                int got = read.getValue();
                if (!ok || got <= 0) {
                    int code = Kernel32.INSTANCE.GetLastError();
                    if (code == WinError.ERROR_BROKEN_PIPE || code == WinError.ERROR_NO_DATA || got <= 0) {
                        throw new Frame.EOF();
                    }
                    throw new IOException("read failed");
                }
                System.arraycopy(chunk, 0, out, off, got);
                off += got;
            }
            return out;
        }

        @Override
        public void writeFully(byte[] payload) throws IOException {
            int off = 0;
            while (off < payload.length) {
                int size = Math.min(payload.length - off, 64 * 1024);
                byte[] chunk = new byte[size];
                System.arraycopy(payload, off, chunk, 0, size);
                IntByReference wrote = new IntByReference();
                boolean ok = Kernel32.INSTANCE.WriteFile(handle, chunk, size, wrote, null);
                if (!ok || wrote.getValue() <= 0) {
                    throw new IOException("write failed");
                }
                off += wrote.getValue();
            }
        }

        @Override
        public void close() {
        }
    }
}
