# Core Runner

## Запуск

```sh
go run cmd/server/main.go -n 8 \
    -l :3000 -s php/public -p php/http.php \
    -rpc 127.0.0.1:6000 -j php/jobs.php
```
