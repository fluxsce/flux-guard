package fluxguard.kafka;

import java.io.Closeable;
import java.io.IOException;

interface BytePipe extends Closeable {
    byte[] readFully(int n) throws IOException;

    void writeFully(byte[] payload) throws IOException;
}
