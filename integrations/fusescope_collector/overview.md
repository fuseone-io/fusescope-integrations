### Monitor the FuseScope Collector

The FuseScope Collector writes its own telemetry, the `otelcol_*` metrics of its receivers, exporters and queues, to the same storage as the data it receives, under `service.name` `fusescope-collector`. This integration reads it.

#### What you get

- **Alerts** on data the collector refuses, fails to receive or drops on export, on ClickHouse insert errors, on export queues over half full, on RUM refusals that point at the collector rather than at clients, and on the collector's telemetry going missing. Each alert names the replica (`host.name`) and the receiver, exporter, table or reason involved.
- **A dashboard** with the same signals per replica and component, the collector's memory, and how many replicas report.

The alerts follow the notification policies of your organization.
