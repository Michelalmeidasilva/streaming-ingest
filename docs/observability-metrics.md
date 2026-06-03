# Observabilidade — Métricas Prometheus (RED)

## Endpoint

`GET /metrics` — porta 8080. Formato text/plain padrão Prometheus.

Implementado com `github.com/ansrivas/fiberprometheus/v2` registrado como middleware no
app Fiber (`cmd/api/main.go`).

## Métricas expostas

| Métrica | Tipo | Descrição |
|---|---|---|
| `http_requests_total` | Counter | Total de requisições HTTP recebidas |
| `http_request_duration_seconds_bucket` | Histogram | Latência das requisições HTTP |

### Labels comuns

`service`, `status_code`, `method`, `path`

Label `service` = `streaming-ingest` em todas as séries.

## Scrape pelo Prometheus

O job `streaming-ingest` já está declarado em
`streaming-telemetry/collector/prometheus-config.yaml`:

```yaml
- job_name: streaming-ingest
  metrics_path: /metrics
  static_configs:
    - targets: [streaming-ingest:8080]
      labels: { service: streaming-ingest }
```

Intervalo de scrape: 15 s (global default).

## Sinais entregues

- **Sinal 1 — Requests:** contador de requisições por serviço.
- **Sinal 4 — Erros:** filtra `status_code=~"5.."` no mesmo counter.
- **Sinal 5 — Latência:** percentil sobre o histograma de duração.

## PromQL de referência

```promql
# Throughput (req/s)
sum by (service) (rate(http_requests_total[5m]))

# Taxa de erros 5xx
sum by (service) (rate(http_requests_total{status_code=~"5.."}[5m]))

# Latência p95
histogram_quantile(0.95, sum by (le,service) (rate(http_request_duration_seconds_bucket[5m])))
```
