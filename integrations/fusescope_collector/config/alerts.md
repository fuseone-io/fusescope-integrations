Each alert compares two adjacent 10-minute windows, so a single blip does not fire, and is evaluated per replica (`host.name`) and component; one replica in trouble alerts on its own.

| Alert | Severity | Fires when |
|---|---|---|
| receiver-refused | critical | a receiver refuses data, by the memory limiter among others, in both windows |
| receiver-failed | critical | a receiver fails to take data in, in both windows |
| export-dropped | critical | `fusescope_clickhouse` or `fusescope_clickhouse/meter` fails to send or enqueue data |
| clickhouse-insert-errors | warning | INSERT attempts into a table fail in both windows |
| queue-backing-up | warning | an exporter queue is over half full in both windows |
| rum-policy-unavailable | critical | RUM requests are refused because the collector holds no ingest policy, in both windows |
| rum-backpressure | warning | RUM requests are refused because the collector or its pipeline is busy, in both windows |
| no-telemetry | critical | no telemetry from the collector for 10 minutes, including when it never reported |

There is no default memory alert: the memory limiter's limit is a share of the container's memory limit, which the collector does not report, and its refusals already raise receiver-refused. RUM refusals by key, origin, signal or rate limit depend on the clients and have no portable baseline: the dashboard shows them by reason.
