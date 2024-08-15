<?php

declare(strict_types=1);

namespace Runner\Messages;

final class File
{
    public function __construct(
        public readonly string $filename,
        public readonly string $tmpPath,
        public readonly int $size,
    ) {
    }
}
