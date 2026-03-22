`PORT` 環境変数を使うものだと簡単に使うことができるが、そうでないものを使い場合は wrapper を通さないと使いづらい。

node 系の有名な server は `--port` オプションを使えるようだが、portless が `--port` に対応しているものは限られているため、wrapper を通す必要がある。

一つの PORT しか自動的に割り当てられないため、一度に複数の port で listen するサーバーには対応できない。

```bash
npm run portless app -- bash ./simpleserver.sh

> portless
> portless app bash ./simpleserver.sh


portless

-- app.localhost (auto-resolves to 127.0.0.1)
-- Proxy is running
-- Using port 4307

  -> http://app.localhost:1355

Running: PORT=4307 HOST=127.0.0.1 PORTLESS_URL=http://app.localhost:1355 bash ./simpleserver.sh

Serving HTTP on 0.0.0.0 port 4307 (http://0.0.0.0:4307/) ...
```
