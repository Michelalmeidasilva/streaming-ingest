FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/streaming-ingest ./cmd/api

FROM public.ecr.aws/awsguru/aws-lambda-adapter:1.1.0 AS lambda-adapter

FROM public.ecr.aws/docker/library/alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=lambda-adapter /lambda-adapter /opt/extensions/lambda-adapter
COPY --from=builder /out/streaming-ingest /var/task/streaming-ingest

ENV PORT=8080
ENV AWS_LWA_PORT=8080

WORKDIR /var/task

RUN adduser -D appuser
USER appuser

CMD ["./streaming-ingest"]
