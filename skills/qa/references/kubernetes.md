# Containers and Kubernetes

Covers Dockerfiles, compose files, Kubernetes manifests, Helm charts and kustomize.

Contents
- Static checks
- Lifecycle, probes and shutdown
- Resources and scaling
- Jobs and CronJobs
- Storage and state
- Security
- Images and builds

## Static checks

Never run anything against a cluster: no `kubectl` commands (including `--dry-run=server`), no `helm install` or `upgrade`.

| Command | Class | Notes |
|---|---|---|
| `kube-linter lint <dir>` | S if installed | |
| `hadolint <Dockerfile>` | S if installed | |
| `helm lint <chart>`, `helm template <chart>` | S if the chart's dependencies are already in `charts/`; N otherwise | Rendering runs templates, not repository code. |
| `kustomize build <dir>` | S without remote bases or plugins; N with remote bases; E with exec or KRM function plugins | |
| `kubeconform` | N unless pointed at local schemas | |
| `docker build`, `docker compose up` | E and N | `RUN` steps execute repository commands and download. |

## Lifecycle, probes and shutdown

- A liveness probe that checks dependencies (database, other services): when a dependency slows, every pod restarts at once.
- No readiness probe, so traffic arrives before the app is ready; or readiness that doesn't reflect real readiness.
- A slow-starting app with a liveness probe but no `startupProbe`, killed before it finishes starting.
- Shell-form `CMD` or `ENTRYPOINT` (`CMD node server.js`) can make `/bin/sh` PID 1, which doesn't forward `SIGTERM`, so the app is killed after the grace period instead of shutting down cleanly. Some shells `exec` a single command and avoid this; check the image's shell, or use the exec form.
- `terminationGracePeriodSeconds` (default 30) shorter than the app's drain time; no `preStop` delay, so the pod stops before endpoints stop routing to it.

## Resources and scaling

- No requests or limits: pods are scheduled anywhere, evicted first under pressure, and an HPA can't scale on CPU without requests.
- Memory limits below real peak usage: OOM kills under load. Runtimes need to know the limit (`GOMEMLIMIT` for Go; Node's `--max-old-space-size`).
- CPU limits that throttle latency-sensitive services.
- A single replica, or several with no `PodDisruptionBudget`, for a service that must stay up through node drains.
- Rolling update settings (`maxUnavailable`, `maxSurge`) that take all capacity down at once, or can't schedule a surge pod on a small cluster.

## Jobs and CronJobs

- `concurrencyPolicy` defaults to `Allow`: a run that outlasts the schedule overlaps the next one. Use `Forbid` or `Replace` unless overlapping is safe.
- `backoffLimit` defaults to 6, so a failing Job retries; the work must be idempotent, and partial work from a failed attempt must be safe.
- Without `startingDeadlineSeconds`, a CronJob that misses more than 100 schedules (for example while its controller was down) stops being scheduled.
- `activeDeadlineSeconds` unset, so a hung job runs forever.
- Schedules are evaluated in the controller's time zone unless `timeZone` is set.

## Storage and state

- `ReadWriteOnce` volumes with more than one replica, or with the `RollingUpdate` strategy: the new pod can't mount the volume while the old one holds it. Node-local provisioners (such as k3s's `local-path`) also pin the pod to one node.
- A stateful service (database, queue) run as a `Deployment` instead of a `StatefulSet`, or with no backup.
- Data written to the container filesystem that is expected to survive a restart.

## Security

- No `securityContext`: running as root, privilege escalation allowed, all capabilities kept, writable root filesystem.
- `privileged: true`, `hostPath` mounts, `hostNetwork`.
- Secrets in plain manifests, ConfigMaps or committed values files; secrets in environment variables visible to anything that can read the pod spec.
- RBAC with wildcard verbs or resources; service account tokens mounted into pods that don't call the API.
- No NetworkPolicy, so every pod can reach every other pod and the database.
- Ingress, Traefik or load balancer timeouts and body-size limits that don't match what the app needs.

## Images and builds

- `COPY . .` with no `.dockerignore`: `.env`, `.git` and credentials end up in the image.
- Secrets passed as `ARG` or `ENV`, which persist in image layers; use build secrets.
- Mutable tags (`latest`, or a floating major version) for base images or deployed images, so the same manifest deploys different code.
- Images that run as root because no `USER` is set.
