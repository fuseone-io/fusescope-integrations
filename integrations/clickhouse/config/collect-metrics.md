### Collect Clickhouse Metrics

You can configure Clickhouse metrics collection by providing the required collector config to your collector.

#### Create collector config file

Save the following config for collecting Clickhouse metrics in a file named `clickhouse-metrics-collection-config.yaml`

```yaml
receivers:
  prometheus/clickhouse:
    config:
      global:
        scrape_interval: 60s
      scrape_configs:
        - job_name: clickhouse
          static_configs:
            - targets:
                - ${env:CLICKHOUSE_PROM_METRICS_ENDPOINT}
          metrics_path: ${env:CLICKHOUSE_PROM_METRICS_PATH}

processors:
  # enriches the data with additional host information
  # see https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#resource-detection-processor
  resourcedetection/system:
    # add additional detectors if needed
    detectors: ["system"]
    system:
      hostname_sources: ["os"]

exporters:
  otlp/fusescope:
    endpoint: ${env:OTLP_DESTINATION_ENDPOINT}
    tls:
      # set to false when the FuseScope endpoint uses TLS
      insecure: true

service:
  pipelines:
    metrics/clickhouse:
      receivers: [prometheus/clickhouse]
      # note: remove this processor if the collector host is not running on the same host as the clickhouse instance
      processors: [resourcedetection/system]
      exporters: [otlp/fusescope]
```

#### Set Environment Variables

Set the following environment variables in your otel-collector environment:

```bash
# Prometheus metrics endpoint on the clickhouse server reachable from the otel collector.
# You can examine clickhouse server configuration to find it. For details see https://clickhouse.com/docs/en/operations/server-configuration-parameters/settings#prometheus
export CLICKHOUSE_PROM_METRICS_ENDPOINT="clickhouse:9363"

# Prometheus metrics path on the clickhouse server
# You can examine clickhouse server configuration to find it. For details see https://clickhouse.com/docs/en/operations/server-configuration-parameters/settings#prometheus
export CLICKHOUSE_PROM_METRICS_PATH="/metrics"

# OTLP gRPC endpoint of your FuseScope collector, see https://docs.fusescope.fuseone.io/sending-data/otlp/
export OTLP_DESTINATION_ENDPOINT="<fusescope-otel-collector-host>:4317"

```

#### Use collector config file

Make the collector config file available to your otel collector and use it by adding the following flag to the command for running your collector  
```bash
--config clickhouse-metrics-collection-config.yaml
```  
Note: the collector can use multiple config files, specified by multiple occurrences of the --config flag.
