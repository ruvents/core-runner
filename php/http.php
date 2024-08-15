<?php

/**
 * Пример обработчика HTTP-сообщений из процесса Go.
 */
require_once "Runner/Dispatcher.php";
require_once "Runner/Messages/File.php";
require_once "Runner/Messages/HTTPResponse.php";
require_once "Runner/Serializer.php";
require_once "Runner/Stream.php";

use Runner\Dispatcher;
use Runner\Messages\File;
use Runner\Messages\HTTPResponse;
use Runner\Serializer;
use Runner\Stream;

(new Dispatcher())->run(
    static function (string $msg): string {
        // Десериализация сообщения в нужный объект. В данном случае это
        // HTTP-запрос, но в теории можно использовать любое сериализуемое
        // сообщение.
        $serializer = new Serializer();
        $request = $serializer->parseHTTPRequest(new Stream($msg));

        // Формируем ответ Go-процессу. Тут будет логика приложения.
        // Собираем HTTP-ответ и отправляем его сериализованную версию.
        $response = new HTTPResponse(
            200,
            $request->headers,
            json_encode([
                'body' => $request->body,
                'files' => array_map(
                    fn (File $f): array => [
                        'filename' => $f->filename,
                        'size' => $f->size,
                        'tmpPath' => $f->tmpPath,
                    ],
                    $request->files,
                ),
                'form' => $request->form,
            ]),
        );

        $stream = new Stream('');
        $serializer->writeHTTPResponse($stream, $response);

        return $stream->toString();
    }
);

?>
