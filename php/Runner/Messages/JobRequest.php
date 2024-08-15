<?php

declare(strict_types=1);

namespace Runner\Messages;

final class JobRequest
{
    public function __construct(
        public readonly string $name,
        public readonly string $payload,
        public readonly int $timeout,
    ) {
    }
}
