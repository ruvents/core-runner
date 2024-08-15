<?php

/**
 * Пример обработчика задач, приходящих из процесса Go.
 */
require_once "Runner/Dispatcher.php";
require_once "Runner/Messages/File.php";
require_once "Runner/Messages/HTTPResponse.php";
require_once "Runner/Serializer.php";
require_once "Runner/Stream.php";

use Runner\Dispatcher;
use Runner\Serializer;
use Runner\Stream;

(new Dispatcher())->run(
    static function (string $msg): string {
        // Десериализация сообщения в нужный объект. В данном случае это
        // запрос на выполнение фоновой задачи.
        $serializer = new Serializer();
        $request = $serializer->parseJobRequest(new Stream($msg));

        // Выполнение задачи.
        file_put_contents('php://stderr', "Выполнение фоновой задачи {$request->name}...\n");
        file_put_contents('php://stderr', "Данные: {$request->payload}\n");
        sleep(2);
        file_put_contents('php://stderr', "Фоновая задача выполнена...\n");

        return 'ok';
    }
);

?>
