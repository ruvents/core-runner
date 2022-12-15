<?php

require_once "vendor/autoload.php";
require_once "Request.php";
require_once "Response.php";
require_once "PBList.php";
require_once "GPBMetadata/Messages.php";

$in = fopen('php://stdin', 'r');
$out = fopen('php://stdout', 'w');
$err = fopen('php://stderr', 'w');

try {
    foreach (messages($in, $err) as $msg) {
        // Формируем PSR-запрос для совместимости с существующими приложениями.
        $req = psrRequest($msg);

        // Формируем ответ. Тут будет логика приложения.
        $resp = formResponse(
            200,
            ['Content-Type' => 'application/json'],
            "{\"method\": \"{$req->getMethod()}\"}"
        );

        // Отвечаем Go-процессу.
        send($out, $resp->serializeToString());
    }
} catch (\Throwable $e) {
    error($err, $e->getMessage(), $e->getTraceAsString());
}

function messages($stdin, $stderr): iterable {
    while (($line = fgets($stdin)) !== false) {
        $line = rtrim($line, PHP_EOL);

        if ($line === "exit") {
            break;
        }

        $len = (int) $line;
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

function formResponse(int $statusCode, array $headers, string $body): \Response {
    return (new \Response())
        ->setStatusCode($statusCode)
        ->setHeaders($headers)
        ->setBody($body)
    ;
}

function send($stdout, string $data): void {
    fwrite($stdout, strlen($data)."\n");

    // FIXME: 2048 здесь указывать неверно, т.к. это количество символов, а
    // должно быть байт.
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
