<?php

declare(strict_types=1);

namespace Runner\Messages;

final class JobResponse
{
    public function __construct(public readonly string $payload) {
    }
}
