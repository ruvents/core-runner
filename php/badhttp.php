<?php

/**
 * Обработчик HTTP-сообщений из процесса Go, периодически выкидывающий ошибки
 * и падающий по таймауту. Нужен для проверки обработки таких ситуаций в Go
 * приложении.
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
        $serializer = new Serializer();
        $request = $serializer->parseHTTPRequest(new Stream($msg));
        usleep(random_int(1, 5) * 1000);

        if (random_int(1, 30) === 7) {
            usleep(20 * 100000);
        }

        if (random_int(1, 30) === 1) {
            throw new \RuntimeException('ERROR');
        }

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
