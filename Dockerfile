FROM golang:1.23-alpine AS builder

WORKDIR /app

# Instala dependências de build (git e make)
RUN apk add --no-cache git make

# Copia o código fonte
COPY . .

# Ajusta o go.mod, cria go.sum e baixa dependências
RUN go mod tidy && go mod download

# Compila o projeto
# -o: nome do executável
# -ldflags: flags de linkagem (opcional, mas útil para remover símbolos de debug)
RUN go build -o event-gateway cmd/api/main.go

# --- Stage 2: Imagem Final (Alpine) ---
FROM alpine:3.18

# Instala apenas o necessário para rodar (caçador de bugs)
RUN apk add --no-cache ca-certificates tzdata

# Define o timezone para UTC (melhor para logs e timestamps)
ENV TZ=UTC

# Cria o usuário não-root (segurança)
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

# Copia o binário compilado do stage anterior
COPY --from=builder /app/event-gateway /app/event-gateway

# Copia o arquivo de configuração (se houver)
COPY --from=builder /app/config.yaml /app/config.yaml

# Define o usuário de execução
USER appuser

# Expõe a porta que o Fiber usa (8080)
EXPOSE 8080

# Comando de inicialização
CMD ["/app/event-gateway"]
