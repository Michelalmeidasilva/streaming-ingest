Estudo de caso plataforma de videos comerciais. 


## Streaming ingest: 
Microsservice Rust


S3 Multipart upload. 
Assinatura das Urls. 


Contexto: 
Upload de 50 videos por dia
Cada video entre 3 e 5 minutos
Considerando um tamanho de em média 554mb(Considerando todas as resoluções).


O arquivo original de vídeo, será armazenado no modo Archive. 
E será enviado para o transcoding para ser armazenado em diferentes resoluções. 



## Banco de dados com os metadados. 
[title: string, description: string, tags: array[string], size: float, id: uuid, status: string]

id,UUID,16 bytes,Armazenado de forma binária.
title,String,~100 bytes,Estimando um título de 100 caracteres em UTF-8.
description,String,~500 bytes,Estimando uma descrição curta de 500 caracteres.
size,Float,8 bytes,Geralmente um double precision (64-bit).
status,String,15 bytes,"Ex: ""active"", ""pending"", ""archived""."
tags,Array[String],~120 bytes,Estimando 5 tags de 20 caracteres + overhead do array.





### Streaming platform upload
## Storage

approx 0,76 KB por registro
40 * 30 = 1200 operações por dia de escrita no banco.
Armazenamento metadados DB: 1200 * 0,76kb = 912kb por dia
Mes: 912kb * 30 = 27.36mb

Armazenamento em storage diariamente em standard premium tier: 40 * 554mb = 22.16gb por dia
Armazenamento em storage diariamente em archive tier: 40 * 122mb = 4.88gb por dia
Mes Premium Tier: 22.16gb * 30 = 664.8gb
Mes ARchive Tier: 4.88gb * 30 = 146.4gb




## Upload ( Multipart upload) - Via async 
Quem envia o video é através de uma API da S3 chamada multipart upload e faz o upload diretamente, com uma lista de URLS pré assinagadas
Chunk do video: Picotar o video e enviar 1 por um. 



