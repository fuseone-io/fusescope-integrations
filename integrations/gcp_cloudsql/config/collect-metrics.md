### Collect Cloud SQL Metrics

The `googlecloudmonitoring` receiver lists time series from the Cloud Monitoring API. Each metric keeps its Cloud Monitoring name, such as `cloudsql.googleapis.com/database/cpu/utilization`, and the `cloudsql_database` resource labels (`project_id`, `database_id`, `region`) become resource attributes.

Cloud Monitoring bills API reads; the default interval below keeps the cost low. The minimum is 60s.

#### Create collector config file

Save the following config in a file named `gcp-cloudsql-metrics-collection-config.yaml`.

```yaml
receivers:
  googlecloudmonitoring:
    project_id: ${env:GCP_PROJECT_ID}
    collection_interval: 120s
    metrics_list:
      - metric_name: "cloudsql.googleapis.com/database/cpu/utilization"
      - metric_name: "cloudsql.googleapis.com/database/memory/utilization"
      - metric_name: "cloudsql.googleapis.com/database/disk/utilization"
      - metric_name: "cloudsql.googleapis.com/database/disk/bytes_used"
      - metric_name: "cloudsql.googleapis.com/database/disk/read_ops_count"
      - metric_name: "cloudsql.googleapis.com/database/disk/write_ops_count"
      - metric_name: "cloudsql.googleapis.com/database/network/received_bytes_count"
      - metric_name: "cloudsql.googleapis.com/database/network/sent_bytes_count"
      - metric_name: "cloudsql.googleapis.com/database/network/connections"
      - metric_name: "cloudsql.googleapis.com/database/postgresql/num_backends"
      - metric_name: "cloudsql.googleapis.com/database/postgresql/transaction_count"
      - metric_name: "cloudsql.googleapis.com/database/mysql/queries"
      - metric_name: "cloudsql.googleapis.com/database/replication/replica_lag"
      - metric_name: "cloudsql.googleapis.com/database/up"

processors:
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
    metrics/gcp_cloudsql:
      receivers: [googlecloudmonitoring]
      processors: [batch]
      exporters: [otlp/fusescope]
```

To collect from several projects, add one `googlecloudmonitoring/<name>` receiver per project, and grant the service account `roles/monitoring.viewer` in each.

#### Set Environment Variables

Set the following environment variables in your collector environment:

```bash

# project that holds the Cloud SQL instances
export GCP_PROJECT_ID="<project-id>"

# OTLP gRPC endpoint of your FuseScope collector, see https://docs.fusescope.fuseone.io/sending-data/otlp/
export OTLP_DESTINATION_ENDPOINT="<fusescope-otel-collector-host>:4317"

```

#### Use collector config file

Pass the config file to your collector with the following flag:
```bash
--config gcp-cloudsql-metrics-collection-config.yaml
```
Note: the collector can use multiple config files, specified by multiple occurrences of the --config flag.
