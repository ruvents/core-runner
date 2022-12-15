...

## Компиляция proto-файлов

https://developers.google.com/protocol-buffers/docs/reference/go-generated#package

Установи protoc и protoc-gen-go, затем:

```sh
# go
protoc --go_out=. messages.proto
```

```sh
# php
composer install
mkdir php -p && protoc --php_out=php/ messages.proto
```
