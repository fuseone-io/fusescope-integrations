### Collect Cloud SQL Logs

Cloud SQL writes its logs to Cloud Logging. A log sink routes them to a Pub/Sub topic, and the `googlecloudpubsub` receiver pulls them from a subscription. The `google_cloud_logentry_encoding` extension turns each `LogEntry` into an OpenTelemetry log: timestamp, severity, trace context, labels and the payload as the body.

#### Route the logs to Pub/Sub

```sh
# topic and pull subscription for the logs
gcloud pubsub topics create fusescope-cloudsql-logs --project "$GCP_PROJECT_ID"
gcloud pubsub subscriptions create fusescope-cloudsql-logs \
  --project "$GCP_PROJECT_ID" \
  --topic fusescope-cloudsql-logs \
  --ack-deadline 60 \
  --message-retention-duration 1d

# sink that sends Cloud SQL logs to the topic
gcloud logging sinks create fusescope-cloudsql-logs \
  "pubsub.googleapis.com/projects/$GCP_PROJECT_ID/topics/fusescope-cloudsql-logs" \
  --project "$GCP_PROJECT_ID" \
  --log-filter 'resource.type="cloudsql_database"'

# let the sink publish to the topic
gcloud pubsub topics add-iam-policy-binding fusescope-cloudsql-logs \
  --project "$GCP_PROJECT_ID" \
  --member "$(gcloud logging sinks describe fusescope-cloudsql-logs --project "$GCP_PROJECT_ID" --format 'value(writerIdentity)')" \
  --role roles/pubsub.publisher

# let the collector pull from the subscription
gcloud pubsub subscriptions add-iam-policy-binding fusescope-cloudsql-logs \
  --project "$GCP_PROJECT_ID" \
  --member "serviceAccount:fusescope-collector@$GCP_PROJECT_ID.iam.gserviceaccount.com" \
  --role roles/pubsub.subscriber
```

Narrow `--log-filter` to keep costs down, for example `resource.type="cloudsql_database" AND severity>=WARNING`. For the slow query log of Cloud SQL for MySQL, set the `slow_query_log` and `log_output=FILE` database flags.

#### Create collector config file

Save the following config in a file named `gcp-cloudsql-logs-collection-config.yaml`.

```yaml
extensions:
  google_cloud_logentry_encoding:
    handle_json_payload_as: json

receivers:
  googlecloudpubsub:
    project: ${env:GCP_PROJECT_ID}
    subscription: ${env:GCP_PUBSUB_SUBSCRIPTION}
    encoding: google_cloud_logentry_encoding

processors:
  attributes/gcp_cloudsql:
    actions:
      - key: source
        value: gcp_cloudsql
        action: insert
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
  extensions: [google_cloud_logentry_encoding]
  pipelines:
    logs/gcp_cloudsql:
      receivers: [googlecloudpubsub]
      processors: [attributes/gcp_cloudsql, batch]
      exporters: [otlp/fusescope]
```

#### Set Environment Variables

Set the following environment variables in your collector environment:

```bash

# project that holds the Pub/Sub subscription
export GCP_PROJECT_ID="<project-id>"

# full name of the subscription created above
export GCP_PUBSUB_SUBSCRIPTION="projects/<project-id>/subscriptions/fusescope-cloudsql-logs"

# OTLP gRPC endpoint of your FuseScope collector, see https://fuseone-io.github.io/fusescope-docs/sending-data/otlp/
export OTLP_DESTINATION_ENDPOINT="<fusescope-otel-collector-host>:4317"

```

#### Use collector config file

Pass the config file to your collector with the following flag:
```bash
--config gcp-cloudsql-logs-collection-config.yaml
```
Note: the collector can use multiple config files, specified by multiple occurrences of the --config flag.

#### Parsed attributes

The log name (for example `cloudsql.googleapis.com%2Fmysql.err`) is in the `cloud.resource_id` resource attribute, the instance in `gcp.label.database_id`.

| Log | Attributes |
|---|---|
| MySQL error log (`mysql.err`) | `thread_id`, `error_code`, `subsystem` |
| PostgreSQL log (`postgres.log`) | `pid`, `db_name`, `db_user` |
