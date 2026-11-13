The FuseScope Collector of the FuseScope chart writes its own telemetry while `fusescopeCollector.selfMetrics.enabled` is `true`, the default:

```yaml
fusescopeCollector:
  selfMetrics:
    enabled: true
    scrapeInterval: 60s
```

The alerts compare two 10-minute windows, so keep `scrapeInterval` at 5 minutes or less.

Only the gateway's telemetry is covered. Traffic from FuseScope agents shows on the gateway as the receiver `otlp/agents`; the agents' own telemetry is not collected yet.
