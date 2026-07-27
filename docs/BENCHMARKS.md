# Goflow Benchmarks

These numbers are local smoke-test results from one development environment. Treat them as directional measurements, not vendor-neutral claims.

Asynchronous trigger latency measures how quickly Goflow accepts and schedules an execution. It does not mean every external API call inside the workflow has completed.

## Scenario A: API-Bound Integration

Workflow shape: GitHub API call plus Google Sheets write.

| Metric | Synchronous trigger | Asynchronous trigger | Notes |
|---|---:|---:|---|
| Total time | 43.256s | 2.356s | Async measures trigger acceptance |
| Throughput | 23.12 req/s | 424.50 req/s | Acceptance rate |
| Success rate | 100% | 100% | 1000 / 1000 requests |
| Average latency | 2098.60ms | 115.86ms | Lower in async mode |
| P50 latency | 2136.33ms | 25.12ms | Lower in async mode |
| P99 latency | 3489.21ms | 1019.07ms | Lower in async mode |
| Idle memory | 22.05MB | 22.05MB | Baseline footprint |
| Peak memory | ~45MB | 162.07MB | Under measured concurrency load |

## Scenario B: CPU-Bound JavaScript And JSON Transform

Workflow shape: recursive Fibonacci(15) inside the Goja JavaScript VM plus JSON Transform.

| Metric | Result |
|---|---:|
| Total time | 1.226s |
| Throughput | 815.98 req/s |
| Success rate | 100% |
| Average latency | 60.47ms |
| P50 latency | 33.95ms |
| P99 latency | 287.31ms |

## Scenario C: Pure Gateway Routing

Workflow shape: fast JSON webhook routing with no heavy computation or external network dependency.

| Metric | Result |
|---|---:|
| Total time | 1.052s |
| Throughput | 950.29 req/s |
| Success rate | 100% |
| Average latency | 52.25ms |
| P50 latency | 12.66ms |
| P99 latency | 301.22ms |

## Reproducibility Notes

Benchmark results depend on machine, Go version, OS, workflow shape, external API latency, credential configuration, and concurrency settings.

When publishing or comparing numbers, include:

- Goflow commit or release version.
- Operating system and CPU/RAM.
- Go version.
- Workflow JSON.
- Request count and worker count.
- Sync or async trigger mode.
- Relevant environment variables such as `GOFLOW_MAX_CONCURRENT_EXECUTIONS` and `GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION`.

