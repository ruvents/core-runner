<?php

declare(strict_types=1);

namespace Runner\Messages;

final class HTTPResponse
{
    /**
     * @param array<string, string> $headers
     */
    public function __construct(
        public readonly int $statusCode,
        public readonly array $headers,
        public readonly string $body,
    ) {
    }
}
