# Init NiceHash

```
# Build a Docker image for the initNiceHash utility
docker build --rm -t init_nicehash .

# Run the initNiceHash utility to retrieve NiceHash configurations
# (using default NiceHash API and ZooKeeper is running on localhost)
docker run --rm init_nicehash initNiceHash -zookeeper 127.0.0.1:2181 -path /nicehash
```