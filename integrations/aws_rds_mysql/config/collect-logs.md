### Collect RDS Logs

#### Find the list of log group names for RDS instance

The log collection of RDS instance requires specifying the list of log group names. From the AWS CloudWatch console, please find the log group(s) relevant to the integration.

#### Create collector config file

Save the following config for collecting RDS logs in a file named `mysql-logs-collection-config.yaml` and set the `region` key with relevant value.

```yaml
receivers:
  awscloudwatch/rds_mysql_logs:
    region: us-east-1
    logs:
      poll_interval: 1m
      groups:
        named:
          # replace the following name with your log group for RDS logs
          /aws/rds/:

processors:
  attributes/add_source_mysql:
    actions:
      - key: source
        value: "rds_mysql"
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
  pipelines:
    logs/mysql:
      receivers: [awscloudwatch/rds_mysql_logs]
      processors: [attributes/add_source_mysql, batch]
      exporters: [otlp/fusescope]
```

#### Update log group names

Add the one or more log group names for the RDS under the `named` section of the awscloudwatch receiver.

#### Set Environment Variables

Set the following environment variables in your otel-collector environment:

```bash

# OTLP gRPC endpoint of your FuseScope collector, see https://docs.fusescope.fuseone.io/sending-data/otlp/
export OTLP_DESTINATION_ENDPOINT="<fusescope-otel-collector-host>:4317"

```

#### Use collector config file

Make the collector config file available to your otel collector and use it by adding the following flag to the command for running your collector  
```bash
--config mysql_logs-collection-config.yaml
```  
Note: the collector can use multiple config files, specified by multiple occurrences of the --config flag.

#### Parse the logs

Use the log pipelines feature to parse and structure the logs https://docs.fusescope.fuseone.io/sending-data/log-pipelines/
