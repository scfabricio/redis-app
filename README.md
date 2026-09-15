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

## Benchmark

```sh
set -a; source .env; set +a
go run ./cmd/benchmark -count 100
```

O comando reporta três cenários sequenciais: somente PostgreSQL, cache frio e cache quente. Ele mostra tempo total, médio, mínimo, máximo, hits e misses. O banco inicia com mais de 100 marcas, portanto a execução padrão consulta 100 registros distintos.

## Arquitetura

`HTTP -> BrandService -> Redis -> PostgreSQL`

`BrandService` concentra a política de cache; `repository` contém apenas acesso ao PostgreSQL; `cache` contém a serialização e chaves Redis. Isso permite testar e trocar cada camada isoladamente.
