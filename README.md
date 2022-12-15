...

## Запуск

```sh
go run cmd/server/main.go -w 8 -p 8080 php/index.php
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
mkdir php -p && protoc --php_out=php/ messages.proto
```
