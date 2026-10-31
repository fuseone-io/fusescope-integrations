### Collect MySQL Logs

The collector reads the error log and the slow query log and tags each record with `source: mysql` and `log_type: error` or `log_type: slow`. The log pipelines installed with this integration parse them in FuseScope.

#### Create collector config file

Save the following config in a file named `mysql-logs-collection-config.yaml`.

```yaml
receivers:
  filelog/mysql_error:
    include: ["${env:MYSQL_ERROR_LOG_FILE}"]
    # a record starts with its timestamp; following lines belong to it
    multiline:
      line_start_pattern: '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}'
    operators:
      - type: add
        field: attributes.source
        value: mysql
      - type: add
        field: attributes.log_type
        value: error

  filelog/mysql_slow:
    include: ["${env:MYSQL_SLOW_QUERY_LOG_FILE}"]
    # an entry starts with its "# Time:" header and holds the statement
    multiline:
      line_start_pattern: '^# Time: '
    operators:
      - type: add
        field: attributes.source
        value: mysql
      - type: add
        field: attributes.log_type
        value: slow

processors:
  batch:
    send_batch_size: 10000
    send_batch_max_size: 11000
    timeout: 10s

exporters:
  otlp/fusescope:
    endpoint: ${env:OTLP_DESTINATION_ENDPOINT}
    tls:
      # set to false when the FuseScope endpoint uses TLS
      insecure: true

service:
  pipelines:
    logs/mysql:
      receivers: [filelog/mysql_error, filelog/mysql_slow]
      processors: [batch]
      exporters: [otlp/fusescope]
```

#### Set Environment Variables

Set the following environment variables in your collector environment:

```bash

# MySQL error log, the value of the log_error variable
export MYSQL_ERROR_LOG_FILE=/var/log/mysql/error.log

# MySQL slow query log, the value of the slow_query_log_file variable
export MYSQL_SLOW_QUERY_LOG_FILE=/var/log/mysql/mysql-slow.log

# OTLP gRPC endpoint of your FuseScope collector, see https://docs.fusescope.fuseone.io/sending-data/otlp/
export OTLP_DESTINATION_ENDPOINT="<fusescope-otel-collector-host>:4317"

```

#### Use collector config file

Pass the config file to your collector with the following flag:
```bash
--config mysql-logs-collection-config.yaml
```
Note: the collector can use multiple config files, specified by multiple occurrences of the --config flag.

#### Parsed attributes

| Log | Attributes |
|---|---|
| Error log | timestamp, severity, `thread_id`, `error_code` (MySQL 8.0+), `subsystem` (MySQL 8.0+) |
| Slow query log | timestamp, `db_user`, `client_host`, `client_ip`, `thread_id`, `query_time`, `lock_time`, `rows_sent`, `rows_examined`; the statement becomes the body |
