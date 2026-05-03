# Deploy da Lambda com Imagem Docker na AWS

Este serviço já possui um `Dockerfile` preparado para AWS Lambda via `aws-lambda-adapter`. O fluxo recomendado é:

1. gerar a imagem localmente;
2. enviar a imagem para o Amazon ECR;
3. criar ou atualizar a função Lambda para usar essa imagem.

Importante:

- A AWS Lambda precisa de uma imagem única por arquitetura.
- Evite publicar `manifest list`, `image index` ou imagens multi-arch para essa função.
- Se a imagem for gerada com `docker buildx`, desabilite `provenance` e publique uma imagem simples de `linux/amd64` ou `linux/arm64`.

## Pré-requisitos

- Docker instalado
- AWS CLI configurado com permissão para `ecr` e `lambda`
- Uma conta AWS com:
  - repositório ECR criado ou permissão para criar
  - função Lambda criada ou permissão para criar
  - acesso de rede ao MongoDB e RabbitMQ usados pela aplicação

## Variáveis obrigatórias da aplicação

Essa aplicação exige estas variáveis:

- `RABBITMQ_URL`
- `MONGODB_URI`

Também podem ser necessárias, conforme o ambiente:

- `AWS_REGION`
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `STORAGE_BUCKET`
- `MINIO_ENDPOINT`
- `MINIO_ROOT_USER`
- `MINIO_ROOT_PASSWORD`

Notas:

- A aplicação sobe HTTP na porta `8080`.
- O `Dockerfile` já define `PORT=8080` e `AWS_LWA_PORT=8080`.
- Se MongoDB e RabbitMQ estiverem em rede privada, a Lambda precisará estar na mesma VPC ou ter conectividade com esses serviços.

## 1. Confirmar a arquitetura da Lambda

Antes do build, confirme a arquitetura da função:

```bash
aws lambda get-function-configuration \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --region "$AWS_REGION" \
  --query 'Architectures'
```

Valores comuns:

- `x86_64` -> usar `linux/amd64`
- `arm64` -> usar `linux/arm64`

## 2. Gerar a imagem localmente

Na raiz deste serviço:

Para `x86_64`:

```bash
docker buildx build \
  --platform linux/amd64 \
  --provenance=false \
  --output=type=docker \
  -t streaming-ingest:latest \
  .
```

Para `arm64`:

```bash
docker buildx build \
  --platform linux/arm64 \
  --provenance=false \
  --output=type=docker \
  -t streaming-ingest:latest \
  .
```

Se você estiver usando `docker build` em vez de `buildx`, garanta que a imagem final não seja multi-arch.

Se quiser validar localmente:

```bash
docker run --rm -p 8080:8080 \
  -e RABBITMQ_URL=amqp://usuario:senha@host:5672/ \
  -e MONGODB_URI='mongodb://usuario:senha@host:27017/streaming' \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=seu-access-key \
  -e AWS_SECRET_ACCESS_KEY=sua-secret-key \
  -e STORAGE_BUCKET=videos \
  streaming-ingest:latest
```

## 3. Criar o repositório no ECR

Defina algumas variáveis de shell:

```bash
export AWS_REGION=us-east-1
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export ECR_REPOSITORY=streaming-ingest
export IMAGE_TAG=$(git rev-parse --short HEAD)
```

Crie o repositório se ele ainda não existir:

```bash
aws ecr describe-repositories \
  --repository-names "$ECR_REPOSITORY" \
  --region "$AWS_REGION" >/dev/null 2>&1 || \
aws ecr create-repository \
  --repository-name "$ECR_REPOSITORY" \
  --region "$AWS_REGION"
```

Faça login no ECR:

```bash
aws ecr get-login-password --region "$AWS_REGION" | \
docker login --username AWS --password-stdin \
  "$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com"
```

## 4. Taguear e enviar a imagem

```bash
export ECR_IMAGE_URI="$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPOSITORY:$IMAGE_TAG"

docker tag streaming-ingest:latest "$ECR_IMAGE_URI"
docker push "$ECR_IMAGE_URI"
```

## 5. Criar a função Lambda usando imagem

Se a função ainda não existe:

```bash
export LAMBDA_FUNCTION_NAME=streaming-ingest
export LAMBDA_EXECUTION_ROLE_ARN=arn:aws:iam::123456789012:role/lambda-streaming-ingest-role

aws lambda create-function \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --package-type Image \
  --code ImageUri="$ECR_IMAGE_URI" \
  --role "$LAMBDA_EXECUTION_ROLE_ARN" \
  --timeout 60 \
  --memory-size 512 \
  --region "$AWS_REGION"
```

Depois configure as variáveis de ambiente:

```bash
aws lambda update-function-configuration \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --region "$AWS_REGION" \
  --environment "Variables={RABBITMQ_URL=amqp://usuario:senha@host:5672/,MONGODB_URI=mongodb://usuario:senha@host:27017/streaming,AWS_REGION=$AWS_REGION,STORAGE_BUCKET=videos}"
```

## 6. Atualizar uma Lambda já existente

Para novos deploys, normalmente basta enviar a nova imagem e apontar a função para ela:

```bash
aws lambda update-function-code \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --image-uri "$ECR_IMAGE_URI" \
  --region "$AWS_REGION"
```

Se precisar alterar memória, timeout, VPC ou variáveis:

```bash
aws lambda update-function-configuration \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --timeout 60 \
  --memory-size 512 \
  --region "$AWS_REGION"
```

## 7. Expor a Lambda por HTTP

Como a aplicação é uma API Fiber, o comum é expor a função via:

- API Gateway HTTP API
- ou Lambda Function URL

Se usar API Gateway, direcione as rotas para a Lambda e mantenha a aplicação ouvindo na porta `8080`, como já está no container.

## 8. Observações de rede

Antes de subir em produção, confirme:

- a Lambda consegue acessar `MONGODB_URI`
- a Lambda consegue acessar `RABBITMQ_URL`
- security groups, subnets e route tables permitem esse tráfego
- se o banco ou broker estiverem em VPC privada, a função também precisa estar nessa VPC

## 9. Erro comum: image manifest not supported

Se o deploy falhar com erro parecido com:

```text
The image manifest, config or layer media type for the source image is not supported
```

as causas mais comuns são:

- imagem multi-arch enviada ao ECR
- `OCI image index` em vez de imagem simples
- build com attestation/provenance habilitado
- arquitetura da imagem diferente da arquitetura da Lambda

O caminho mais seguro é rebuildar assim:

```bash
docker buildx build \
  --platform linux/amd64 \
  --provenance=false \
  --output=type=docker \
  -t streaming-ingest:$IMAGE_TAG \
  .
```

Ou, se a função for `arm64`:

```bash
docker buildx build \
  --platform linux/arm64 \
  --provenance=false \
  --output=type=docker \
  -t streaming-ingest:$IMAGE_TAG \
  .
```

Depois:

```bash
docker tag streaming-ingest:$IMAGE_TAG "$ECR_IMAGE_URI"
docker push "$ECR_IMAGE_URI"

aws lambda update-function-code \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --image-uri "$ECR_IMAGE_URI" \
  --region "$AWS_REGION"
```

Se quiser inspecionar o manifest no ECR:

```bash
aws ecr batch-get-image \
  --repository-name "$ECR_REPOSITORY" \
  --image-ids imageTag="$IMAGE_TAG" \
  --region "$AWS_REGION" \
  --query 'images[].imageManifest' \
  --output text
```

Se o retorno indicar algo como `application/vnd.oci.image.index.v1+json`, a imagem publicada não está no formato esperado para esse uso na Lambda.

## 10. Comandos rápidos de deploy

Depois que o ECR e a Lambda já existirem, o ciclo mais curto fica:

```bash
export AWS_REGION=us-east-1
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export ECR_REPOSITORY=streaming-ingest
export LAMBDA_FUNCTION_NAME=streaming-ingest
export IMAGE_TAG=$(git rev-parse --short HEAD)
export ECR_IMAGE_URI="$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPOSITORY:$IMAGE_TAG"

docker buildx build \
  --platform linux/amd64 \
  --provenance=false \
  --output=type=docker \
  -t streaming-ingest:$IMAGE_TAG \
  .

docker tag streaming-ingest:$IMAGE_TAG "$ECR_IMAGE_URI"
docker push "$ECR_IMAGE_URI"

aws lambda update-function-code \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --image-uri "$ECR_IMAGE_URI" \
  --region "$AWS_REGION"
```

Se sua função for `arm64`, troque `linux/amd64` por `linux/arm64`.

## 11. Verificação

Para checar a configuração:

```bash
aws lambda get-function \
  --function-name "$LAMBDA_FUNCTION_NAME" \
  --region "$AWS_REGION"
```

Para ver logs:

```bash
aws logs tail "/aws/lambda/$LAMBDA_FUNCTION_NAME" --follow --region "$AWS_REGION"
```
