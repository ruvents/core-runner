# Core Runner

## Запуск

```sh
go run cmd/server/main.go -n 8 -l :3000 -s php/public php/index.php
```

## Компиляция proto-файлов

https://developers.google.com/protocol-buffers/docs/reference/go-generated#package

Установи protoc и protoc-gen-go, затем:

```sh
# go
protoc --go_out=. messages.proto
```

```sh
# php
(cd php && composer install)
protoc --php_out=php/ messages.proto
```
