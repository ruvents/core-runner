<?php

/**
 * Пример обработчика задач, приходящих из процесса Go.
 */
require_once "Runner/Dispatcher.php";
require_once "Runner/Serializer.php";
require_once "Runner/Stream.php";
require_once "Runner/Messages/JobResponse.php";

use Runner\Dispatcher;
use Runner\Serializer;
use Runner\Stream;
use Runner\Messages\JobResponse;

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

        $stream = new Stream('');
        $serializer->writeJobResponse($stream, new JobResponse('ok'));

        return $stream->toString();
    }
);

?>
