<?php

declare(strict_types=1);

namespace Runner;

/**
 * Обертка над строкой для избежания повторного ее копирования. Простая версия
 * стримов из PSR.
 */
final class Stream
{
    /** @var resource $data */
    private $stream;

    public function __construct(string $data)
    {
        $stream = fopen('php://temp', 'r+');
        fwrite($stream, $data);
        rewind($stream); 
        $this->stream = $stream;
    }

    public function __destruct()
    {
        fclose($this->stream);
    }

    public function read(int $len): string|false
    {
        return fread($this->stream, $len);
    }

    public function write(string $data): int|false
    {
        return fwrite($this->stream, $data);
    }

    public function toString(): string
    {
        rewind($this->stream);
        return stream_get_contents($this->stream);
    }
}
