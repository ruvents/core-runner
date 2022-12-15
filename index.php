<?php

require_once "vendor/autoload.php";
require_once "php/Request.php";
require_once "php/Response.php";
require_once "php/PBList.php";
require_once "php/GPBMetadata/Proto/Request.php";

$in = fopen('php://stdin', 'r');
$out = fopen('php://stdout', 'w');
$err = fopen('php://stderr', 'w');

try {
    foreach (messages($in) as $msg) {
        // Формируем PSR-запрос для совместимости с существующими приложениями.
        $req = psrRequest($msg);

        // Формируем ответ. Тут будет логика приложения.
        $resp = (new \Response())
            ->setBody($req->getMethod())
            ->setHeaders([
                'Content-Type' => (new \PBList())->setValue(['application/json'])
            ]);

        // Отвечаем Go-процессу.
        send($out, $resp->serializeToString());
    }
} catch (\Throwable $e) {
    error($err, $e->getMessage(), $e->getTraceAsString());
}

function messages($stdin): iterable {
    while (($line = fgets($stdin)) !== false) {
        if ($line === "exit\n") {
            break;
        }

        $len = (int) rtrim($line, "\n");
        $msg = '';

        while (($data = fread($stdin, min($len, 2048))) !== false) {
            $msg .= $data;
            $len -= strlen($data);

            if ($len <= 0) {
                yield $msg;
                break;
            }
        }
    }
}

function psrRequest(string $msg): \Request {
    $req = new \Request();
    $req->mergeFromString($msg);

    return $req;
}

function send($stdout, string $data): void {
    fwrite($stdout, strlen($data)."\n");

    foreach (str_split($data, 2048) as $spl) {
        fwrite($stdout, $spl);
    }
}

function error($stderr, string $error, string $context = null): void {
    fwrite($stderr, "\033[1;31mError from PHP: $error\033[0m\n");

    if ($context !== null) {
        $context = preg_replace('/\n/', "\n         ", $context);
        fwrite($stderr, "\033[1mContext:\033[0m {$context}\n");
    }
}

fclose($in);
fclose($out);
fclose($err);
?>
