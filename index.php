<?php

class Request {
    public string $httpVersion;
    public string $method;
    public array $query = [];
    public string $path;
    public array $headers = [];
    public string $body = '';
}

function parseHTTP(string $http): Request {
    $req = new Request();
    $lines = explode(PHP_EOL, $http);

    [$req->method, $path, $req->httpVersion] = explode(' ', array_shift($lines), 3);
    $req->path = parse_url($path, PHP_URL_PATH);

    if (is_string($queryStr = parse_url($path, PHP_URL_QUERY))) {
        parse_str($queryStr, $req->query);
    }

    $isBody = false;
    foreach ($lines as $line) {
        if ($line === "") {
            $isBody = true;

            continue;
        }

        if ($isBody === false) {
            // Заголовки.
            [$key, $val] = explode(': ', $line, 2);
            $req->headers[$key] = $val;
        } else {
            // Тело.
            $req->body .= $line;
        }
    }

    return $req;
}

$in = fopen('php://stdin', 'r');
$out = fopen('php://stdout', 'w');

foreach (messages($in) as $msg) {
    $req = parseHTTP($msg);
    $resp = print_r($req, true);
    fwrite($out, mb_strlen($resp."\n")."\n");
    foreach (str_split($resp, 2048) as $spl) {
        fwrite($out, $spl);
    }
    fwrite($out, "\n");
}

function messages($stdin): iterable {
    $len = 0;
    $msg = '';

    while (($line = fgets($stdin)) !== false) {
        if ($len === 0 && $line === "exit\n") {
            break;
        }

        if ($len === 0) {
            echo $len;
            $len = (int) $line;

            continue;
        }

        $msg .= $line;
        $len -= mb_strlen($line);

        if ($len === 0) {
            yield $msg;

            $msg = '';
        }

    }
}


fclose($in);
fclose($out);
?>
