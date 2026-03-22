<https://zenn.dev/mickamy/articles/9a251e7cf51b9c> の方式

```bash
docker network create traefik
docker compose up -d
docker run --rm -d \
    --name nginx \
    --network=traefik \
    --label com.docker.compose.service=web \
    --label com.docker.compose.project=hoge \
    nginx:latest

docker build -t simpleserver .
docker run --rm -d \
    --name simpleserver \
    --network=traefik \
    --label com.docker.compose.service=simple \
    --label com.docker.compose.project=hoge \
    simpleserver:latest

# http://web.hoge.localhost で接続できるようになる

docker stop nginx
docker compose down
docker network rm traefik
```

コンテナ側に label を付与するバージョン

```bash
docker compose -f compose2.yaml up -d
# http://example.com.localhost で接続できるようになる
```
