package fluxguard.kafka;

import java.io.EOFException;
import java.io.IOException;

final class Frame {
    static final int MAX = 1 << 20;

    private Frame() {
    }

    static byte[] read(BytePipe pipe) throws IOException {
        byte[] header = pipe.readFully(4);
        int n = (header[0] & 0xff) | ((header[1] & 0xff) << 8) | ((header[2] & 0xff) << 16) | ((header[3] & 0xff) << 24);
        if (n < 0 || n > MAX) {
            throw new IOException("frame too large");
        }
        if (n == 0) {
            return new byte[0];
        }
        return pipe.readFully(n);
    }

    static void write(BytePipe pipe, byte[] payload) throws IOException {
        if (payload.length > MAX) {
            throw new IOException("frame too large");
        }
        byte[] header = new byte[4];
        int n = payload.length;
        header[0] = (byte) n;
        header[1] = (byte) (n >>> 8);
        header[2] = (byte) (n >>> 16);
        header[3] = (byte) (n >>> 24);
        pipe.writeFully(header);
        if (n > 0) {
            pipe.writeFully(payload);
        }
    }

    static final class EOF extends EOFException {
        EOF() {
            super("eof");
        }
    }
}
