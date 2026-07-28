# Cedar 認可サンプル

Cedar による認可のサンプルです。

- [01_simple](01_simple/README.md): ドキュメントの所有者・閲覧者による認可
- [02_rbac](02_rbac/README.md): ロールベースアクセス制御 (RBAC)

## Docker で実行

リポジトリ直下で Cedar CLI のイメージを作成します。

```sh
docker build -t cedar-authz-sample .
```

各サンプルをコンテナの `/work` にマウントして実行します。

```sh
docker run --rm \
  -v "$PWD/01_simple:/work" \
  -w /work \
  cedar-authz-sample \
  cedar validate --policies policy.cedar --schema schema.cedarschema

docker run --rm \
  -v "$PWD/01_simple:/work" \
  -w /work \
  cedar-authz-sample \
  bash ./check.sh
```

`02_rbac` を実行する場合は、上記コマンドの `01_simple` を `02_rbac` に変更してください。
