# Catálogo de marcas com cache Redis

Aplicação Go que consulta marcas de carros em PostgreSQL usando o padrão **cache-aside**. Cada marca fica no Redis por 10 segundos; falhas no cache não impedem a consulta ao banco.

## Executar

```sh
cp .env.example .env
docker compose up -d
set -a; source .env; set +a
go run ./cmd/api
```

Em outra janela:

```sh
curl -i http://localhost:8080/brands/1
```

O cabeçalho `X-Data-Source` indica `database` na primeira chamada e `cache` nas chamadas seguintes, até o TTL expirar.

## Eventos de brands (Redis Pub/Sub)

Após atualizar uma marca, a API publica um JSON no canal `brand.updated`. O evento contém
o tipo (`brand.updated`), o instante UTC da alteração e o snapshot completo da marca.
Pub/Sub é **best-effort**: se o Redis não estiver disponível, a marca continua atualizada
no PostgreSQL e a API registra a falha no log.

Em um segundo terminal, execute o consumidor de exemplo:

```sh
set -a; source .env; set +a
go run ./cmd/brand-events
```

Então atualize uma marca:

```sh
curl -i -X PUT http://localhost:8080/brands/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Toyota Motor","country":"Japan"}'
```

O consumidor exibirá o evento recebido. Para inspecionar diretamente no Redis, use outro
assinante antes do `PUT`:

```sh
docker exec -it redis-app redis-cli -a "$REDIS_PASSWORD" SUBSCRIBE brand.updated
```

Mensagens só chegam aos assinantes conectados quando o `PUBLISH` ocorre; Pub/Sub não
armazena, confirma nem reprocessa eventos. Para entrega confiável, use Redis Streams ou
um transactional outbox.

## Benchmark

```sh
set -a; source .env; set +a
go run ./cmd/benchmark -count 100
```

O comando reporta três cenários sequenciais: somente PostgreSQL, cache frio e cache quente. Ele mostra tempo total, médio, mínimo, máximo, hits e misses. O banco inicia com mais de 100 marcas, portanto a execução padrão consulta 100 registros distintos.

## Arquitetura

`HTTP -> BrandService -> PostgreSQL`

Na leitura, o serviço usa Redis como cache-aside. Na atualização, invalida a entrada de
cache e publica `brand.updated` para consumidores conectados.

`BrandService` concentra a política de cache; `repository` contém apenas acesso ao PostgreSQL; `cache` contém a serialização e chaves Redis. Isso permite testar e trocar cada camada isoladamente.
