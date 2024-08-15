<?php

declare(strict_types=1);

namespace Runner\Messages;

final class HTTPRequest
{
    /**
     * @param array<string, string> $headers
     * @param array<string, File> $files
     * @param array<string, string> $form
     */
    public function __construct(
        public readonly string $method,
        public readonly string $url,
        public readonly string $body,
        public readonly array $headers,
        public readonly array $files,
        public readonly array $form,
    ) {
    }
}
