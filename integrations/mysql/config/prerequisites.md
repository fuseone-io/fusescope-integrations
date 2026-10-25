## Before You Begin

To collect metrics and logs from a MySQL server, you need the following.

### Prepare the MySQL server

- **Create a monitoring user**
  The `mysql` receiver runs `SHOW GLOBAL STATUS` and reads `performance_schema`. Create a dedicated user for it:
  ```SQL
  CREATE USER 'fusescope'@'%' IDENTIFIED BY '<PASSWORD>';
  GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'fusescope'@'%';
  GRANT SELECT ON performance_schema.* TO 'fusescope'@'%';
  ```
  Restrict the host part (`'%'`) to the address of your collector where possible.

- **Enable the slow query log** (optional, for slow query logs)
  ```SQL
  SET GLOBAL slow_query_log = 'ON';
  SET GLOBAL long_query_time = 1;
  SHOW VARIABLES LIKE 'slow_query_log_file';
  ```
  Add the same settings to `my.cnf` to keep them across restarts.

- **Find the error log**
  ```SQL
  SHOW VARIABLES LIKE 'log_error';
  ```
  The pipelines expect the default text format of the error log (`log_error_services` with `log_sink_internal`), with `log_timestamps` set to `UTC` (the default) or `SYSTEM`.

### Prepare the OpenTelemetry Collector

- **Run an OpenTelemetry Collector**
  If needed, [install the OpenTelemetry Collector Contrib distribution](https://opentelemetry.io/docs/collector/installation/). The configs below are validated with `otelcol-contrib` v0.163.

  Make sure you can pass config files, environment variables and command line flags to it.

- **Give the collector access to MySQL**
  To collect metrics, the collector must reach the MySQL server as a client with the monitoring user.

  To collect logs, the collector must run on the MySQL host (or see its log directory) and be able to read the error log and the slow query log.
