<?php

declare(strict_types=1);

namespace Runner;

require_once "Messages/File.php";
require_once "Messages/HTTPRequest.php";
require_once "Messages/HTTPResponse.php";
require_once "Messages/JobRequest.php";

use Runner\Messages;

/**
 * Класс, реализующий сериализацию и десериализацию сообщений по бинарному
 * протоколу для передачи данных в Go процесс и получения данных от него.
 *
 * Сам протокол очень простой, поддерживает два базовых типа данных:
 * 1) string (+ отдельно bytes, но PHP не знает разницы между ними)
 * 2) uint64
 *
 * Все данные записываются/читаются в little endian. Тип данных в самом поле не
 * указывается. Порядок записи и чтения полей должен повторяться, т.е. порядок
 * полей имеет значение.
 *
 * Строка записывается так: сначала uint64 с количеством байт в строке, потом
 * сама строка:
 * [len(str)][str]
 *
 * Массивы записываются так: сначала uint64 с количеством элементов, потом сами
 * элементы подряд:
 * [len(arr)][element1][element2][...]
 *
 * Карты/словари записываются так: сначала uint64 с количеством элементов,
 * потом пара: ключ элемента и сам элемент:
 * [len(map)][key1][value1][key2][value2][...].
 */
final class Serializer
{
    public function parseHTTPRequest(Stream $stream): Messages\HTTPRequest {
        $method = $this->parseString($stream);

        if ($method === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле method.'
            );
        }

        $url = $this->parseString($stream);

        if ($url === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле url.'
            );
        }

        $headers = $this->parseStringMap($stream);

        if ($headers === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле headers.'
            );
        }

        $body = $this->parseString($stream);

        if ($body === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле body.'
            );
        }

        $files = $this->parseFileMap($stream);

        if ($files === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле files.'
            );
        }

        $form = $this->parseStringMap($stream);

        if ($form === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле form.'
            );
        }

        return new Messages\HTTPRequest(
            $method,
            $url,
            $body,
            $headers,
            $files,
            $form
        );
    }

    /**
     * @param resource $file
     */
    public function writeHTTPResponse(
        Stream $stream,
        Messages\HTTPResponse $response,
    ): void
    {
        $this->writeUint64($stream, $response->statusCode);
        $this->writeStringMap($stream, $response->headers);
        $this->writeString($stream, $response->body);
    }

    public function parseFile(Stream $stream): Messages\File {
        $filename = $this->parseString($stream);

        if ($filename === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле filename'
            );
        }

        $tmpPath = $this->parseString($stream);

        if ($tmpPath === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле tmpPath'
            );
        }

        $size = $this->parseUint64($stream);

        if ($size === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле size'
            );
        }

        return new Messages\File($filename, $tmpPath, $size);
    }

    public function parseJobRequest(Stream $stream): Messages\JobRequest
    {
        $name = $this->parseString($stream);

        if ($name === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле name'
            );
        }

        $payload = $this->parseString($stream);

        if ($payload === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле payload'
            );
        }

        $timeout = $this->parseUint64($stream);

        if ($timeout === false) {
            throw new \RuntimeException(
                'Не получилось десериализовать поле timeout'
            );
        }

        return new Messages\JobRequest($name, $payload, $timeout);
    }

    private function writeString(Stream $stream, string $value): void {
        $this->writeUint64($stream, strlen($value));
        $stream->write($value);
    }

    private function parseString(Stream $stream): string|false {
        $len = $this->parseUint64($stream);

        if ($len === false) {
            return false;
        }

        if ($len === 0) {
            return "";
        }

        return $stream->read($len);
    }

    private function writeUint64(Stream $stream, int $value): void {
        $packed = pack('P', $value);
        $stream->write((string) $packed);
    }

    private function parseUint64(Stream $stream): int|false {
        $bytes = $stream->read(8);

        if ($bytes === false) {
            return false;
        }

        $result = unpack('P', $bytes);

        if ($result === false) {
            return false;
        }

        return reset($result);
    }

    /**
     * @return array<string, string>|false
     */
    private function parseStringMap(Stream $stream): array|false {
        $len = $this->parseUint64($stream);

        if ($len === false) {
            return false;
        }

        $result = [];

        for ($i = 0; $i < $len; $i++) {
            $key = $this->parseString($stream);

            if ($key === false) {
                return false;
            }

            $value = $this->parseString($stream);

            if ($value === false) {
                return false;
            }

            $result[$key] = $value;
        }

        return $result;
    }

    /**
     * @param array<string, string> $map
     */
    private function writeStringMap(Stream $stream, array $map): void {
        $this->writeUint64($stream, count($map));

        foreach ($map as $key => $value) {
            $this->writeString($stream, $key);
            $this->writeString($stream, $value);
        }
    }

    /**
     * @return array<string, Messages\File>|false
     */
    private function parseFileMap(Stream $stream): array|false {
        $len = $this->parseUint64($stream);

        if ($len === false) {
            return false;
        }

        $result = [];

        for ($i = 0; $i < $len; $i++) {
            $key = $this->parseString($stream);

            if ($key === false) {
                return false;
            }

            $result[$key] = $this->parseFile($stream);
        }

        return $result;
    }
}
