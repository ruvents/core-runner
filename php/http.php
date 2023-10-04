<?php

use Runner\Messages\Request;
use Runner\Messages\Response;
use Runner\Dispatcher;

require_once "vendor/autoload.php";
require_once "GPBMetadata/Messages.php";
require_once "Runner/Messages/Request.php";
require_once "Runner/Messages/Response.php";
require_once "Runner/Messages/File.php";
require_once "Runner/Dispatcher.php";

(new Dispatcher())->run(
    static function (string $msg): string {
        // Десериализация сообщения в нужный объект. В данном случае, это
        // HTTP-запрос, но в теории можно использовать любое protobuf-сообщение.
        $req = new Request(); 
        $req->mergeFromString($msg);

        // Формируем ответ Go-процессу. Тут будет логика приложения.
        // Собираем protobuf-сообщение с HTTP-ответом и отправляем его
        // сериализованную версию.
        $response = (new Response())
            ->setStatusCode(200)
            ->setHeaders(['Content-Type' => 'application/json'])
            ->setBody("{\"method\": \"{$req->getMethod()}\"}")
        ;

        return $response->serializeToString();
    }
);

?>
