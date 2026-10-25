### Collect MySQL Metrics

#### Create collector config file

Save the following config in a file named `mysql-metrics-collection-config.yaml`.

```yaml
receivers:
  mysql:
    # host:port of the MySQL server
    endpoint: ${env:MYSQL_ENDPOINT}
    username: ${env:MYSQL_USERNAME}
    password: ${env:MYSQL_PASSWORD}
    collection_interval: 60s
    # tls:
    #   insecure: false
    #   ca_file: /etc/ssl/certs/mysql-ca.pem
    metrics:
      mysql.client.network.io:
        enabled: true
      mysql.commands:
        enabled: true
      mysql.connection.count:
        enabled: true
      mysql.connection.errors:
        enabled: true
      mysql.joins:
        enabled: true
      mysql.max_used_connections:
        enabled: true
      mysql.query.count:
        enabled: true
      mysql.query.slow.count:
        enabled: true
      mysql.replica.sql_delay:
        enabled: true
      mysql.replica.time_behind_source:
        enabled: true

processors:
  # adds host.name; remove it when the collector does not run on the MySQL host
  resourcedetection/system:
    detectors: ["system"]
    system:
      hostname_sources: ["os"]
  batch:
    send_batch_size: 10000
    timeout: 10s

exporters:
  otlp/fusescope:
    endpoint: ${env:OTLP_DESTINATION_ENDPOINT}
    tls:
      # set to false when the FuseScope endpoint uses TLS
      insecure: true

service:
  pipelines:
    metrics/mysql:
      receivers: [mysql]
      processors: [resourcedetection/system, batch]
      exporters: [otlp/fusescope]
```

#### Set Environment Variables

Set the following environment variables in your collector environment:

```bash

# MySQL server reachable from the collector, as host:port
export MYSQL_ENDPOINT="localhost:3306"

# monitoring user created in the prerequisites
export MYSQL_USERNAME="fusescope"

# password of the monitoring user
export MYSQL_PASSWORD="<PASSWORD>"

# OTLP gRPC endpoint of your FuseScope collector, see https://fuseone-io.github.io/fusescope-docs/sending-data/otlp/
export OTLP_DESTINATION_ENDPOINT="<fusescope-otel-collector-host>:4317"

```

#### Use collector config file

Pass the config file to your collector with the following flag:
```bash
--config mysql-metrics-collection-config.yaml
```
Note: the collector can use multiple config files, specified by multiple occurrences of the --config flag.
