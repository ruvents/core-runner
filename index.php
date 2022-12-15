<?php

require_once "vendor/autoload.php";
require_once "php/Request.php";
require_once "php/Response.php";
require_once "php/PBList.php";
require_once "php/GPBMetadata/Proto/Request.php";

$in = fopen('php://stdin', 'r');
$out = fopen('php://stdout', 'w');

foreach (messages($in) as $msg) {
    $req = new \Request();
    $req->mergeFromString($msg);

    $resp = (new \Response())
        ->setBody($req->getMethod())
        ->setHeaders([
            'Content-Type' => (new \PBList())->setValue(['application/json'])
        ]);

    $res = $resp->serializeToString();
    fwrite($out, strlen($res)."\n", FILE_APPEND);
    foreach (str_split($res, 2048) as $spl) {
        fwrite($out, $spl);
    }
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

fclose($in);
fclose($out);
?>
