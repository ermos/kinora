-- Per account cap on the throughput of each stream relayed by the proxy, in Mbit/s (0: unlimited).
ALTER TABLE users ADD COLUMN max_stream_mbps INTEGER NOT NULL DEFAULT 0;
