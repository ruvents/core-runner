<?php

namespace Runner;

final class RPCRequest {
    private int $id;

    public function __construct(
        private string $method,
        private mixed $arg
    ) {
        $this->id = random_int(0, 10000); // TODO: UUIDv4
    }

    public function serialize(): string {
        return json_encode([
            'id' => $this->id,
            'method' => $this->method,
            'params' => [$this->arg],
        ]);
    }
}
