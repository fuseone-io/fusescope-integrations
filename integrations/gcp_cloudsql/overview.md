### Monitor Google Cloud SQL with FuseScope

Collect Cloud SQL metrics from Cloud Monitoring with the OpenTelemetry `googlecloudmonitoring` receiver and view CPU, memory, disk, network, connections and replica lag on an out of the box dashboard.

Collect Cloud SQL logs by routing them from Cloud Logging to a Pub/Sub topic and reading the subscription with the `googlecloudpubsub` receiver. Installed log pipelines parse the MySQL error log and the PostgreSQL log.

Covers Cloud SQL for MySQL, PostgreSQL and SQL Server. To collect engine metrics as well, add the MySQL or PostgreSQL integration pointed at the instance.
