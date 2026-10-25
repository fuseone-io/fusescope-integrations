## Before You Begin

To collect Cloud SQL metrics and logs, you need the following.

### Prepare Google Cloud

The commands below use the [gcloud CLI](https://cloud.google.com/sdk/docs/install) and read the project from `GCP_PROJECT_ID`:

```bash
export GCP_PROJECT_ID="<project-id>"
```

- **Enable the APIs** in the project that holds the Cloud SQL instances:
  ```sh
  gcloud services enable monitoring.googleapis.com logging.googleapis.com pubsub.googleapis.com \
    --project "$GCP_PROJECT_ID"
  ```

- **Create a service account for the collector**
  ```sh
  gcloud iam service-accounts create fusescope-collector \
    --project "$GCP_PROJECT_ID" \
    --display-name "FuseScope collector"
  ```

- **Grant it the roles it needs**

  | Role | On | Used for |
  |---|---|---|
  | `roles/monitoring.viewer` | project | reading metrics from Cloud Monitoring |
  | `roles/pubsub.subscriber` | the log subscription | pulling logs from Pub/Sub, granted in the **Collect Logs** step |

  ```sh
  gcloud projects add-iam-policy-binding "$GCP_PROJECT_ID" \
    --member "serviceAccount:fusescope-collector@$GCP_PROJECT_ID.iam.gserviceaccount.com" \
    --role roles/monitoring.viewer
  ```

  The person running the setup needs `roles/iam.serviceAccountAdmin`, `roles/resourcemanager.projectIamAdmin`, `roles/pubsub.admin` and `roles/logging.configWriter`, or `roles/owner`.

- **Give the collector the service account's credentials**
  The receivers use [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials).
  - On Compute Engine, attach the service account to the VM.
  - On GKE, bind it to the collector's Kubernetes service account with [Workload Identity Federation](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).
  - Elsewhere, create a key and point `GOOGLE_APPLICATION_CREDENTIALS` at it. Store the key file as a secret and keep it out of version control.
    ```sh
    gcloud iam service-accounts keys create fusescope-collector-key.json \
      --iam-account "fusescope-collector@$GCP_PROJECT_ID.iam.gserviceaccount.com"
    ```
    ```bash
    export GOOGLE_APPLICATION_CREDENTIALS=/etc/otelcol-contrib/fusescope-collector-key.json
    ```

### Prepare the OpenTelemetry Collector

- **Run an OpenTelemetry Collector**
  If needed, [install the OpenTelemetry Collector Contrib distribution](https://opentelemetry.io/docs/collector/installation/). The configs below are validated with `otelcol-contrib` v0.163; older releases may lack the `google_cloud_logentry_encoding` extension.

  Make sure you can pass config files, environment variables and command line flags to it.

- **Allow outbound HTTPS** from the collector to `monitoring.googleapis.com` and `pubsub.googleapis.com`.
